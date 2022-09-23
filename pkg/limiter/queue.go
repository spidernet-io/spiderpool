// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package limiter

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

type Limiter interface {
	AcquireTicket(ctx context.Context, tickets ...string) (Reason, error)
	ReleaseTicket(ctx context.Context, tickets ...string)
	Start(ctx context.Context) error
}

func NewLimiter(c *LimiterConfig) Limiter {
	q := &queue{
		cond:           sync.NewCond(&lock.Mutex{}),
		maxQueueSize:   c.MaxQueueSize,
		maxWaitTime:    c.MaxWaitTime,
		elements:       make([]*e, 0, c.MaxQueueSize),
		grantedTickets: map[string]int{},
	}

	return q
}

const DefaultTicket = "not to use this"

type Reason int

const (
	Checkin Reason = iota
	CheckinTimeout
	UnexpectedBlocking
	ShutdownQueue
	FullQueue
)

var (
	ErrUnexpectedBlocking = errors.New("unexpected blocking, queuer may be lost")
	ErrShutdownQueue      = errors.New("queue has been shutdown")
	ErrFullQueue          = errors.New("queue is full")
)

type queue struct {
	cond           *sync.Cond
	shuttingDown   bool
	maxQueueSize   int
	maxWaitTime    time.Duration
	elements       []*e
	grantedTickets map[string]int
}

type e struct {
	wantedTickets        []string
	notifyCheckin        chan empty
	notifyCheckinTimeout chan empty
	finalCheckinTime     time.Time
}

type empty struct{}

func (q *queue) AcquireTicket(ctx context.Context, tickets ...string) (Reason, error) {
	logger := logutils.FromContext(ctx)

	logger.Sugar().Debugf("Waiting in queue with expect tickets: %v", tickets)
	e, err := q.queueUp(tickets...)
	if err != nil {
		if errors.Is(err, ErrShutdownQueue) {
			return ShutdownQueue, err
		}
		if errors.Is(err, ErrFullQueue) {
			return FullQueue, err
		}
	}

	select {
	case <-e.notifyCheckin:
		logger.Debug("Succeed to acquire tickets")
		return Checkin, nil
	case <-e.notifyCheckinTimeout:
		logger.Debug("Succeed to acquire tickets due to timeout")
		return CheckinTimeout, nil
	case <-time.After(time.Until(e.finalCheckinTime.Add(3 * time.Second))):
		return UnexpectedBlocking, ErrUnexpectedBlocking
	}
}

func (q *queue) queueUp(tickets ...string) (*e, error) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	if q.shuttingDown {
		return nil, ErrShutdownQueue
	}

	if len(q.elements) >= q.maxQueueSize {
		return nil, fmt.Errorf("%w with a maximum length of %d", ErrFullQueue, q.maxQueueSize)
	}

	if len(tickets) == 0 {
		tickets = append(tickets, DefaultTicket)
	}

	e := &e{
		wantedTickets:        tickets,
		notifyCheckin:        make(chan empty),
		notifyCheckinTimeout: make(chan empty),
		finalCheckinTime:     time.Now().Add(q.maxWaitTime),
	}
	q.elements = append(q.elements, e)

	// When a new queuer begins to queue, here should try to wake up the
	// conductor who may be rest in two cases at this time:
	// 1. Queue is empty.
	// 2. Checkin is blocking to avoid long polling.
	q.cond.Signal()

	return e, nil
}

func (q *queue) ReleaseTicket(ctx context.Context, tickets ...string) {
	logger := logutils.FromContext(ctx)
	logger.Debug("Work has been completed, try to release tickets")

	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	if len(tickets) == 0 {
		tickets = append(tickets, DefaultTicket)
	}
	for _, t := range tickets {
		q.grantedTickets[t]--
		if q.grantedTickets[t] == 0 {
			delete(q.grantedTickets, t)
		}
	}

	// When work is finished, the conductor who may be rest should be awakened
	// to continue ticket checking. The reason for using Broadcast instead of
	// Signal is that checkin and waitAllTicketsRetrieved will wait at the
	// same time when the queue shutdown.
	q.cond.Broadcast()
}

func (q *queue) Start(ctx context.Context) error {
	defer q.gracefulShutdown()
	logger := logutils.FromContext(ctx)

	go func() {
		for !q.checkin() {
		}
	}()
	<-ctx.Done()
	logger.Info("Begin to shutdown the queue")

	return nil
}

func (q *queue) checkin() (shuttingDown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	if len(q.elements) == 0 && !q.shuttingDown {
		// When no one is in queue, don't do meaningless ticket checking. Here may
		// be awakened by the following cases:
		// 1. A new queuer added.
		// 2. An ongoing work has just been completed.
		// 3. Queue shutdown.
		q.cond.Wait()
	}

	if len(q.elements) == 0 {
		return q.shuttingDown
	}

	for i := 0; i < len(q.elements); i++ {
		if time.Now().After(q.elements[i].finalCheckinTime) {
			q.grantTicket(q.elements[i], CheckinTimeout)
			q.elements = append(q.elements[:i], q.elements[i+1:]...)
			i--
			continue
		}

		if !q.checkAvailableTicket(q.elements[i].wantedTickets...) {
			continue
		}

		q.grantTicket(q.elements[i], Checkin)
		q.elements = append(q.elements[:i], q.elements[i+1:]...)
		i--
	}

	if len(q.elements) != 0 {
		finish := make(chan empty)
		waitForFirstQueuer := func(e *e) {
			select {
			case <-time.After(time.Until(e.finalCheckinTime)):
				q.cond.Broadcast()
			case <-finish:
			}
		}
		go waitForFirstQueuer(q.elements[0])

		// Waiting here for avoiding next unnecessary round of polling q.elements
		// following cases could make it move on:
		// 1. A new queuer call queueUp().
		// 2. ReleaseTicket() when ticket revert.
		// 3. waitForFirstQueuer() found the earliest queuer who does not want
		// waiting anymore.
		// 4. shutdown() informs to close the queue.
		q.cond.Wait()

		// inform waitForFirstQueuer to exist if it is still running
		close(finish)
	}

	return false
}

func (q *queue) checkAvailableTicket(tickets ...string) bool {
	for _, t := range tickets {
		if _, ok := q.grantedTickets[t]; ok {
			return false
		}
	}

	return true
}

func (q *queue) grantTicket(e *e, r Reason) {
	for _, t := range e.wantedTickets {
		q.grantedTickets[t]++
	}

	if r == Checkin {
		close(e.notifyCheckin)
	} else if r == CheckinTimeout {
		close(e.notifyCheckinTimeout)
	}
}

func (q *queue) gracefulShutdown() {
	q.shutdown()
	for !q.isAllTicketsRetrieved() {
		q.waitAllTicketsRetrieved()
	}
}

func (q *queue) shutdown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	q.shuttingDown = true

	// When the queue shutdown, notify the conductor do checkin once. If
	// there are no queuers at this time, checkin successfully returns.
	// Otherwise, after all queuers enter work, checkin returns.
	q.cond.Signal()
}

func (q *queue) isAllTicketsRetrieved() bool {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	return len(q.grantedTickets) == 0
}

func (q *queue) waitAllTicketsRetrieved() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()

	// Make sure here don't wait for queue without working elements, as that
	// could result in waiting for ReleaseTicket to be called on items in an
	// empty queue which has already been shutdown, which will result in waiting
	// indefinitely.
	if len(q.grantedTickets) == 0 {
		return
	}

	// Wait for a working elements to complete their work. Here will be awakened
	// by ReleaseTicket when queue shutdown. When all the work to be completed
	// is finished, gracefulShutdown will ensure that waitAllTicketsRetrieved will
	// not be called again, and then shutdown safely.
	q.cond.Wait()
}
