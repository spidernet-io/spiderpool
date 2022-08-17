// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

func SetNsAnnotation(nsObject *corev1.Namespace, PoolNameList []string, keyAnno string) {
	vIppoolAnnoValue := types.AnnoNSDefautlV4PoolValue{}
	b, err := json.Marshal(append(vIppoolAnnoValue, PoolNameList...))
	Expect(err).NotTo(HaveOccurred())
	vNamespaceIppoolAnnoStr := string(b)
	nsObject.Annotations[keyAnno] = vNamespaceIppoolAnnoStr
	GinkgoWriter.Printf("Generate namespace objects: %v with namespace annotations \n", nsObject.Annotations)
}
