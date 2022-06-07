// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/*
	需要gc的地方:
	一. k8s中不存在的pod对象，但是存在于ippool里的ip-allocationDetail里
	二. k8s中的pod对象使用ip1，但是在ippool的ip-allocationDetail中使用了错误的ip2
	三. k8s中的pod对象使用ip1，但是在ippool的ip-allocationDetail中占用了ip1，ip2等多个ip --》 kubelet调用多次cni，但可以用podName+containerID识别
*/

// TODO 环境变量或configmap
var (
	// 这个应该大一些，追踪那些进入terminating｜succeeded｜failed的pod，因为每个都要等到优雅时间后才能开始正常工作，因此会出现事件堆积的情况
	TRACE_POD_WORKER_NUM = 5
	// 真正执行ippool里面ip释放的worker数量
	RELEASE_IPPOOLIP_WORKER_NUM = 1
	// 真正执行wep对象释放的worker数量
	RELEASE_WEP_WORKER_NUM = 1
	// 如果是succeed｜failed事件，需要先等一下，免得与cmdDel竞争
	SucceedOrFailedWaitDuration = 30
	// 默认gcAll扫描时间
	DEFAULT_GC_INTERVAL_DURATION = 60

	// 初始化channel tracePodSignal的缓冲大小
	tracePodSignalBuffer = 1024
	// 初始化channel wepGCSignal的缓冲大小
	gcWEPSignalBuffer = 1024
	// 初始化channel gcIPPoolIPSignal的缓冲大小
	gcIPPoolIPSignalBuffer = 512
)

var logger = logutils.Logger.Named("garbage-collection")

type GCManager interface {
	Start(ctx context.Context)

	// for CMD use
	GetPodDatabase()

	// for CMD use
	TriggerGCAll()

	Health()
}

var _ GCManager = &SpiderGC{}

type SpiderGC struct {
	KClient client.Client
	PodDB   PodDBer

	ControllerManager ctrl.Manager

	// 默认触发gc扫时间
	DefaultGCIntervalDuration time.Duration

	// signal
	gcAllSignal      chan struct{}
	gcCliSignal      chan struct{}
	gcIPPoolIPSignal chan gcIPPoolIPIdentify
	gcWEPSignal      chan gcWEPIdentify
	tracePodSignal   chan tracePodIdentify

	// PodManager
	// IPPoolManager
	// WEPManager
}

func NewGCManager(client client.Client, mgr ctrl.Manager) GCManager {
	spiderGC := &SpiderGC{
		KClient:                   client,
		PodDB:                     NewPodDBer(),
		ControllerManager:         mgr,
		DefaultGCIntervalDuration: time.Duration(DEFAULT_GC_INTERVAL_DURATION),
		gcAllSignal:               make(chan struct{}),
		gcCliSignal:               make(chan struct{}),
		gcIPPoolIPSignal:          make(chan gcIPPoolIPIdentify, gcIPPoolIPSignalBuffer),
		gcWEPSignal:               make(chan gcWEPIdentify, gcWEPSignalBuffer),
		tracePodSignal:            make(chan tracePodIdentify, tracePodSignalBuffer),
	}

	return spiderGC
}

func (s *SpiderGC) Start(ctx context.Context) {
	// 1. 注册pod controller并启动reconcile watch
	err := s.registerPodReconcile()
	if nil != err {
		logger.Sugar().Fatalf("error: Register pod reconcile failed %v", err)
	}

	for i := 0; i < TRACE_POD_WORKER_NUM; i++ {
		go s.tracePodWorker(ctx)
	}

	for i := 0; i < RELEASE_IPPOOLIP_WORKER_NUM; i++ {
		go s.releaseIPPoolIPWorker(ctx)
	}

	for i := 0; i < RELEASE_WEP_WORKER_NUM; i++ {
		go s.releaseWEPWorker(ctx)
	}

	go s.monitorCLISignal(ctx)
	go s.monitorDefaultGCInterval(ctx)

	logger.Info("Running gc...")
}

func (s *SpiderGC) GetPodDatabase() {
	//TODO implement me
	panic("implement me")
}

func (s *SpiderGC) TriggerGCAll() {
	s.gcCliSignal <- struct{}{}
}

func (s *SpiderGC) Health() {
	//TODO implement me
	panic("implement me")
}

// ----------------------------------------------------------------------------------------------------

// registerPodReconcile registers watch pod
func (s *SpiderGC) registerPodReconcile() error {
	c, err := controller.New("watch-pod-controller", s.ControllerManager, controller.Options{
		Reconciler: &reconcilePod{
			spiderGC: s,
		},
	})
	if nil != err {
		return err
	}

	// TODO 后面的predicate用于过滤，是否需要Genric事件？
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
	if nil != err {
		return err
	}

	return nil
}

// executeScanAllPod scans the whole pod and whole IPPoolList
func (s *SpiderGC) executeScanAllPod(ctx context.Context) {
	// 查出来的
	podList := &corev1.PodList{}
	poolList := &v1.IPPoolList{}

	// 场景1：k8s中不存在的pod对象，但是存在于ippool里的ip-allocationDetail里
	var foundInPod string
	for _, pool := range poolList.Items {
		for ippoolIP, poolIPAllocation := range pool.Status.AllocatedIPs {
			for _, pod := range podList.Items {

				// ippool记录中的pod在k8s中有
				if poolIPAllocation.Pod == pod.Name && poolIPAllocation.Namespace == pod.Namespace {
					foundInPod = pod.Name

					s.filterIP(ctx, &pod, pool, ippoolIP)
					break
				}
			}

			// 场景1：k8s中不存在的pod对象，但是存在于ippool里的ip-allocationDetail里
			if len(foundInPod) == 0 {
				s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{
					Pool:         &pool,
					IP:           ippoolIP,
					PodName:      poolIPAllocation.Pod,
					PodNamespace: poolIPAllocation.Namespace,
					IsReleaseWEP: true,
				}
				continue
			}

			// 找到后就置空，等下一个ippoolIP
			foundInPod = ""
		}
	}

	return
}

// filterIP check the given pod whether matches the ippoolIP, if not then gc the bad ippoolIP
func (s *SpiderGC) filterIP(ctx context.Context, pod *corev1.Pod, pool v1.IPPool, ippoolIP string) {
	// 场景2：防止watch遗漏? (ippool记录中的pod在k8s中有)：k8s中pod存在，pod属于terminating状态,
	// 如果cache里面有记录且状态已经标志为terminating则表明已经被追踪，否则就更新cache且发信号
	if nil != pod.DeletionTimestamp {
		podCache, err := s.PodDB.Get(pod.Name, pod.Namespace)
		if nil != err {
			// 1.有可能已经被gc  2.有可能watch丢失. 如果是已经被gc，没关系重做一次，后续逻辑都能正常。
			podEntry, err := s.PodDB.Create(pod)
			if nil != err {
				// 实际上此处就不可能出现err的，哪怕有也无所谓。等下一轮gcAll继续处理
				logger.Sugar().Errorf("error: create pod '%+v' cache  failed '%v', concurrency conflicts", types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}, err)
				return
			}

			s.tracePodSignal <- tracePodIdentify{
				PodCache: podEntry,
				PodIP:    ippoolIP,
				PoolName: pool.Name,
			}
			return
		}

		if nil != podCache.TerminatingStartTime {
			// 表明已被追踪，不关注
			return
		}

		// Update过程中，如果watch那边已经执行完了gc，导致内存数据已经没数据了-->那就直接忽略skip
		podCache, err = s.PodDB.Update(pod)
		if nil != err {
			return
		}
		// 发信号去GC， 这里会监听gracePeriod，超时后内存里找不到podCache则表明其他地方gc成功，否则gc
		s.tracePodSignal <- tracePodIdentify{
			PodCache: podCache,
			PodIP:    ippoolIP,
			PoolName: pool.Name,
		}
		return
	}

	// 场景3： 防止watch遗漏 (ippool记录中的pod在k8s中有)，且Pod.status.phase!=pending (pending表示pod可能在creating或者deleting)
	// 需要检查:
	// 3.1：能查询到wep记录，且其中的currentIP与ippoolIP不一致，则需要回收ippoolIP，不需要回收wep -->对应 kubelet多次调用cni，拿到了多个ip，但containerID变了
	// 3.2：phase == Succeeded || Failed, 查询podDB，
	//		若其中已经记录了该pod且正确Succeeded，则不关注(watch已经跟踪)
	//		否则,检查podDB,如果缓存里没有表明没人追踪他，那就直接清理掉

	// 3.2场景
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		podCache, err := s.PodDB.Get(pod.Name, pod.Namespace)
		if nil != err {
			// 1.有可能已经被gc  2.有可能watch丢失
			// 那就直接重复发信号去gc. 之所以terminating是去发pod追踪信号是因为想让cmdDel先处理。过了优雅时间我们再处理。此处状态直接干活.
			s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{
				Pool:         &pool,
				IP:           ippoolIP,
				PodName:      pod.Name,
				PodNamespace: pod.Namespace,
				IsReleaseWEP: true,
			}
			return
		}
		if podCache.PodPhase == corev1.PodSucceeded || podCache.PodPhase == corev1.PodFailed {
			// 内存中查到Succeed|Failed状态则表明已经被watch追踪到了，skip
			return
		}
		// 否则其他状态说明watch比此gcAll慢了，直接删
		s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{
			Pool:         &pool,
			IP:           ippoolIP,
			PodName:      pod.Name,
			PodNamespace: pod.Namespace,
			IsReleaseWEP: true,
		}
		return
	}

	// 3.1场景
	wep := v1.WorkloadEndpoint{}
	err := s.KClient.Get(ctx, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}, &wep)
	if nil != err {
		// 报个错，降级，直接下一轮gcAll处理
		logger.Sugar().Errorf("get workloadendpoint '%+v' failed in filterIP phase.", types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
		return
	}

	isSameWithWEP := false
	// 不使用current是因为cmdDel时会移除掉current
	if len(wep.Status.History) != 0 {
		for _, wepCurrentAllocationDetail := range wep.Status.History[0].IPs {
			// 如果currentIP与ippoolIP一致，则表明没问题
			if ippoolIP == *wepCurrentAllocationDetail.IPv4 || ippoolIP == *wepCurrentAllocationDetail.IPv6 {
				isSameWithWEP = true
				break
			}
		}
	}

	if !isSameWithWEP {
		//不一致，就回收ippoolIP,但不需要回收wep
		s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{
			Pool:         &pool,
			IP:           ippoolIP,
			PodName:      pod.Name,
			PodNamespace: pod.Namespace,
			IsReleaseWEP: false,
		}
	}

	return
}

// 监听信号，如CLI, DefaultGCInterval
func (s *SpiderGC) monitorCLISignal(ctx context.Context) {
	for {
		select {
		case <-s.gcCliSignal:
			s.executeScanAllPod(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// DefaultGCInterval 主模式下，会定期扫. 之所以不与monitorCLISignal一起，是因为select并发场景，case会随机选一个
func (s *SpiderGC) monitorDefaultGCInterval(ctx context.Context) {
IS_LEADER:
	<-s.ControllerManager.Elected()

	for {
		select {
		case <-time.After(s.DefaultGCIntervalDuration):
			s.executeScanAllPod(ctx)
			goto IS_LEADER
		case <-ctx.Done():
			return
		}
	}
}

// release ippool实例中的ip-allocation键值对
type gcIPPoolIPIdentify struct {
	Pool         *v1.IPPool
	IP           string
	PodName      string
	PodNamespace string
	IsReleaseWEP bool
}

// GC IPPool中的ip
func (s *SpiderGC) releaseIPPoolIPWorker(ctx context.Context) {
	for {
		select {
		case gcIPPoolIP := <-s.gcIPPoolIPSignal:
			err := s.leaderReleaseCR(ctx, &gcIPPoolIP)
			if nil != err {
				logger.With(
					zap.String("IPPool", gcIPPoolIP.Pool.Name),
					zap.String("Pod", gcIPPoolIP.PodName),
					zap.String("PodNs", gcIPPoolIP.PodNamespace),
				).Sugar().Error(err)

				continue
			}
			// 释放内存pod数据库
			s.PodDB.Delete(gcIPPoolIP.PodName, gcIPPoolIP.PodNamespace)

		case <-ctx.Done():
			logger.Warn("ctx done for 'release IPPoolIP worker'...")
			return
		}
	}
}

// 只有leader才能真正的发release IPPoolIP和WEP的信号
func (s *SpiderGC) leaderReleaseCR(ctx context.Context, gcIPPoolIP *gcIPPoolIPIdentify) error {
	// 只有主才能去真正的修改CR
	select {
	case <-s.ControllerManager.Elected():
		// 更新ippool实例(删除掉实例里面记录的ip-allocation键值对)
		pool := gcIPPoolIP.Pool
		delete(pool.Status.AllocatedIPs, gcIPPoolIP.IP)
		err := s.KClient.Update(ctx, pool)
		if nil != err {
			return fmt.Errorf("error: gc IP '%s' failed: '%v' ", gcIPPoolIP.IP, err)
		}

		// 释放wep
		if gcIPPoolIP.IsReleaseWEP {
			s.gcWEPSignal <- gcWEPIdentify{
				WepName:   gcIPPoolIP.PodName,
				NameSpace: gcIPPoolIP.PodNamespace,
			}
		}

		// 日志
		logger.With(
			zap.String("IPPool", gcIPPoolIP.Pool.Name),
			zap.String("Pod", gcIPPoolIP.PodName),
			zap.String("PodNs", gcIPPoolIP.PodNamespace),
		).Sugar().Infof("gc IP '%s' successfully", gcIPPoolIP.IP)
	default:
	}

	return nil
}

// GC WEP CRD实例
type gcWEPIdentify struct {
	WepName   string
	NameSpace string
}

// GC WEP CRD实例.
func (s *SpiderGC) releaseWEPWorker(ctx context.Context) {
	for {
		select {
		case gcWep := <-s.gcWEPSignal:
			var wep v1.WorkloadEndpoint
			err := s.KClient.Get(ctx, types.NamespacedName{Namespace: gcWep.NameSpace, Name: gcWep.WepName}, &wep)
			if apierrors.IsNotFound(err) {
				logger.Sugar().Warnf("workloadendpoint '%+v' not found.", types.NamespacedName{Namespace: gcWep.NameSpace, Name: gcWep.WepName})
				continue
			}
			if nil != err {
				logger.Sugar().Errorf("error: get workloadendpoint '%+v' failed: '%v'", types.NamespacedName{Namespace: gcWep.NameSpace, Name: gcWep.WepName}, err)
				continue
			}

			// 移除finalizer
			wep.Finalizers = removeSpiderFinalizer(wep.Finalizers, "************")
			err = s.KClient.Update(ctx, &wep)
			if nil != err {
				// TODO
			}

		case <-ctx.Done():
			return
		}
	}
}

// removeSpiderFinalizer removes a specific finalizer field in finalizers string array.
func removeSpiderFinalizer(finalizers []string, field string) []string {
	newFinalizers := []string{}
	for _, finalizer := range finalizers {
		if finalizer == field {
			continue
		}
		newFinalizers = append(newFinalizers, finalizer)
	}

	return newFinalizers
}

// 追踪进入terminating状态的pod标识
type tracePodIdentify struct {
	PodCache *PodEntry
	PodIP    string
	PoolName string
}

// tracePodWorker用于接收追踪terminatingPod, deletedPod || SucceedPod || FailedPod的信号
func (s *SpiderGC) tracePodWorker(ctx context.Context) {
	for {
		select {
		case tracePod := <-s.tracePodSignal:
			err := s.tracePodTask(ctx, tracePod)
			if nil != err {
				logger.Error(err.Error())
			}

		case <-ctx.Done():
			logger.Warn("ctx done for 'trace Pod worker'...")
			return
		}
	}
}

// tracePodTask追踪一个pod，并决定是否真的去gc
func (s *SpiderGC) tracePodTask(ctx context.Context, tracePod tracePodIdentify) error {
	if tracePod.PodCache.TerminatingStopTime != nil {
		for {
			if time.Now().UTC().After(*tracePod.PodCache.TerminatingStopTime) {
				_, err := s.PodDB.Get(tracePod.PodCache.PodName, tracePod.PodCache.Namespace)
				if nil != err {
					// 过了超时时间后，不存在，表明已经gc过了
					return nil
				}
				// 还存在，表明需要去执行gc
				break
			}
			// 免得一直在那里for循环判断
			time.Sleep(time.Second)
		}
	}

	// TODO 对于Succeed|Failed，是否需要等待一下呢？让cmdDel先处理，免得跟他有并发冲突. 不像terminating还有优雅时间等一下. 该处状态由watch那边发信号
	// 且，这两个状态并没有DeletionTimestamp,因此可以自定义一个时间段？
	if tracePod.PodCache.PodPhase == corev1.PodSucceeded || tracePod.PodCache.PodPhase == corev1.PodFailed {
		time.Sleep(time.Second * time.Duration(SucceedOrFailedWaitDuration))
	}

	// 寻找pool
	pool, err := s.findPoolWithPoolName(ctx, tracePod.PoolName)
	if apierrors.IsNotFound(err) {
		s.PodDB.Delete(tracePod.PodCache.PodName, tracePod.PodCache.Namespace)
		return fmt.Errorf("error: no ippool '%s' CR object in trace pod phase", tracePod.PoolName)
	}

	if nil != err {
		return fmt.Errorf("error: get ippool '%s' failed '%v' in trace pod phase", tracePod.PoolName, err)
	}

	// 优雅时间超过后，如果IPPoolIP已经没了，表明被cmdDel干掉了
	ipCorrespondingPod, ok := pool.Status.AllocatedIPs[tracePod.PodIP]
	if !ok {
		//已被cmdDel gc完了
		s.PodDB.Delete(tracePod.PodCache.PodName, tracePod.PodCache.Namespace)
		return nil
	}

	// TODO Terminating｜Succeeded｜Failed的pod才会进到该tracePod.  似乎都不需要再考虑containerID了.
	// 但是后续releaseIPPoolIPWoker时候会出现一个场景。gcall与watch并发情况下，会多个信号。如果某个已经gc成功，且该ip立刻被其他人占用。后一个信号就会出现错误gc的情况!!!!!!!
	if ipCorrespondingPod.Pod == tracePod.PodCache.PodName && ipCorrespondingPod.Namespace == tracePod.PodCache.Namespace {
		// Deleted || Succeed || Failed状态直接删
		s.gcIPPoolIPSignal <- gcIPPoolIPIdentify{
			Pool:         pool,
			IP:           tracePod.PodIP,
			PodName:      tracePod.PodCache.PodName,
			PodNamespace: tracePod.PodCache.Namespace,
			IsReleaseWEP: true,
		}
	}

	return nil
}

// 根据poolName查询到对应的ippool cr对象
func (s *SpiderGC) findPoolWithPoolName(ctx context.Context, poolName string) (*v1.IPPool, error) {
	pool := &v1.IPPool{}
	err := s.KClient.Get(ctx, types.NamespacedName{Name: poolName}, pool)
	if nil != err {
		return nil, err
	}

	return pool, nil
}
