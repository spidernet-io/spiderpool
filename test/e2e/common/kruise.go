// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"errors"
	"fmt"

	kruisev1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	frame "github.com/spidernet-io/e2eframework/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GenerateExampleKruiseCloneSetYaml(name, namespace string, replica int32) *kruisev1.CloneSet {
	return &kruisev1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: kruisev1.CloneSetSpec{
			Replicas: pointer.Int32Ptr(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "sleep infinity"},
						},
					},
				},
			},
		},
	}
}

func GenerateExampleKruiseStatefulSetYaml(name, namespace string, replica int32) *kruisev1.StatefulSet {

	return &kruisev1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: kruisev1.StatefulSetSpec{
			Replicas: pointer.Int32Ptr(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "sleep infinity"},
						},
					},
				},
			},
		},
	}
}

func CreateKruiseCloneSet(f *frame.Framework, kruiseCloneSet *kruisev1.CloneSet, opts ...client.CreateOption) error {
	if f == nil || kruiseCloneSet == nil {
		return errors.New("wrong input")
	}

	fake := &kruisev1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: kruiseCloneSet.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &kruisev1.CloneSet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create, a same kruise cloneset %v/%v exists", kruiseCloneSet.ObjectMeta.Namespace, kruiseCloneSet.ObjectMeta.Name)
	}
	return f.CreateResource(kruiseCloneSet, opts...)
}

func DeleteKruiseCloneSetByName(f *frame.Framework, name, namespace string, opts ...client.DeleteOption) error {
	if name == "" || namespace == "" || f == nil {
		return errors.New("wrong input")
	}
	cloneSet := &kruisev1.CloneSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return f.DeleteResource(cloneSet, opts...)
}

func CreateKruiseStatefulSet(f *frame.Framework, kruiseStatefulSet *kruisev1.StatefulSet, opts ...client.CreateOption) error {
	if f == nil || kruiseStatefulSet == nil {
		return errors.New("wrong input")
	}

	fake := &kruisev1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: kruiseStatefulSet.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &kruisev1.StatefulSet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create, a same kruise statefulSet %v/%v exists", kruiseStatefulSet.ObjectMeta.Namespace, kruiseStatefulSet.ObjectMeta.Name)
	}
	return f.CreateResource(kruiseStatefulSet, opts...)
}

func DeleteKruiseStatefulSetByName(f *frame.Framework, name, namespace string, opts ...client.DeleteOption) error {
	if name == "" || namespace == "" || f == nil {
		return errors.New("wrong input")
	}
	statefulSet := &kruisev1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return f.DeleteResource(statefulSet, opts...)
}
