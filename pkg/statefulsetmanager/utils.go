// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager

import (
	"regexp"
	"strconv"
)

// statefulPodRegex is a regular expression that extracts the parent StatefulSet and ordinal from the Name of a Pod
var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// getStatefulSetNameAndOrdinal gets the name of pod's parent StatefulSet and pod's ordinal as extracted from its Name. If
// the Pod was not created by a StatefulSet, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func getStatefulSetNameAndOrdinal(podName string) (parent string, ordinal int, found bool) {
	parent = ""
	ordinal = -1

	subMatches := statefulPodRegex.FindStringSubmatch(podName)
	if len(subMatches) < 3 {
		return parent, ordinal, false
	}

	parent = subMatches[1]
	i, err := strconv.ParseInt(subMatches[2], 10, 32)
	if err != nil {
		return parent, ordinal, false
	}

	ordinal = int(i)
	return parent, ordinal, true
}
