package dra

import (
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
)

func PodNameIndexFunc(obj interface{}) ([]string, error) {
	claim, ok := obj.(*resourcev1beta1.ResourceClaim)
	if !ok {
		return []string{}, nil
	}

	result := []string{}
	for _, reserved := range claim.Status.ReservedFor {
		if reserved.Resource != "pods" || reserved.APIGroup != "" {
			continue
		}
		result = append(result, string(reserved.Name))
	}
	return result, nil
}
