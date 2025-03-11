// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
	corev1 "k8s.io/api/core/v1"
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
	if err != nil {
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
			if err != nil {
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
					zap.String("poolName", pool.Name),
					zap.String("podNS", podNS),
					zap.String("podName", podName),
					zap.String("podUID", poolIPAllocation.PodUID),
				)

				flagGCIPPoolIP := false
				flagGCEndpoint := false
				flagPodStatusShouldGCIP := false
				flagTracePodEntry := false
				flagStaticIPPod := false
				endpoint, endpointErr := s.wepMgr.GetEndpointByName(ctx, podNS, podName, constant.UseCache)
				podYaml, podErr := s.podMgr.GetPodByName(ctx, podNS, podName, constant.UseCache)

				// handle the pod not existed with the same name
				if podErr != nil {
					// case: The pod in IPPool's ip-allocationDetail is not exist in k8s
					if apierrors.IsNotFound(podErr) {
						if endpointErr != nil {
							if apierrors.IsNotFound(endpointErr) {
								scanAllLogger.Sugar().Infof("pod %s/%s does not exist and its endpoint %s/%s cannot be found, only recycle IPPool.Status.AllocatedIPs %s in IPPool %s", podNS, podName, podNS, podName, poolIP, pool.Name)
								flagGCIPPoolIP = true
								flagGCEndpoint = false
								goto GCIP
							} else {
								scanAllLogger.Sugar().Errorf("pod %s/%s does not exist and failed to get endpoint %s/%s, ignore handle IP %s and endpoint, error: '%v'", podNS, podName, podNS, podName, poolIP, endpointErr)
								continue
							}
						} else {
							vaildPod, err := s.isValidStatefulsetOrKubevirt(ctx, scanAllLogger, podNS, podName, poolIP, endpoint.Status.OwnerControllerType)
							if err != nil {
								scanAllLogger.Sugar().Errorf("pod %s/%s does not exist and the pod static type check fails, ignore handle IP %s and endpoint %s/%s, error: %v", podNS, podName, poolIP, podNS, podName, err)
								continue
							}
							if vaildPod {
								scanAllLogger.Sugar().Debugf("pod %s/%s does not exist, but the pod is a valid static pod, ignore handle IP %s and endpoint %s/%s", podNS, podName, poolIP, podNS, podName)
								continue
							} else {
								scanAllLogger.Sugar().Infof("pod %s/%s does not exist and is an invalid static pod. IPPool.Status.AllocatedIPs %s and endpoint %s/%s should be reclaimed", podNS, podName, poolIP, podNS, podName)
								flagGCIPPoolIP = true
								flagGCEndpoint = true
								goto GCIP
							}
						}
					} else {
						scanAllLogger.Sugar().Errorf("failed to get pod from kubernetes, error '%v'", podErr)
						continue
					}
				}

				if podYaml != nil {
					flagStaticIPPod = podmanager.IsStaticIPPod(s.gcConfig.EnableStatefulSet, s.gcConfig.EnableKubevirtStaticIP, podYaml)
				} else {
					scanAllLogger.Sugar().Errorf("podYaml is nil for pod %s/%s", podNS, podName)
					continue
				}

				// check the pod status
				switch {
				case podYaml.Status.Phase == corev1.PodSucceeded || podYaml.Status.Phase == corev1.PodFailed:
					wrappedLog := scanAllLogger.With(zap.String("gc-reason", fmt.Sprintf("The current state of the Pod %s/%s is: %v", podNS, podName, podYaml.Status.Phase)))
					// PodFailed means that all containers in the pod have terminated, and at least one container has
					// terminated in a failure (exited with a non-zero exit code or was stopped by the system).
					// case: When statefulset or kubevirt is restarted, it may enter the failed state for a short time,
					// causing scall all to incorrectly reclaim the IP address, thereby changing the fixed IP address of the static Pod.
					if flagStaticIPPod {
						vaildPod, err := s.isValidStatefulsetOrKubevirt(ctx, scanAllLogger, podNS, podName, poolIP, podYaml.OwnerReferences[0].Kind)
						if err != nil {
							wrappedLog.Sugar().Errorf("pod %s/%s static type check fails, ignore handle IP %s, error: %v", podNS, podName, poolIP, err)
							continue
						}
						if vaildPod {
							wrappedLog.Sugar().Infof("pod %s/%s is a valid static pod, tracking through gc trace", podNS, podName)
							flagPodStatusShouldGCIP = false
							flagTracePodEntry = true
						} else {
							wrappedLog.Sugar().Infof("pod %s/%s is an invalid static Pod. the IPPool.Status.AllocatedIPs %s in IPPool %s should be reclaimed. ", podNS, podName, poolIP, pool.Name)
							flagPodStatusShouldGCIP = true
						}
					} else {
						wrappedLog.Sugar().Infof("pod %s/%s is not a static Pod. the IPPool.Status.AllocatedIPs %s in IPPool %s should be reclaimed. ", podNS, podName, poolIP, pool.Name)
						flagPodStatusShouldGCIP = true
					}
				case podYaml.Status.Phase == corev1.PodPending:
					// PodPending means the pod has been accepted by the system, but one or more of the containers
					// has not been started. This includes time before being bound to a node, as well as time spent
					// pulling images onto the host.
					scanAllLogger.Sugar().Debugf("The Pod %s/%s status is %s , and the IP %s should not be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP)
					flagPodStatusShouldGCIP = false
				case podYaml.DeletionTimestamp != nil:
					podTracingGracefulTime := (time.Duration(*podYaml.DeletionGracePeriodSeconds) + time.Duration(s.gcConfig.AdditionalGraceDelay)) * time.Second
					podTracingStopTime := podYaml.DeletionTimestamp.Time.Add(podTracingGracefulTime)
					if time.Now().UTC().After(podTracingStopTime) {
						scanAllLogger.Sugar().Infof("the graceful deletion period of pod '%s/%s' is over, try to reclaim the IP %s in the IPPool %s.", podNS, podName, poolIP, pool.Name)
						flagPodStatusShouldGCIP = true
					} else {
						wrappedLog := scanAllLogger.With(zap.String("gc-reason", "The graceful deletion period of kubernetes Pod has not yet ended"))
						if len(podYaml.Status.PodIPs) != 0 {
							wrappedLog.Sugar().Infof("pod %s/%s still holds the IP address %v. try to track it through trace GC.", podNS, podName, podYaml.Status.PodIPs)
							flagPodStatusShouldGCIP = false
							// The graceful deletion period of kubernetes Pod has not yet ended, and the Pod's already has an IP address. Let trace_worker track and recycle the IP in time.
							// In addition, avoid that all trace data is blank when the controller is just started.
							flagTracePodEntry = true
						} else {
							wrappedLog.Sugar().Infof("pod %s/%s IP has been reclaimed, try to reclaim the IP %s in IPPool %s", podNS, podName, poolIP, pool.Name)
							flagPodStatusShouldGCIP = true
						}
					}
				default:
					wrappedLog := scanAllLogger.With(zap.String("gc-reason", fmt.Sprintf("The current state of the Pod %s/%s is: %v", podNS, podName, podYaml.Status.Phase)))
					if len(podYaml.Status.PodIPs) != 0 {
						// pod is running, pod has been assigned IP address
						wrappedLog.Sugar().Debugf("pod %s/%s has been assigned IP address %v, ignore handle IP %s", podNS, podName, podYaml.Status.PodIPs, poolIP)
						flagPodStatusShouldGCIP = false
					} else {
						if flagStaticIPPod {
							vaildPod, err := s.isValidStatefulsetOrKubevirt(ctx, scanAllLogger, podNS, podName, poolIP, podYaml.OwnerReferences[0].Kind)
							if err != nil {
								wrappedLog.Sugar().Errorf("pod %s/%s has no IP address assigned and the pod static type check fails, ignore handle IP %s, error: %v", podNS, podName, poolIP, err)
								continue
							}
							if vaildPod {
								wrappedLog.Sugar().Debugf("pod %s/%s has no IP address assigned, but is a valid static pod, ignore handle IP %s", podNS, podName, poolIP)
								flagPodStatusShouldGCIP = false
							} else {
								wrappedLog.Sugar().Infof("pod %s/%s has no IP address assigned and it is a invalid static Pod. the IPPool.Status.AllocatedIPs %s in IPPool should be reclaimed. ", podNS, podName, poolIP)
								flagPodStatusShouldGCIP = true
							}
						} else {
							wrappedLog.Sugar().Infof("pod %s/%s has no IP address assigned and is not a static Pod. IPPool.Status.AllocatedIPs %s in IPPool should be reclaimed.", podNS, podName, poolIP)
							flagPodStatusShouldGCIP = true
						}
					}
				}

				// The goal is to promptly reclaim IP addresses and to avoid having all trace data being blank when the spiderppol controller has just started or during a leader election.
				if flagTracePodEntry && s.leader.IsElected() {
					scanAllLogger.Sugar().Debugf("The spiderppol controller pod might have just started or is undergoing a leader election, and is tracking pods %s/%s in the graceful termination phase via trace_worker.", podNS, podName)
					// check pod status phase with its yaml
					podEntry, err := s.buildPodEntry(nil, podYaml, false)
					if err != nil {
						scanAllLogger.Sugar().Errorf("failed to build podEntry in scanAll, error: %v", err)
					} else {
						err = s.PodDB.ApplyPodEntry(podEntry)
						if err != nil {
							scanAllLogger.Sugar().Errorf("failed to refresh PodEntry database in scanAll, error: %v", err.Error())
						} else {
							scanAllLogger.With(zap.String("tracing-reason", string("the spiderppol controller pod might have just started or is undergoing a leader election."))).
								Sugar().Infof("update podEntry '%s/%s' successfully", podNS, podName)
						}
					}
				}

				// handle same name pod with different uid in the ippool
				if string(podYaml.UID) != poolIPAllocation.PodUID {
					wrappedLog := scanAllLogger.With(zap.String("gc-reason", fmt.Sprintf("Pod: %s/%s UID %s is different from IPPool: %s UID %s", podNS, podName, podYaml.UID, pool.Name, poolIPAllocation.PodUID)))
					if flagStaticIPPod {
						// Check if the status.ips of the current K8S Pod has a value.
						// If there is a value, it means that the pod has been started and the IP has been successfully assigned through cmdAdd
						// If there is no value, it means that the new pod is still starting.
						if len(podYaml.Status.PodIPs) != 0 {
							wrappedLog.Sugar().Infof("pod %s/%s is a static Pod with a status of %v and has been assigned an different IP address, the IPPool.Status.AllocatedIPs %s in IPPool should be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP)
							flagGCIPPoolIP = true
						} else {
							vaildPod, err := s.isValidStatefulsetOrKubevirt(ctx, scanAllLogger, podNS, podName, poolIP, podYaml.OwnerReferences[0].Kind)
							if err != nil {
								wrappedLog.Sugar().Errorf("failed to check pod static type, ignore handle IP %s, error: %v", poolIP, err)
								continue
							}
							if vaildPod {
								wrappedLog.Sugar().Debugf("pod %s/%s is a valid static Pod with a status of %v and no IP address assigned. the IPPool.Status.AllocatedIPs %s in IPPool %s should not be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP, pool.Name)
								flagGCIPPoolIP = false
							} else {
								scanAllLogger.Sugar().Infof("pod %s/%s is an invalid static Pod with a status of %v and no IP address assigned. the IPPool.Status.AllocatedIPs %s in IPPool %s should be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP, pool.Name)
								flagGCIPPoolIP = true
							}
						}
					} else {
						wrappedLog.Sugar().Infof("pod %s/%s is not a static Pod with a status of %v, the IPPool.Status.AllocatedIPs %s in IPPool %s should be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP, pool.Name)
						flagGCIPPoolIP = true
					}
				} else {
					if flagPodStatusShouldGCIP {
						scanAllLogger.Sugar().Infof("pod %s/%s status is: %s, the IPPool.Status.AllocatedIPs %s in IPPool %s should be reclaimed", podNS, podName, podYaml.Status.Phase, poolIP, pool.Name)
						flagGCIPPoolIP = true
					} else {
						scanAllLogger.Sugar().Debugf("pod %s/%s status is: %s, and Pod UID %s is the same as IPPool UID %s, the IPPool.Status.AllocatedIPs %s in IPPool %s should not be reclaimed",
							podNS, podName, podYaml.Status.Phase, podYaml.UID, poolIPAllocation.PodUID, poolIP, pool.Name)
					}
				}

				// handle the endpoint
				if endpointErr != nil {
					if apierrors.IsNotFound(endpointErr) {
						scanAllLogger.Sugar().Debugf("SpiderEndpoint %s/%s does not exist, ignore it", podNS, podName)
						flagGCEndpoint = false
					} else {
						scanAllLogger.Sugar().Errorf("failed to get SpiderEndpoint %s/%s, ignore handle SpiderEndpoint, error: %v", podNS, podName, err)
						flagGCEndpoint = false
					}
				} else {
					// handle same name pod with different uid in the endpoint
					if string(podYaml.UID) != endpoint.Status.Current.UID {
						wrappedLog := scanAllLogger.With(zap.String("gc-reason", fmt.Sprintf("Pod:%s/%s UID %s is different from endpoint:%s/%s UID %s", podNS, podName, podYaml.UID, endpoint.Namespace, endpoint.Name, poolIPAllocation.PodUID)))
						if flagStaticIPPod {
							// Check if the status.ips of the current K8S Pod has a value.
							// If there is a value, it means that the pod has been started and the IP has been successfully assigned through cmdAdd
							// If there is no value, it means that the new pod is still starting.
							if len(podYaml.Status.PodIPs) != 0 {
								wrappedLog.Sugar().Infof("pod %s/%s is a static Pod with a status of %v and has been assigned an different IP address, the endpoint %v/%v should be reclaimed", podNS, podName, poolIP)
								flagGCEndpoint = true
							} else {
								vaildPod, err := s.isValidStatefulsetOrKubevirt(ctx, scanAllLogger, podNS, podName, poolIP, podYaml.OwnerReferences[0].Kind)
								if err != nil {
									wrappedLog.Sugar().Errorf("failed to check pod static type, ignore handle endpoint %s, error: %v", endpoint.Namespace, endpoint.Name, err)
									continue
								}
								if vaildPod {
									wrappedLog.Sugar().Debugf("pod %s/%s is a valid static Pod with a status of %v and no IP address assigned. the endpoint %v/%v should not be reclaimed", podNS, podName, podYaml.Status.Phase, endpoint.Namespace, endpoint.Name)
									flagGCEndpoint = false
								} else {
									scanAllLogger.Sugar().Infof("pod %s/%s is an invalid static Pod with a status of %v and no IP address assigned. the endpoint %v/%v should be reclaimed", podNS, podName, podYaml.Status.Phase, endpoint.Namespace, endpoint.Name)
									flagGCEndpoint = true
								}
							}
						} else {
							wrappedLog.Sugar().Infof("pod %s/%s is not a static Pod with a status of %v, the endpoint %v/%v should be reclaimed", podNS, podName, podYaml.Status.Phase, endpoint.Namespace, endpoint.Name)
							flagGCIPPoolIP = true
							flagGCEndpoint = true
						}
					} else {
						if flagPodStatusShouldGCIP {
							scanAllLogger.Sugar().Infof("pod %s/%s status is: %s, the endpoint %v/%v should be reclaimed ", podNS, podName, podYaml.Status.Phase, endpoint.Namespace, endpoint.Name)
							flagGCEndpoint = true
						} else {
							scanAllLogger.Sugar().Debugf("pod %s/%s status is: %s, and Pod UID %s is the same as endpoint UID %s, the endpoint %v/%v should not be reclaimed ",
								podNS, podName, podYaml.Status.Phase, podYaml.UID, endpoint.Status.Current.UID, podNS, podName)
						}
					}
				}

			GCIP:
				if flagGCIPPoolIP {
					err = s.ippoolMgr.ReleaseIP(ctx, pool.Name, []types.IPAndUID{{
						IP:  poolIP,
						UID: poolIPAllocation.PodUID},
					})
					if err != nil {
						scanAllLogger.Sugar().Errorf("failed to release ip '%s' in IPPool: %s, error: '%v'", poolIP, pool.Name, err)
					} else {
						scanAllLogger.Sugar().Infof("scan all successfully reclaimed the IP %s in IPPool: %s", poolIP, pool.Name)
					}
				}
				if flagGCEndpoint {
					err = s.wepMgr.ReleaseEndpointAndFinalizer(logutils.IntoContext(ctx, scanAllLogger), podNS, podName, constant.UseCache)
					if nil != err {
						scanAllLogger.Sugar().Errorf("failed to remove SpiderEndpoint '%s/%s', error: '%v'", podNS, podName, err)
					} else {
						scanAllLogger.Sugar().Infof("scan all successfully reclaimed SpiderEndpoint %s/%s", podNS, podName)
					}
				}
			}
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

// Helps check if it is a valid static Pod (StatefulSet or Kubevirt), if it is a valid static Pod. Return true
func (s *SpiderGC) isValidStatefulsetOrKubevirt(ctx context.Context, logger *zap.Logger, podNS, podName, poolIP, ownerControllerType string) (bool, error) {
	if s.gcConfig.EnableStatefulSet && ownerControllerType == constant.KindStatefulSet {
		isValidStsPod, err := s.stsMgr.IsValidStatefulSetPod(ctx, podNS, podName, constant.KindStatefulSet)
		if err != nil {
			logger.Sugar().Errorf("failed to check if StatefulSet pod IP '%s' should be cleaned or not, error: %v", poolIP, err)
			return true, err
		}
		if isValidStsPod {
			logger.Sugar().Warnf("no need to release IP '%s' for StatefulSet pod", poolIP)
			return true, nil
		}
	}

	if s.gcConfig.EnableKubevirtStaticIP && ownerControllerType == constant.KindKubevirtVMI {
		isValidVMPod, err := s.kubevirtMgr.IsValidVMPod(ctx, podNS, constant.KindKubevirtVMI, podName)
		if err != nil {
			logger.Sugar().Errorf("failed to check if KubeVirt VM pod IP '%s' should be cleaned or not, error: %v", poolIP, err)
			return true, err
		}
		if isValidVMPod {
			logger.Sugar().Warnf("no need to release IP '%s' for KubeVirt VM pod", poolIP)
			return true, nil
		}
	}

	return false, nil
}
