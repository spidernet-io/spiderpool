// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/spiderpool/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

func SetNamespaceIppoolAnnotation(IppoolAnnoValue []string, nsObject *corev1.Namespace, PoolNameList []string, keyAnno string) {
	b, err := json.Marshal(append(IppoolAnnoValue, PoolNameList...))
	Expect(err).NotTo(HaveOccurred())
	NamespaceIppoolAnnoStr := string(b)
	nsObject.Annotations[keyAnno] = NamespaceIppoolAnnoStr
	GinkgoWriter.Printf("Generate namespace objects: %v with namespace annotations \n", nsObject.Annotations)
}

func GeneratePodIPPoolAnnotations(frame *e2e.Framework, nic string, v4PoolNameList, v6PoolNameList []string) string {
	podAnno := types.AnnoPodIPPoolValue{
		NIC: &nic,
	}
	podAnno.IPv4Pools = v4PoolNameList
	podAnno.IPv6Pools = v6PoolNameList

	b, err := json.Marshal(podAnno)
	Expect(err).NotTo(HaveOccurred())
	podAnnoStr := string(b)
	return podAnnoStr
}

func GeneratePodIPPoolsAnnotations(frame *e2e.Framework, nic string, cleanGateway bool, v4PoolNameList, v6PoolNameList []string) string {
	podIppoolsAnno := types.AnnoPodIPPoolsValue{
		types.AnnoIPPoolItem{
			NIC:          nic,
			CleanGateway: cleanGateway,
		},
	}
	podIppoolsAnno[0].IPv4Pools = v4PoolNameList
	podIppoolsAnno[0].IPv6Pools = v6PoolNameList

	b, err := json.Marshal(podIppoolsAnno)
	Expect(err).NotTo(HaveOccurred())
	podIppoolsAnnostr := string(b)
	return podIppoolsAnnostr
}
