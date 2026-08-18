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
	decoded    *decodedPoolMetadata
	err        error
}

type decodedIPMetadata map[string]spiderpoolv2beta1.IPMetadataEntry

// decodedPoolMetadata is the decoded logical payload of
// status.ipMetaData.metadata:
// {"scope": "<nodeName>"|"", "parentNic": "<nic>", "ips": {addr: entry}}.
type decodedPoolMetadata struct {
	// scope is an empty string for a global pool, or the pinned node
	// name for a node-level pool.
	scope     string
	parentNic string
	entries   decodedIPMetadata
}

// isGlobal reports whether the decoded metadata declares the global pool
// mode (an explicit empty scope).
func (m *decodedPoolMetadata) isGlobal() bool {
	return m != nil && m.scope == ""
}

// ipEntries returns the decoded per-IP entry map, nil-safe for non-IaaS
// pools whose snapshot is never fetched.
func (m *decodedPoolMetadata) ipEntries() decodedIPMetadata {
	if m == nil {
		return nil
	}
	return m.entries
}

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

func (c *metadataSnapshotCache) snapshot(pool *spiderpoolv2beta1.SpiderIPPool) (*decodedPoolMetadata, error) {
	observedGeneration, raw, err := metadataRevision(pool)
	if err != nil {
		return nil, err
	}
	key := metadataKey(pool)
	c.mu.RLock()
	current := c.snapshots[key]
	if current != nil && current.generation == observedGeneration && current.raw == raw {
		c.mu.RUnlock()
		return current.decoded, current.err
	}
	c.mu.RUnlock()

	return nil, fmt.Errorf("%w: pool %s has no current metadata cache snapshot", constant.ErrIPMetadataNotReady, pool.Name)
}

// decodePoolMetadata parses one authoritative metadata JSON string
// ({"scope": ..., "parentNic": ..., "ips": {...}}) into its decoded logical
// form, enforcing the mode invariants against the pool spec:
//   - node-level (scope = "<node>"): scope must equal the pool's single
//     spec.nodeName entry and no per-entry "node" may appear;
//   - global (scope = ""): the pool must not be node-pinned via
//     spec.nodeName;
//   - payload without a "scope" key: not yet reconciled (fail closed).
func decodePoolMetadata(pool *spiderpoolv2beta1.SpiderIPPool, raw string) (*decodedPoolMetadata, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return nil, fmt.Errorf("%w: pool %s metadata is malformed: %w", constant.ErrIPMetadataNotReady, pool.Name, err)
	}

	decoded := &decodedPoolMetadata{entries: make(decodedIPMetadata)}

	rawScope, hasScope := top["scope"]
	if !hasScope {
		return nil, fmt.Errorf("%w: pool %s metadata has no scope: not yet reconciled",
			constant.ErrIPMetadataNotReady, pool.Name)
	}
	if err := json.Unmarshal(rawScope, &decoded.scope); err != nil {
		return nil, fmt.Errorf("%w: pool %s metadata scope is malformed: %w", constant.ErrIPMetadataNotReady, pool.Name, err)
	}
	if rawNic, ok := top[constant.IPPoolMetadataParentNicKey]; ok {
		if err := json.Unmarshal(rawNic, &decoded.parentNic); err != nil {
			return nil, fmt.Errorf("%w: pool %s metadata parentNic is malformed: %w", constant.ErrIPMetadataNotReady, pool.Name, err)
		}
	}
	if rawIPs, ok := top["ips"]; ok {
		if err := json.Unmarshal(rawIPs, (*map[string]spiderpoolv2beta1.IPMetadataEntry)(&decoded.entries)); err != nil {
			return nil, fmt.Errorf("%w: pool %s metadata ips map is malformed: %w", constant.ErrIPMetadataNotReady, pool.Name, err)
		}
	}

	if decoded.scope == "" {
		// Global pool: per-IP placement lives in entry.node; the pool
		// itself must not be node-pinned.
		if len(pool.Spec.NodeName) != 0 {
			return nil, fmt.Errorf("%w: pool %s metadata declares global scope but the pool is pinned via spec.nodeName %v",
				constant.ErrIPMetadataNotReady, pool.Name, pool.Spec.NodeName)
		}
	} else {
		// Node-level pool: scope must match the pinned node and no
		// per-entry placement may appear.
		if len(pool.Spec.NodeName) != 1 || pool.Spec.NodeName[0] != decoded.scope {
			return nil, fmt.Errorf("%w: pool %s metadata scope %q does not match spec.nodeName %v",
				constant.ErrIPMetadataNotReady, pool.Name, decoded.scope, pool.Spec.NodeName)
		}
		for addr, entry := range decoded.entries {
			if entry.Node != nil {
				return nil, fmt.Errorf("%w: pool %s is node-level (scope %q) but metadata entry %s carries a per-entry node",
					constant.ErrIPMetadataNotReady, pool.Name, decoded.scope, addr)
			}
		}
	}
	return decoded, nil
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

	decoded, err := decodePoolMetadata(pool, raw)

	next := &metadataSnapshot{
		generation: observedGeneration,
		raw:        raw,
		decoded:    decoded,
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
