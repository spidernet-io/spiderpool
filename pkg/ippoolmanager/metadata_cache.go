// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"encoding/json"
	"fmt"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/lock"
)

type metadataCacheKey struct {
	uid  apitypes.UID
	name string
}

type metadataSnapshot struct {
	generation int64
	raw        string
	entries    decodedIPMetadata
	err        error
}

type decodedIPMetadata map[string]spiderpoolv2beta1.IPMetadataEntry

type metadataSnapshotCache struct {
	mu        lock.RWMutex
	snapshots map[metadataCacheKey]*metadataSnapshot
}

func newMetadataSnapshotCache() *metadataSnapshotCache {
	return &metadataSnapshotCache{
		snapshots: make(map[metadataCacheKey]*metadataSnapshot),
	}
}

func metadataKey(pool *spiderpoolv2beta1.SpiderIPPool) metadataCacheKey {
	return metadataCacheKey{uid: pool.UID, name: pool.Name}
}

func metadataRevision(pool *spiderpoolv2beta1.SpiderIPPool) (int64, string, error) {
	if pool == nil || pool.Status.IPMetaData == nil || pool.Status.IPMetaData.ObservedGeneration == nil {
		return 0, "", fmt.Errorf("%w: pool status has no observed generation", constant.ErrIPMetadataNotReady)
	}

	observedGeneration := *pool.Status.IPMetaData.ObservedGeneration
	if observedGeneration != pool.Generation {
		return 0, "", fmt.Errorf("%w: pool %s generation %d is not observed (status %d)",
			constant.ErrIPMetadataNotReady, pool.Name, pool.Generation, observedGeneration)
	}
	if pool.Status.IPMetaData.Metadata == nil {
		return 0, "", fmt.Errorf("%w: pool %s has no metadata snapshot", constant.ErrIPMetadataNotReady, pool.Name)
	}
	return observedGeneration, *pool.Status.IPMetaData.Metadata, nil
}

func (c *metadataSnapshotCache) snapshot(pool *spiderpoolv2beta1.SpiderIPPool) (decodedIPMetadata, error) {
	observedGeneration, raw, err := metadataRevision(pool)
	if err != nil {
		return nil, err
	}
	key := metadataKey(pool)
	c.mu.RLock()
	current := c.snapshots[key]
	if current != nil && current.generation == observedGeneration && current.raw == raw {
		c.mu.RUnlock()
		return current.entries, current.err
	}
	c.mu.RUnlock()

	return nil, fmt.Errorf("%w: pool %s has no current metadata cache snapshot", constant.ErrIPMetadataNotReady, pool.Name)
}

func (c *metadataSnapshotCache) update(pool *spiderpoolv2beta1.SpiderIPPool) {
	if pool == nil {
		return
	}
	if !IsIaaSPool(pool) {
		c.delete(pool)
		return
	}

	observedGeneration, raw, revisionErr := metadataRevision(pool)
	if revisionErr != nil {
		c.delete(pool)
		return
	}

	key := metadataKey(pool)
	c.mu.RLock()
	current := c.snapshots[key]
	if current != nil && current.generation == observedGeneration && current.raw == raw {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	entries := make(decodedIPMetadata)
	err := json.Unmarshal([]byte(raw), &entries)
	if err != nil {
		err = fmt.Errorf("%w: pool %s metadata is malformed: %w", constant.ErrIPMetadataNotReady, pool.Name, err)
		entries = nil
	}

	next := &metadataSnapshot{
		generation: observedGeneration,
		raw:        raw,
		entries:    entries,
		err:        err,
	}
	c.mu.Lock()
	c.snapshots[key] = next
	c.mu.Unlock()
}

func (c *metadataSnapshotCache) delete(pool *spiderpoolv2beta1.SpiderIPPool) {
	if pool == nil {
		return
	}
	c.mu.Lock()
	delete(c.snapshots, metadataKey(pool))
	c.mu.Unlock()
}

func (c *metadataSnapshotCache) register(ctx context.Context, runtimeCache ctrlcache.Cache) error {
	informer, err := runtimeCache.GetInformer(ctx, &spiderpoolv2beta1.SpiderIPPool{})
	if err != nil {
		return fmt.Errorf("failed to get SpiderIPPool informer for metadata cache: %w", err)
	}
	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pool, ok := obj.(*spiderpoolv2beta1.SpiderIPPool); ok {
				c.update(pool)
			}
		},
		UpdateFunc: func(_, newObj interface{}) {
			if pool, ok := newObj.(*spiderpoolv2beta1.SpiderIPPool); ok {
				c.update(pool)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pool, ok := obj.(*spiderpoolv2beta1.SpiderIPPool)
			if !ok {
				if tombstone, tombstoneOK := obj.(cache.DeletedFinalStateUnknown); tombstoneOK {
					pool, ok = tombstone.Obj.(*spiderpoolv2beta1.SpiderIPPool)
				}
			}
			if ok {
				c.delete(pool)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to register SpiderIPPool metadata cache handler: %w", err)
	}
	return nil
}
