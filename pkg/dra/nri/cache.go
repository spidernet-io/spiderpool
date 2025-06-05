// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package nri

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/containernetworking/cni/libcni"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	resourcev1 "k8s.io/api/resource/v1"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type Cache struct {
	mu sync.RWMutex

	resourceSliceByNode map[string]*resourcev1.ResourceSlice
	resourceSliceTS     map[string]time.Time

	nadConfigByKey map[string]string
	nadTS          map[string]time.Time

	confListByKey map[string]*libcni.NetworkConfigList
	confListTS    map[string]time.Time

	resourceClaimByKey map[string]*resourcev1.ResourceClaim
	resourceClaimTS    map[string]time.Time

	podClaimsByUID    map[string]map[string]struct{}
	podClaimsByNSName map[string]map[string]struct{}

	podNetworkStatusByUID map[string][]*NetworkStatus
	podNetworkStatusTS    map[string]time.Time

	podResourcesList *podresourcesapi.ListPodResourcesResponse
	podResourcesTS   time.Time

	nodeWarmupTS map[string]time.Time
}

var defaultCache = NewCache()

func NewCache() *Cache {
	return &Cache{
		resourceSliceByNode:      map[string]*resourcev1.ResourceSlice{},
		resourceSliceTS:          map[string]time.Time{},
		nadConfigByKey:           map[string]string{},
		nadTS:                    map[string]time.Time{},
		confListByKey:            map[string]*libcni.NetworkConfigList{},
		confListTS:               map[string]time.Time{},
		resourceClaimByKey:       map[string]*resourcev1.ResourceClaim{},
		resourceClaimTS:          map[string]time.Time{},
		podClaimsByUID:           map[string]map[string]struct{}{},
		podClaimsByNSName:        map[string]map[string]struct{}{},
		podNetworkStatusByUID:    map[string][]*NetworkStatus{},
		podNetworkStatusTS:       map[string]time.Time{},
		nodeWarmupTS:             map[string]time.Time{},
	}
}

func claimKey(namespace, name string) string {
	return namespace + "/" + name
}

func podNSNameKey(namespace, name string) string {
	return namespace + "/" + name
}

func (c *Cache) SetResourceClaim(rc *resourcev1.ResourceClaim) {
	if rc == nil || rc.Namespace == "" || rc.Name == "" {
		return
	}
	key := claimKey(rc.Namespace, rc.Name)
	copy := rc.DeepCopy()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resourceClaimByKey[key] = copy
	c.resourceClaimTS[key] = time.Now()
}

func (c *Cache) GetResourceClaim(namespace, name string, maxAge time.Duration) (*resourcev1.ResourceClaim, bool) {
	key := claimKey(namespace, name)
	c.mu.RLock()
	rc := c.resourceClaimByKey[key]
	ts := c.resourceClaimTS[key]
	c.mu.RUnlock()
	if rc == nil {
		return nil, false
	}
	if maxAge > 0 && time.Since(ts) > maxAge {
		return nil, false
	}
	return rc, true
}

func (c *Cache) DeleteResourceClaim(namespace, name string) {
	key := claimKey(namespace, name)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.resourceClaimByKey, key)
	delete(c.resourceClaimTS, key)
}

func (c *Cache) DeletePodClaimIndexByNSName(podNamespace, podName string) {
	if podNamespace == "" || podName == "" {
		return
	}
	key := podNSNameKey(podNamespace, podName)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.podClaimsByNSName, key)
}

func (c *Cache) DeletePodClaimIndexByUID(podUID string) {
	if podUID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.podClaimsByUID, podUID)
}

func (c *Cache) IndexPodClaimsFromResourceClaim(rc *resourcev1.ResourceClaim) {
	if rc == nil || rc.Namespace == "" || rc.Name == "" {
		return
	}
	ck := claimKey(rc.Namespace, rc.Name)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, consumer := range rc.Status.ReservedFor {
		if consumer.Resource != "pods" {
			continue
		}
		if consumer.UID != "" {
			uid := string(consumer.UID)
			if _, ok := c.podClaimsByUID[uid]; !ok {
				c.podClaimsByUID[uid] = map[string]struct{}{}
			}
			c.podClaimsByUID[uid][ck] = struct{}{}
		}
		if consumer.Name != "" {
			pk := podNSNameKey(rc.Namespace, consumer.Name)
			if _, ok := c.podClaimsByNSName[pk]; !ok {
				c.podClaimsByNSName[pk] = map[string]struct{}{}
			}
			c.podClaimsByNSName[pk][ck] = struct{}{}
		}
	}
}

func (c *Cache) GetPodClaimRefs(podUID, podNamespace, podName string) ([]client.ObjectKey, bool) {
	c.mu.RLock()
	var set map[string]struct{}
	if podUID != "" {
		set = c.podClaimsByUID[podUID]
	}
	if set == nil && podNamespace != "" && podName != "" {
		set = c.podClaimsByNSName[podNSNameKey(podNamespace, podName)]
	}
	c.mu.RUnlock()
	if len(set) == 0 {
		return nil, false
	}

	refs := make([]client.ObjectKey, 0, len(set))
	for ck := range set {
		parts := strings.SplitN(ck, "/", 2)
		if len(parts) != 2 {
			continue
		}
		refs = append(refs, client.ObjectKey{Namespace: parts[0], Name: parts[1]})
	}
	return refs, len(refs) > 0
}

func GetCache() *Cache {
	return defaultCache
}

func (c *Cache) SetPodNetworkStatus(podUID string, status []*NetworkStatus) {
	if podUID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.podNetworkStatusByUID[podUID] = status
	c.podNetworkStatusTS[podUID] = time.Now()
}

func (c *Cache) GetPodNetworkStatus(podUID string) ([]*NetworkStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	status, ok := c.podNetworkStatusByUID[podUID]
	return status, ok
}

func (c *Cache) DeletePod(podUID string) {
	if podUID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.podNetworkStatusByUID, podUID)
	delete(c.podNetworkStatusTS, podUID)
	delete(c.podClaimsByUID, podUID)
}

func (c *Cache) GetPodResourcesList(maxAge time.Duration) (*podresourcesapi.ListPodResourcesResponse, bool) {
	c.mu.RLock()
	resp := c.podResourcesList
	ts := c.podResourcesTS
	c.mu.RUnlock()
	if resp == nil {
		return nil, false
	}
	if maxAge > 0 && time.Since(ts) > maxAge {
		return nil, false
	}
	return resp, true
}

func (c *Cache) SetPodResourcesList(resp *podresourcesapi.ListPodResourcesResponse) {
	if resp == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.podResourcesList = resp
	c.podResourcesTS = time.Now()
}

func (c *Cache) GetResourceSlice(nodeName string, maxAge time.Duration) (*resourcev1.ResourceSlice, bool) {
	if nodeName == "" {
		return nil, false
	}
	c.mu.RLock()
	rs := c.resourceSliceByNode[nodeName]
	ts := c.resourceSliceTS[nodeName]
	c.mu.RUnlock()
	if rs == nil {
		return nil, false
	}
	if maxAge > 0 && time.Since(ts) > maxAge {
		return nil, false
	}
	return rs, true
}

func (c *Cache) SetResourceSlice(nodeName string, rs *resourcev1.ResourceSlice) {
	if nodeName == "" || rs == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resourceSliceByNode[nodeName] = rs
	c.resourceSliceTS[nodeName] = time.Now()
}

func nadKey(namespace, name string) string {
	return namespace + "/" + name
}

func confListKey(namespace, name, deviceID string) string {
	return fmt.Sprintf("%s/%s@%s", namespace, name, deviceID)
}

func (c *Cache) GetNADConfig(namespace, name string, maxAge time.Duration) (string, bool) {
	key := nadKey(namespace, name)
	c.mu.RLock()
	cfg, ok := c.nadConfigByKey[key]
	ts := c.nadTS[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if maxAge > 0 && time.Since(ts) > maxAge {
		return "", false
	}
	return cfg, true
}

func (c *Cache) SetNADConfig(namespace, name, cfg string) {
	if namespace == "" || name == "" || cfg == "" {
		return
	}
	key := nadKey(namespace, name)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nadConfigByKey[key] = cfg
	c.nadTS[key] = time.Now()
}

func (c *Cache) GetConfList(namespace, name, deviceID string, maxAge time.Duration) (*libcni.NetworkConfigList, bool) {
	key := confListKey(namespace, name, deviceID)
	c.mu.RLock()
	conf := c.confListByKey[key]
	ts := c.confListTS[key]
	c.mu.RUnlock()
	if conf == nil {
		return nil, false
	}
	if maxAge > 0 && time.Since(ts) > maxAge {
		return nil, false
	}
	return conf, true
}

func (c *Cache) SetConfList(namespace, name, deviceID string, conf *libcni.NetworkConfigList) {
	if namespace == "" || name == "" || deviceID == "" || conf == nil {
		return
	}
	key := confListKey(namespace, name, deviceID)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.confListByKey[key] = conf
	c.confListTS[key] = time.Now()
}

func (c *Cache) WarmupNode(ctx context.Context, k8sClient client.Client, nodeName, nadNamespace string) {
	if k8sClient == nil || nodeName == "" || nadNamespace == "" {
		return
	}

	c.mu.RLock()
	last := c.nodeWarmupTS[nodeName]
	c.mu.RUnlock()
	if time.Since(last) < 10*time.Second {
		return
	}

	rs, err := getResourceSliceByNode(ctx, k8sClient, nodeName)
	if err != nil {
		return
	}
	c.SetResourceSlice(nodeName, rs)

	cniConfigNames := extractCniConfigNamesFromResourceSlice(rs)
	for _, name := range cniConfigNames {
		nad := &netv1.NetworkAttachmentDefinition{}
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: nadNamespace, Name: name}, nad); err != nil {
			continue
		}
		if nad.Spec.Config == "" {
			continue
		}
		c.SetNADConfig(nadNamespace, name, nad.Spec.Config)
	}

	c.mu.Lock()
	c.nodeWarmupTS[nodeName] = time.Now()
	c.mu.Unlock()
}

func getResourceSliceByNode(ctx context.Context, k8sClient client.Client, nodeName string) (*resourcev1.ResourceSlice, error) {
	fieldSelector := client.MatchingFields(map[string]string{
		resourcev1.ResourceSliceSelectorNodeName: nodeName,
		resourcev1.ResourceSliceSelectorDriver:   constant.DRADriverName,
	})

	rsList := &resourcev1.ResourceSliceList{}
	if err := k8sClient.List(ctx, rsList, fieldSelector); err != nil {
		return nil, err
	}
	if len(rsList.Items) == 0 {
		return nil, fmt.Errorf("no ResourceSlice found for node %s", nodeName)
	}
	return &rsList.Items[0], nil
}

func extractCniConfigNamesFromResourceSlice(rs *resourcev1.ResourceSlice) []string {
	if rs == nil {
		return nil
	}

	seen := map[string]struct{}{}
	var names []string
	for _, dev := range rs.Spec.Devices {
		if dev.Attributes == nil {
			continue
		}
		if !IsReadyRdmaResourceDevice(dev) {
			continue
		}
		cniConfigsStr := GetStringValueForAttributes("cniConfigs", dev.Attributes)
		if cniConfigsStr == "" {
			continue
		}
		for _, n := range strings.Split(cniConfigsStr, ",") {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			names = append(names, n)
		}
	}
	return names
}
