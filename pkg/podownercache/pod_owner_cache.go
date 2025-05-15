// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podownercache

import (
	"context"
	"strings"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PodOwnerCache struct {
	ctx       context.Context
	apiReader client.Reader

	cacheLock lock.RWMutex
	pods      map[types.NamespacedName]Pod
	ipToPod   map[string]types.NamespacedName
	// Cache for final owner references to reduce API calls, using pod NamespacedName as key
	ownerCache map[types.NamespacedName]*OwnerInfo
}

type Pod struct {
	types.NamespacedName
	OwnerInfo OwnerInfo
	IPs       []string
}

type OwnerInfo struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

type CacheInterface interface {
	GetPodByIP(ip string) *Pod
}

var logger *zap.Logger

func New(ctx context.Context, podInformer cache.SharedIndexInformer, apiReader client.Reader) (CacheInterface, error) {
	logger = logutils.Logger.Named("PodOwnerCache")
	logger.Info("create PodOwnerCache informer")

	res := &PodOwnerCache{
		ctx:        ctx,
		apiReader:  apiReader,
		cacheLock:  lock.RWMutex{},
		pods:       make(map[types.NamespacedName]Pod),
		ipToPod:    make(map[string]types.NamespacedName),
		ownerCache: make(map[types.NamespacedName]*OwnerInfo),
	}

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    res.onPodAdd,
		UpdateFunc: res.onPodUpdate,
		DeleteFunc: res.onPodDel,
	})
	if nil != err {
		logger.Error(err.Error())
		return nil, err
	}

	return res, nil
}

func (s *PodOwnerCache) onPodAdd(obj any) {
	if pod, ok := obj.(*corev1.Pod); ok {
		if pod.Spec.HostNetwork {
			return
		}

		if pod.Annotations[constant.MultusNetworkStatus] == "" {
			return
		}

		if !strings.Contains(pod.Annotations[constant.MultusNetworkStatus], "rdma-device") {
			return
		}

		if len(pod.Status.PodIPs) > 0 {
			ips := make([]string, 0, len(pod.Status.PodIPs))
			for _, p := range pod.Status.PodIPs {
				ips = append(ips, p.IP)
			}
			owner, err := s.getFinalOwner(pod)
			if err != nil {
				logger.Warn("failed to get final owner", zap.Error(err))
				return
			}
			s.cacheLock.Lock()
			defer s.cacheLock.Unlock()
			key := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
			item := Pod{NamespacedName: key, IPs: ips}
			if owner != nil {
				item.OwnerInfo = *owner
			}
			s.pods[key] = item
			for _, ip := range ips {
				s.ipToPod[ip] = key
			}
		}
	}
}

func (s *PodOwnerCache) onPodUpdate(oldObj, newObj interface{}) {
	s.onPodAdd(newObj)
}

func (s *PodOwnerCache) onPodDel(obj interface{}) {
	if pod, ok := obj.(*corev1.Pod); ok {
		s.cacheLock.Lock()
		defer s.cacheLock.Unlock()

		key := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
		if _, ok := s.pods[key]; !ok {
			return
		}
		for _, ip := range s.pods[key].IPs {
			delete(s.ipToPod, ip)
		}
		delete(s.pods, key)
	}
}

func (s *PodOwnerCache) getFinalOwner(obj metav1.Object) (*OwnerInfo, error) {
	var finalOwner *OwnerInfo

	// Create pod NamespacedName as the cache key
	podKey := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	// Check if we already have a cached final owner for this pod
	s.cacheLock.RLock()
	cachedOwner, exists := s.ownerCache[podKey]
	s.cacheLock.RUnlock()

	if exists {
		// If we found a cached result, return it immediately
		logger.Sugar().Debugf("Using cached owner for pod %s/%s", obj.GetNamespace(), obj.GetName())
		return cachedOwner, nil
	}

	for {
		ownerRefs := obj.GetOwnerReferences()
		if len(ownerRefs) == 0 {
			break
		}

		// Assuming the first owner reference
		ownerRef := ownerRefs[0]

		// If not in cache, create the owner info
		finalOwner = &OwnerInfo{
			APIVersion: ownerRef.APIVersion,
			Kind:       ownerRef.Kind,
			Namespace:  obj.GetNamespace(),
			Name:       ownerRef.Name,
		}

		// Prepare an empty object of the owner kind
		ownerObj := &unstructured.Unstructured{}
		ownerObj.SetAPIVersion(ownerRef.APIVersion)
		ownerObj.SetKind(ownerRef.Kind)

		err := s.apiReader.Get(s.ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      ownerRef.Name,
		}, ownerObj)
		if err != nil {
			if errors.IsForbidden(err) || errors.IsNotFound(err) {
				logger.Sugar().Debugf("%v for pod %s/%s", err, obj.GetNamespace(), obj.GetName())
				// Cache the negative result
				s.cacheLock.Lock()
				s.ownerCache[podKey] = nil
				s.cacheLock.Unlock()
				return nil, nil
			}
			return nil, err
		}

		// Set obj to the current owner to continue the loop
		obj = ownerObj
	}

	// Cache the final owner (or nil if no owner found)
	s.cacheLock.Lock()
	s.ownerCache[podKey] = finalOwner
	s.cacheLock.Unlock()

	if finalOwner != nil {
		logger.Sugar().Debugf("Cached final owner %s/%s of kind %s for pod %s/%s",
			finalOwner.Namespace, finalOwner.Name, finalOwner.Kind,
			podKey.Namespace, podKey.Name)
	} else {
		logger.Sugar().Debugf("No owner found for pod %s/%s", podKey.Namespace, podKey.Name)
	}

	return finalOwner, nil
}

func (s *PodOwnerCache) GetPodByIP(ip string) *Pod {
	s.cacheLock.RLock()
	defer s.cacheLock.RUnlock()
	item, exists := s.ipToPod[ip]
	if !exists {
		return nil
	}
	pod, exists := s.pods[item]
	if !exists {
		return nil
	}
	return &pod
}
