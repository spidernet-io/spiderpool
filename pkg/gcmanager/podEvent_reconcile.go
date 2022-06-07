// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package gcmanager

import (
	"context"
	"fmt"

	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO: 注意reconcile最后return的error都交给controller-runtime的log来实现，
// 但是，log似乎没有实现？？？因此是否还需要return详细信息？

var _ reconcile.Reconciler = &reconcilePod{}

type reconcilePod struct {
	spiderGC *SpiderGC
}

// 事件Create, Update, Delete. 	如果是选主leader --> 可以派发任务. 注意，此处更新内存数据库会与gcall产生并发冲突
// notice: if reconcile received an error, then the correspond request will requeue.
func (r *reconcilePod) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Pod form the cache
	pod := &corev1.Pod{}
	err := r.spiderGC.KClient.Get(ctx, request.NamespacedName, pod)

	// if not found --> delete event
	if apierrors.IsNotFound(err) {
		logger.Sugar().Errorf("Could not find '%+v' in k8s", request.NamespacedName)

		// TODO：是否需要更新一下内存数据库，更改他的phase为自定义的"DELETED"以供展示？因为等到gc完成需要一段时间
		podEntry, err := r.spiderGC.PodDB.Get(request.Name, request.Namespace)
		if nil != err {
			// 没找到pod cache，则表明已经GC过了
			logger.Warn(err.Error())
			return reconcile.Result{}, nil
		}

		if podEntry.PodPhase == corev1.PodSucceeded || podEntry.PodPhase == corev1.PodFailed || podEntry.TerminatingStartTime != nil {
			// 前面已追踪过
			return reconcile.Result{}, nil
		}

		// 此场景为，k8s已经删了，但是内存数据库中还有pod，且状态为Pending(creating), Running, Unknown这三种
		wep := &v1.WorkloadEndpoint{}
		err = r.spiderGC.KClient.Get(ctx, request.NamespacedName, wep)
		if apierrors.IsNotFound(err) {
			// 当wep找不到，是否说就已经完全gc完了？这时候把内存数据库给给清除就ok
			logger.Sugar().Warnf("no found wep '%s' in namespace '%s'", request.Name, request.Namespace)
			r.spiderGC.PodDB.Delete(request.Name, request.Namespace)
			return reconcile.Result{}, nil
		}

		// return err and make this request requeue.
		if nil != err {
			return reconcile.Result{}, err
		}

		sendTracePodSignalWithWEPHistory(r.spiderGC.tracePodSignal, podEntry, wep)

		return reconcile.Result{}, nil
	}

	// return err and make this request requeue.
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch Pod: '%v' with error: %v", request.NamespacedName, err)
	}

	// pod create event  --> 1.新建pod  2.重启controller时候内存消失，也是走这里
	_, err = r.spiderGC.PodDB.Get(pod.Name, pod.Namespace)
	if nil != err {
		// 没有就新建
		logger.Warn(err.Error())
		podCache, err := r.spiderGC.PodDB.Create(pod)
		if nil != err {
			return reconcile.Result{}, fmt.Errorf("error: create pod cache '%+v' failed '%v', concurrency conflicts", request.NamespacedName, err)
		}

		err = r.filterDyingPod(ctx, pod, podCache, request)
		if nil != err {
			return reconcile.Result{}, fmt.Errorf("error: filter dying pod '%+v' failed '%v'", request.NamespacedName, err)
		}

		return reconcile.Result{}, nil
	}

	// pod update event
	podEntry, err := r.spiderGC.PodDB.Update(pod)
	if nil != err {
		logger.Sugar().Errorf("error: update pod cache '%v' failed '%v', concurrency conflicts", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	err = r.filterDyingPod(ctx, pod, podEntry, request)
	if nil != err {
		return reconcile.Result{}, fmt.Errorf("error: filter dying pod '%+v' failed '%v'", request.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconcilePod) filterDyingPod(ctx context.Context, pod *corev1.Pod, podCache *PodEntry, request reconcile.Request) error {
	// Pod.status.phase == Succeed || Failed此类场景  或者terminating场景
	if (pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed) || pod.ObjectMeta.DeletionTimestamp != nil {
		wep := &v1.WorkloadEndpoint{}
		err := r.spiderGC.KClient.Get(ctx, request.NamespacedName, wep)
		if apierrors.IsNotFound(err) {
			// 说明cmdDel已经完成了gc,即可退出
			r.spiderGC.PodDB.Delete(pod.Name, pod.Namespace)
			return nil
		}

		if nil != err {
			// 重新进队列
			return err
		}

		sendTracePodSignalWithWEPHistory(r.spiderGC.tracePodSignal, podCache, wep)
	}

	return nil
}

// 将history的所有记录，全部拿去追踪
func sendTracePodSignalWithWEPHistory(tracePodSignal chan<- tracePodIdentify, podCache *PodEntry, wep *v1.WorkloadEndpoint) {
	for _, historyAllocation := range wep.Status.History {
		for _, historyIPAllocationDetail := range historyAllocation.IPs {
			if historyIPAllocationDetail.IPv4 != nil && historyIPAllocationDetail.IPv4Pool != nil {
				tracePodSignal <- tracePodIdentify{
					PodCache: podCache,
					PodIP:    *historyIPAllocationDetail.IPv4,
					PoolName: *historyIPAllocationDetail.IPv4Pool,
				}
			}

			if historyIPAllocationDetail.IPv6 != nil && historyIPAllocationDetail.IPv6Pool != nil {
				tracePodSignal <- tracePodIdentify{
					PodCache: podCache,
					PodIP:    *historyIPAllocationDetail.IPv6,
					PoolName: *historyIPAllocationDetail.IPv6Pool,
				}
			}
		}
	}
}
