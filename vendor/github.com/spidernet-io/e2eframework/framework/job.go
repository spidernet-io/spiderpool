// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"time"

	"github.com/spidernet-io/e2eframework/tools"

	//appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateJob(jb *batchv1.Job, opts ...client.CreateOption) error {

	// try to wait for finish last deleting
	fake := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: jb.ObjectMeta.Namespace,
			Name:      jb.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &batchv1.Job{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return ErrAlreadyExisted
	}
	t := func() bool {
		existing := &batchv1.Job{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.Log("waiting for a same Job %v/%v to finish deleting \n", jb.ObjectMeta.Name, jb.ObjectMeta.Namespace)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return ErrTimeOut
	}

	return f.CreateResource(jb, opts...)
}

func (f *Framework) DeleteJob(name, namespace string, opts ...client.DeleteOption) error {
	if name == "" || namespace == "" {
		return ErrWrongInput

	}

	jb := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(jb, opts...)
}

func (f *Framework) GetJob(name, namespace string) (*batchv1.Job, error) {
	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	jb := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(jb)
	existing := &batchv1.Job{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) GetJobPodList(jb *batchv1.Job) (*corev1.PodList, error) {
	if jb == nil {
		return nil, ErrWrongInput
	}
	pods := &corev1.PodList{}
	ops := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(jb.Spec.Selector.MatchLabels),
		},
	}
	e := f.ListResource(pods, ops...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

// WaitJobFinished wait for all job pod finish , no matter succceed or fail
func (f *Framework) WaitJobFinished(jobName, namespace string, ctx context.Context) (*batchv1.Job, bool, error) {
	for {
		select {
		default:
			job, err := f.GetJob(jobName, namespace)
			if err != nil {
				return nil, false, err
			}
			for _, c := range job.Status.Conditions {
				if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
					return job, false, nil
				}
				if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
					return job, true, nil
				}
			}

			time.Sleep(time.Second)
		case <-ctx.Done():
			return nil, false, ErrTimeOut

		}
	}
}
