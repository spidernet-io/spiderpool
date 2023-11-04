// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

// monitorGCSignal will monitor signal from CLI, DefaultGCInterval
func (s *SpiderGC) monitorGCSignal(ctx context.Context) {
	logger.Debug("start to monitor gc signal for CLI or default GC interval")

	d := time.Duration(s.gcConfig.DefaultGCIntervalDuration) * time.Second
	logger.Sugar().Debugf("default IP GC interval duration is %v", d)
	timer := time.NewTimer(d)
	defer timer.Stop()

	go func() {
		logger.Debug("initial scan all for cluster firstly")
		s.gcSignal <- struct{}{}
	}()

	for {
		select {
		case <-timer.C:
			select {
			// In concurrency situation, the backup controller must execute scanAll
			case <-s.gcSignal:
				logger.Info("receive CLI GC request, execute scan all right now!")
				s.executeScanAll(ctx)
			default:
				// The Elected controller will scan All with default GC interval
				if s.leader.IsElected() {
					logger.Info("trigger default GC interval, execute scan all right now!")
					s.executeScanAll(ctx)
				}
			}

			// CLI request
		case <-s.gcSignal:
			logger.Info("receive CLI GC request, execute scan all right now!")
			s.executeScanAll(ctx)
			time.Sleep(time.Duration(s.gcConfig.GCSignalGapDuration) * time.Second)

			// discard the concurrent signal
			select {
			case <-timer.C:
			default:
			}

		case <-ctx.Done():
			logger.Warn("receive ctx done, stop monitoring gc signal!")
			return
		}

		timer.Reset(time.Duration(s.gcConfig.DefaultGCIntervalDuration) * time.Second)
	}
}

// executeScanAll scans the whole pod and whole IPPoolList
func (s *SpiderGC) executeScanAll(ctx context.Context) {
	poolList, err := s.ippoolMgr.ListIPPools(ctx, constant.UseCache)
	if nil != err {
		if apierrors.IsNotFound(err) {
			logger.Sugar().Warnf("scan all failed, ippoolList not found!")
			return
		}

		logger.Sugar().Errorf("scan all failed: '%v'", err)
		return
	}

	var v4poolList, v6poolList []spiderpoolv2beta1.SpiderIPPool
	for i := range poolList.Items {
		if poolList.Items[i].Spec.IPVersion != nil {
			if *poolList.Items[i].Spec.IPVersion == constant.IPv4 {
				v4poolList = append(v4poolList, poolList.Items[i])
			} else {
				v6poolList = append(v6poolList, poolList.Items[i])
			}
		}
	}

	fnScanAll := func(pools []spiderpoolv2beta1.SpiderIPPool) {
		for _, pool := range pools {
			logger.Sugar().Debugf("checking IPPool '%s'", pool.Name)
			poolAllocatedIPs, err := convert.UnmarshalIPPoolAllocatedIPs(pool.Status.AllocatedIPs)
			if nil != err {
				logger.Sugar().Errorf("failed to parse IPPool '%v' status AllocatedIPs, error: %v", pool, err)
				continue
			}

			for poolIP, poolIPAllocation := range poolAllocatedIPs {
				podNS, podName, err := cache.SplitMetaNamespaceKey(poolIPAllocation.NamespacedName)
				if err != nil {
					logger.Error(err.Error())
					continue
				}

				scanAllLogger := logger.With(
					zap.String("podNS", podNS),
					zap.String("podName", podName),
					zap.String("podUID", poolIPAllocation.PodUID),
				)

				podYaml, err := s.podMgr.GetPodByName(ctx, podNS, podName, constant.UseCache)
				if err != nil {
					// case: The pod in IPPool's ip-allocationDetail is not exist in k8s
					if apierrors.IsNotFound(err) {
						wrappedLog := scanAllLogger.With(zap.String("gc-reason", "pod not found in k8s but still exists in IPPool allocation"))
						endpoint, err := s.wepMgr.GetEndpointByName(ctx, podNS, podName, constant.UseCache)
						if nil != err {
							// just continue if we meet other errors
							if !apierrors.IsNotFound(err) {
								wrappedLog.Sugar().Errorf("failed to get SpiderEndpoint: %v", err)
								continue
							}
						} else {
							if s.gcConfig.EnableStatefulSet && endpoint.Status.OwnerControllerType == constant.KindStatefulSet {
								isValidStsPod, err := s.stsMgr.IsValidStatefulSetPod(ctx, podNS, podName, constant.KindStatefulSet)
								if nil != err {
									scanAllLogger.Sugar().Errorf("failed to check StatefulSet pod IP '%s' should be cleaned or not, error: %v", poolIP, err)
									continue
								}
								if isValidStsPod {
									scanAllLogger.Sugar().Warnf("no need to release IP '%s' for StatefulSet pod ", poolIP)
									continue
								}
							}
							if s.gcConfig.EnableKubevirtStaticIP && endpoint.Status.OwnerControllerType == constant.KindKubevirtVMI {
								isValidVMPod, err := s.kubevirtMgr.IsValidVMPod(logutils.IntoContext(ctx, scanAllLogger), podNS, constant.KindKubevirtVMI, endpoint.Status.OwnerControllerName)
								if nil != err {
									scanAllLogger.Sugar().Errorf("failed to check kubevirt vm pod IP '%s' should be cleaned or not, error: %v", poolIP, err)
									continue
								}
								if isValidVMPod {
									scanAllLogger.Sugar().Warnf("no need to release IP '%s' for kubevirt vm pod ", poolIP)
									continue
								}
							}
						}

						wrappedLog.Sugar().Warnf("found IPPool '%s' legacy IP '%s', try to release it", pool.Name, poolIP)
						err = s.releaseSingleIPAndRemoveWEPFinalizer(logutils.IntoContext(ctx, wrappedLog), pool.Name, poolIP, poolIPAllocation)
						if nil != err {
							wrappedLog.Error(err.Error())
						}
						// no matter whether succeed to clean up IPPool IP and SpiderEndpoint, just continue to the next poolIP
						continue
					}

					scanAllLogger.Sugar().Errorf("failed to get pod from kubernetes, error '%v'", err)
					continue
				}

				// check pod status phase with its yaml
				podEntry, err := s.buildPodEntry(nil, podYaml, false)
				if nil != err {
					scanAllLogger.Sugar().Errorf("failed to build podEntry in scanAll, error: %v", err)
					continue
				}

				// case: The pod in IPPool's ip-allocationDetail is also exist in k8s, but the pod is in 'Terminating|Succeeded|Failed' status phase
				if podEntry != nil {
					if time.Now().UTC().After(podEntry.TracingStopTime) {
						wrappedLog := scanAllLogger.With(zap.String("gc-reason", "pod is out of time"))
						err = s.releaseSingleIPAndRemoveWEPFinalizer(logutils.IntoContext(ctx, wrappedLog), pool.Name, poolIP, poolIPAllocation)
						if nil != err {
							wrappedLog.Error(err.Error())
							continue
						}
					} else {
						// otherwise, flush the PodEntry database and let tracePodWorker to solve it if the current controller is elected master.
						if s.leader.IsElected() {
							err = s.PodDB.ApplyPodEntry(podEntry)
							if nil != err {
								scanAllLogger.Error(err.Error())
								continue
							}

							scanAllLogger.With(zap.String("tracing-reason", string(podEntry.PodTracingReason))).
								Sugar().Infof("update podEntry '%s/%s' successfully", podNS, podName)
						}
					}
				} else {
					// case: The pod in IPPool's ip-allocationDetail is also exist in k8s, but the IPPool IP corresponding allocation pod UID is different with pod UID
					if string(podYaml.UID) != poolIPAllocation.PodUID {
						// Once the static IP Pod restarts, it will retrieve the Pod IP from it SpiderEndpoint.
						// So at this moment the Pod UID is different from the IPPool's ip-allocationDetail, we should not release it.
						if podmanager.IsStaticIPPod(s.gcConfig.EnableStatefulSet, s.gcConfig.EnableKubevirtStaticIP, podYaml) {
							scanAllLogger.Sugar().Debugf("Static IP Pod just restarts, keep the static IP '%s' from the IPPool", poolIP)
						} else {
							wrappedLog := scanAllLogger.With(zap.String("gc-reason", "IPPoolAllocation pod UID is different with pod UID"))
							// we are afraid that no one removes the old same name Endpoint finalizer
							err := s.releaseSingleIPAndRemoveWEPFinalizer(logutils.IntoContext(ctx, wrappedLog), pool.Name, poolIP, poolIPAllocation)
							if nil != err {
								wrappedLog.Sugar().Errorf("failed to release ip '%s', error: '%v'", poolIP, err)
								continue
							}
						}
					} else {
						endpoint, err := s.wepMgr.GetEndpointByName(ctx, podYaml.Namespace, podYaml.Name, constant.UseCache)
						if err != nil {
							scanAllLogger.Sugar().Errorf("failed to get Endpoint '%s/%s', error: %v", podYaml.Namespace, podYaml.Name, err)
							continue
						}

						if endpoint.Status.Current.UID == string(podYaml.UID) {
							// case: The pod in IPPool's ip-allocationDetail is also exist in k8s,
							// and the IPPool IP corresponding allocation pod UID is same with Endpoint pod UID, but the IPPool IP isn't belong to the Endpoint IPs
							wrappedLog := scanAllLogger.With(zap.String("gc-reason", "same pod UID but IPPoolAllocation IP is different with Endpoint IP"))
							isBadIP := true
							for _, endpointIP := range endpoint.Status.Current.IPs {
								if *pool.Spec.IPVersion == constant.IPv4 {
									if endpointIP.IPv4 != nil && strings.Split(*endpointIP.IPv4, "/")[0] == poolIP {
										isBadIP = false
									}
								} else {
									if endpointIP.IPv6 != nil && strings.Split(*endpointIP.IPv6, "/")[0] == poolIP {
										isBadIP = false
									}
								}
							}
							if isBadIP {
								// release IP but no need to clean up SpiderEndpoint object
								err = s.ippoolMgr.ReleaseIP(ctx, pool.Name, []types.IPAndUID{{
									IP:  poolIP,
									UID: poolIPAllocation.PodUID},
								})
								if nil != err {
									wrappedLog.Sugar().Errorf("failed to release ip '%s', error: '%v'", poolIP, err)
									continue
								}
								wrappedLog.Sugar().Infof("release ip '%s' successfully!", poolIP)
							}
						}
						// It's impossible that a new IP would be allocated when an old same name Endpoint object exist, because we already avoid it in IPAM
					}
				}
			}
			logger.Sugar().Debugf("task checking IPPool '%s' is completed", pool.Name)
		}
	}

	wg := sync.WaitGroup{}
	if len(v4poolList) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fnScanAll(v4poolList)
		}()
	}

	if len(v6poolList) != 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fnScanAll(v6poolList)
		}()
	}

	wg.Wait()
	logger.Sugar().Debugf("IP GC scan all finished")
}

// releaseSingleIPAndRemoveWEPFinalizer serves for handleTerminatingPod to gc singleIP and remove wep finalizer
func (s *SpiderGC) releaseSingleIPAndRemoveWEPFinalizer(ctx context.Context, poolName, poolIP string, poolIPAllocation spiderpoolv2beta1.PoolIPAllocation) error {
	log := logutils.FromContext(ctx)

	singleIP := []types.IPAndUID{{IP: poolIP, UID: poolIPAllocation.PodUID}}
	err := s.ippoolMgr.ReleaseIP(ctx, poolName, singleIP)
	if nil != err {
		metric.IPGCFailureCounts.Add(ctx, 1)
		return fmt.Errorf("failed to release IP '%s', error: '%v'", poolIP, err)
	}

	metric.IPGCTotalCounts.Add(ctx, 1)
	log.Sugar().Infof("release ip '%s' successfully", poolIP)

	podNS, podName, err := cache.SplitMetaNamespaceKey(poolIPAllocation.NamespacedName)
	if err != nil {
		return err
	}

	endpoint, err := s.wepMgr.GetEndpointByName(ctx, podNS, podName, constant.UseCache)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Sugar().Debugf("SpiderEndpoint '%s/%s' is already cleaned up", podNS, podName)
			return nil
		}
		return err
	}

	// The StatefulSet/KubevirtVM SpiderEndpoint doesn't have ownerRef which can not lead to cascade deletion.
	if endpoint.DeletionTimestamp == nil {
		err := s.wepMgr.DeleteEndpoint(ctx, endpoint)
		if nil != err {
			return err
		}
	}

	if err := s.wepMgr.RemoveFinalizer(ctx, endpoint); err != nil {
		return err
	}

	log.Sugar().Infof("remove SpiderEndpoint '%s/%s' finalizer successfully", podNS, podName)
	return nil
}
