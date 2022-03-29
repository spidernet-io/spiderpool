// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

const (
	AnnotationPre              = "ipam.spidernet.io"
	AnnoPodIppool              = AnnotationPre + "/ippool"
	AnnoPodIppools             = AnnotationPre + "/ippools"
	AnnoPodRoute               = AnnotationPre + "/routes"
	AnnoPodDns                 = AnnotationPre + "/dns"
	AnnoPodStatus              = AnnotationPre + "/status"
	AnnoNamespaceDefautlV4Pool = AnnotationPre + "/defaultv4ippool"
	AnnoNamespaceDefautlV6Pool = AnnotationPre + "/defaultv6ippool"
)
