// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func SetNamespaceIppoolAnnotation(IppoolAnnoValue []string, nsObject *corev1.Namespace, PoolNameList []string, keyAnno string) {
	b, err := json.Marshal(append(IppoolAnnoValue, PoolNameList...))
	Expect(err).NotTo(HaveOccurred())
	NamespaceIppoolAnnoStr := string(b)
	nsObject.Annotations[keyAnno] = NamespaceIppoolAnnoStr
	GinkgoWriter.Printf("Generate namespace objects: %v with namespace annotations \n", nsObject.Annotations)
}
