// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
)

// Label("unitest")

func TestMetadataSnapshotCache(t *testing.T) {
	raw := `{"scope":"","parentNic":"eth0","ips":{"10.0.0.1":{"mac":"00:11:22:33:44:55","vlan":7}}}`
	pool := &spiderpoolv2beta1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "pool",
			UID:        types.UID("pool-uid"),
			Generation: 3,
			Labels: map[string]string{
				constant.LabelIPPoolIaasProvider: "huaweicloud",
			},
		},
		Status: spiderpoolv2beta1.IPPoolStatus{
			IPMetaData: &spiderpoolv2beta1.IPMetaData{
				Metadata:           &raw,
				ObservedGeneration: ptr.To(int64(3)),
			},
		},
	}
	cache := newMetadataSnapshotCache()

	cache.update(pool)
	first, err := cache.snapshot(pool)
	if err != nil {
		t.Fatalf("first snapshot failed: %v", err)
	}
	if first.entries["10.0.0.1"].MAC != "00:11:22:33:44:55" {
		t.Fatalf("unexpected decoded metadata: %#v", first)
	}
	if _, exists := first.entries[constant.IPPoolMetadataParentNicKey]; exists {
		t.Fatalf("reserved parentNic key leaked into decoded entries: %#v", first)
	}
	if len(first.entries) != 1 {
		t.Fatalf("unexpected decoded entry count: %#v", first)
	}

	pool.ResourceVersion = "2"
	pool.Status.AllocatedIPs = ptr.To(`{"10.0.0.9":{"pod":"default/pod","podUid":"uid"}}`)
	second, err := cache.snapshot(pool)
	if err != nil {
		t.Fatalf("reused snapshot failed: %v", err)
	}
	if reflect.ValueOf(first).Pointer() != reflect.ValueOf(second).Pointer() {
		t.Fatal("allocatedIPs-only update reparsed metadata")
	}

	replacement := `{"scope":"","ips":{"10.0.0.2":{"mac":"00:11:22:33:44:66","vlan":8}}}`
	pool.Status.IPMetaData.Metadata = &replacement
	cache.update(pool)
	third, err := cache.snapshot(pool)
	if err != nil {
		t.Fatalf("replacement snapshot failed: %v", err)
	}
	if reflect.ValueOf(second).Pointer() == reflect.ValueOf(third).Pointer() {
		t.Fatal("metadata status update did not replace snapshot")
	}
	if _, exists := third.entries["10.0.0.2"]; !exists {
		t.Fatalf("replacement metadata was not installed: %#v", third)
	}

	cache.delete(pool)
	if _, exists := cache.snapshots[metadataKey(pool)]; exists {
		t.Fatal("snapshot was not evicted")
	}
}

func TestMetadataSnapshotCacheMissFailsClosed(t *testing.T) {
	raw := `{}`
	pool := &spiderpoolv2beta1.SpiderIPPool{
		ObjectMeta: metav1.ObjectMeta{Name: "pool", UID: types.UID("uid"), Generation: 1},
		Status: spiderpoolv2beta1.IPPoolStatus{
			IPMetaData: &spiderpoolv2beta1.IPMetaData{
				Metadata:           &raw,
				ObservedGeneration: ptr.To(int64(1)),
			},
		},
	}
	if _, err := newMetadataSnapshotCache().snapshot(pool); err == nil {
		t.Fatal("expected cache miss to fail closed")
	}
}

func BenchmarkIPMetadataCache(b *testing.B) {
	for _, size := range []int{64, 1000} {
		entries := make(decodedIPMetadata, size)
		for i := 0; i < size; i++ {
			entries[fmt.Sprintf("10.0.%d.%d", i/256, i%256)] = spiderpoolv2beta1.IPMetadataEntry{
				MAC:  fmt.Sprintf("02:00:%02x:%02x:%02x:%02x", byte(i>>24), byte(i>>16), byte(i>>8), byte(i)),
				VLAN: ptr.To(int32(100 + i)),
			}
		}
		encoded, err := json.Marshal(map[string]interface{}{"scope": "", "ips": entries})
		if err != nil {
			b.Fatal(err)
		}
		raw := string(encoded)
		pool := &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "pool",
				UID:        types.UID("uid"),
				Generation: 1,
				Labels: map[string]string{
					constant.LabelIPPoolIaasProvider: "huaweicloud",
				},
			},
			Status: spiderpoolv2beta1.IPPoolStatus{
				IPMetaData: &spiderpoolv2beta1.IPMetaData{
					Metadata:           &raw,
					ObservedGeneration: ptr.To(int64(1)),
				},
			},
		}
		cache := newMetadataSnapshotCache()
		cache.update(pool)

		b.Run(fmt.Sprintf("provider-build/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := json.Marshal(entries); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("deepcopy-string/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = pool.DeepCopy()
			}
		})
		b.Run(fmt.Sprintf("allocation-cache-hit/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := cache.snapshot(pool); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("allocation-without-cache/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var decoded decodedIPMetadata
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("outer-marshal/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := json.Marshal(pool); err != nil {
					b.Fatal(err)
				}
			}
		})
		outer, err := json.Marshal(pool)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("outer-unmarshal/%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var decoded spiderpoolv2beta1.SpiderIPPool
				if err := json.Unmarshal(outer, &decoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestMetadataSnapshotCacheFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		generation int64
		observed   *int64
		raw        *string
	}{
		{name: "missing observed generation", generation: 1, raw: ptr.To(`{}`)},
		{name: "generation mismatch", generation: 2, observed: ptr.To(int64(1)), raw: ptr.To(`{}`)},
		{name: "missing metadata", generation: 1, observed: ptr.To(int64(1))},
		{name: "malformed metadata", generation: 1, observed: ptr.To(int64(1)), raw: ptr.To(`{`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := &spiderpoolv2beta1.SpiderIPPool{
				ObjectMeta: metav1.ObjectMeta{Name: "pool", UID: types.UID(tt.name), Generation: tt.generation},
				Status: spiderpoolv2beta1.IPPoolStatus{
					IPMetaData: &spiderpoolv2beta1.IPMetaData{
						Metadata:           tt.raw,
						ObservedGeneration: tt.observed,
					},
				},
			}
			cache := newMetadataSnapshotCache()
			cache.update(pool)
			if _, err := cache.snapshot(pool); err == nil {
				t.Fatal("expected fail-closed metadata error")
			}
		})
	}
}

func TestDecodePoolMetadataSchemaV2(t *testing.T) {
	newPool := func(nodeNames ...string) *spiderpoolv2beta1.SpiderIPPool {
		return &spiderpoolv2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{Name: "pool"},
			Spec:       spiderpoolv2beta1.IPPoolSpec{NodeName: nodeNames},
		}
	}

	t.Run("global scope with per-entry node", func(t *testing.T) {
		raw := `{"scope":"","parentNic":"eth0","ips":{"10.0.0.1":{"mac":"aa","vlan":7,"node":"node-1"},"10.0.0.2":{"mac":"bb","vlan":-1}}}`
		decoded, err := decodePoolMetadata(newPool(), raw)
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if !decoded.isGlobal() {
			t.Fatal("expected global scope")
		}
		if decoded.parentNic != "eth0" {
			t.Fatalf("unexpected parentNic: %q", decoded.parentNic)
		}
		if e := decoded.entries["10.0.0.1"]; e.Node == nil || *e.Node != "node-1" {
			t.Fatalf("per-entry node lost: %#v", e)
		}
		if e := decoded.entries["10.0.0.2"]; e.Node != nil || *e.VLAN != -1 {
			t.Fatalf("unbound entry mangled: %#v", e)
		}
	})

	t.Run("global scope rejects node-pinned pool", func(t *testing.T) {
		raw := `{"scope":"","ips":{}}`
		if _, err := decodePoolMetadata(newPool("node-1"), raw); err == nil {
			t.Fatal("expected mode invariant failure")
		}
	})

	t.Run("node-level scope must match spec.nodeName", func(t *testing.T) {
		raw := `{"scope":"node-1","ips":{"10.0.0.1":{"mac":"aa","vlan":7}}}`
		decoded, err := decodePoolMetadata(newPool("node-1"), raw)
		if err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if decoded.isGlobal() || decoded.scope != "node-1" {
			t.Fatalf("unexpected scope: %#v", decoded.scope)
		}
		if _, err := decodePoolMetadata(newPool("node-2"), raw); err == nil {
			t.Fatal("expected scope/nodeName mismatch failure")
		}
	})

	t.Run("node-level scope rejects per-entry node", func(t *testing.T) {
		raw := `{"scope":"node-1","ips":{"10.0.0.1":{"mac":"aa","vlan":7,"node":"node-2"}}}`
		if _, err := decodePoolMetadata(newPool("node-1"), raw); err == nil {
			t.Fatal("expected per-entry node rejection on a node-level pool")
		}
	})

	t.Run("metadata without scope fails closed", func(t *testing.T) {
		raw := `{"ips":{"10.0.0.1":{"mac":"aa","vlan":7}}}`
		if _, err := decodePoolMetadata(newPool(), raw); err == nil {
			t.Fatal("expected not-yet-reconciled failure")
		}
	})

	t.Run("flat shape without scope is rejected", func(t *testing.T) {
		raw := `{"parentNic":"eth0","10.0.0.1":{"mac":"aa","vlan":7}}`
		if _, err := decodePoolMetadata(newPool("node-1"), raw); err == nil {
			t.Fatal("expected scope-less metadata rejection")
		}
	})
}
