// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"errors"
)

var (
	ErrWrongInput                       = errors.New("wrong input")
	ErrNoAvailablePool                  = errors.New("no IPPool available")
	ErrRetriesExhausted                 = errors.New("exhaust all retries")
	ErrIPUsedOut                        = errors.New("all IP addresses used out")
	ErrIPConflict                       = errors.New("ip conflict")
	ErrForbidReleasingStatefulWorkload  = errors.New("forbid releasing IPs for stateful workload ")
	ErrForbidReleasingStatelessWorkload = errors.New("forbid releasing IPs for stateless workload")
)

var ErrMissingRequiredParam = errors.New("must be specified")

var ErrUnknown = errors.New("unknown")

var ErrFreeIPsNotEnough = errors.New("IPPool available free IPs are not enough")
