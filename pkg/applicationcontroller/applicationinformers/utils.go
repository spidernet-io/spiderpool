// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// ClusterSubnetDefaultFlexibleIPNumber is a singleton recording cluster default Subnet flexible IP number.
var ClusterSubnetDefaultFlexibleIPNumber = new(int)

var errInvalidInput = func(str string) error {
	return fmt.Errorf("invalid input '%s'", str)
}

const (
	maxNameLength = k8svalidation.DNS1123SubdomainMaxLength
	randomLength  = 5
)

func AutoPoolName(controllerName string, ipVersion types.IPVersion, ifName string, appUID apitypes.UID) string {
	// the format of uuid is "xxxx-xxxx-xxxx-xxxxxxxxxxxx"
	// ref: https://github.com/google/uuid/blob/44b5fee7c49cf3bcdf723f106b36d56ef13ccc88/uuid.go#L185
	splits := strings.Split(string(appUID), "-")
	lastOne := splits[len(splits)-1]
	// we constrain the max random length to 5
	if len(lastOne) > randomLength {
		lastOne = lastOne[:randomLength]
	}

	// 7 means "auto${IPVersion}" prefix and 2 bound symbol "-" length
	maxCustomNameLength := maxNameLength - len(lastOne) - 7
	if len(controllerName+ifName) > maxCustomNameLength {
		controllerName = controllerName[:maxCustomNameLength-len(ifName)]
	}

	return fmt.Sprintf("auto%d-%s-%s-%s", ipVersion, strings.ToLower(controllerName), strings.ToLower(ifName), strings.ToLower(lastOne))
}

// ApplicationLabelGV switches the kubernetes APIVersion from "/" link format to "_" link format for kubernetes label value usage.
func ApplicationLabelGV(apiVersion string) string {
	// Kubernetes API Group might be empty, ref: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#api-versions
	first, second, hasGroup := strings.Cut(apiVersion, "/")
	if hasGroup {
		return fmt.Sprintf("%s_%s", first, second)
	}

	return first
}

// ParseApplicationGVStr will parse a label value string back to apiVersion format string, its corresponding function is ApplicationLabelGV.
func ParseApplicationGVStr(str string) (apiVersion string, isMatch bool) {
	split := strings.Split(str, "_")

	// no API Group
	if len(split) == 1 {
		return schema.GroupVersion{
			Group:   "",
			Version: split[0],
		}.String(), true
	} else if len(split) == 2 {
		return schema.GroupVersion{
			Group:   split[0],
			Version: split[1],
		}.String(), true
	}

	return
}

// ApplicationNamespacedName will joint the application apiVersion, application type, namespace and name as a string, then we need unpack it for tracing
// [ns and object name constraint Ref]: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
// We set format is "{apiVersion}:{appKind}:{appNS}:{appName}"
func ApplicationNamespacedName(appNamespacedName types.AppNamespacedName) string {
	// Kubernetes API Group might be empty, ref: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#api-versions
	first, second, hasGroup := strings.Cut(appNamespacedName.APIVersion, "/")
	if hasGroup {
		return fmt.Sprintf("%s_%s_%s_%s_%s", first, second, appNamespacedName.Kind, appNamespacedName.Namespace, appNamespacedName.Name)
	}

	return fmt.Sprintf("%s_%s_%s_%s", first, appNamespacedName.Kind, appNamespacedName.Namespace, appNamespacedName.Name)
}

// ParseApplicationNamespacedName will unpack the appNamespacedNameKey, its corresponding function is ApplicationNamespacedName
func ParseApplicationNamespacedName(appNamespacedNameKey string) (appNamespacedName types.AppNamespacedName, isMatch bool) {
	split := strings.Split(appNamespacedNameKey, "_")

	// no API Group
	if len(split) == 4 {
		return types.AppNamespacedName{
			APIVersion: schema.GroupVersion{
				Group:   "",
				Version: split[0],
			}.String(),
			Kind:      split[1],
			Namespace: split[2],
			Name:      split[3],
		}, true
	} else if len(split) == 5 {
		return types.AppNamespacedName{
			APIVersion: schema.GroupVersion{
				Group:   split[0],
				Version: split[1],
			}.String(),
			Kind:      split[2],
			Namespace: split[3],
			Name:      split[4],
		}, true
	}

	return
}

func GetAppReplicas(replicas *int32) int {
	if replicas == nil {
		return 0
	}

	return int(*replicas)
}

func GenSubnetFreeIPs(subnet *spiderpoolv2beta1.SpiderSubnet) ([]net.IP, error) {
	var used []string

	if subnet.Status.ControlledIPPools != nil {
		var controlledIPPools spiderpoolv2beta1.PoolIPPreAllocations
		err := json.Unmarshal([]byte(*subnet.Status.ControlledIPPools), &controlledIPPools)
		if nil != err {
			return nil, err
		}
		for _, pool := range controlledIPPools {
			used = append(used, pool.IPs...)
		}
	}

	usedIPs, err := spiderpoolip.ParseIPRanges(*subnet.Spec.IPVersion, used)
	if err != nil {
		return nil, err
	}

	totalIPs, err := spiderpoolip.AssembleTotalIPs(*subnet.Spec.IPVersion, subnet.Spec.IPs, subnet.Spec.ExcludeIPs)
	if err != nil {
		return nil, err
	}
	freeIPs := spiderpoolip.IPsDiffSet(totalIPs, usedIPs, true)

	return freeIPs, nil
}

// GetSubnetAnnoConfig generates SpiderSubnet configuration from pod annotation,
// if the pod doesn't have the related subnet annotation but has IPPools/IPPool relative annotation it will return nil.
// If the pod doesn't have any subnet/ippool annotations, it will use the cluster default subnet configuration.
func GetSubnetAnnoConfig(podAnnotations map[string]string, log *zap.Logger) (*types.PodSubnetAnnoConfig, error) {
	var subnetAnnoConfig types.PodSubnetAnnoConfig

	// annotation: ipam.spidernet.io/subnets
	subnets, ok := podAnnotations[constant.AnnoSpiderSubnets]
	if ok {
		log.Sugar().Debugf("found SpiderSubnet feature annotation '%s' value '%s'", constant.AnnoSpiderSubnets, subnets)
		err := json.Unmarshal([]byte(subnets), &subnetAnnoConfig.MultipleSubnets)
		if nil != err {
			return nil, fmt.Errorf("failed to parse anntation '%s' value '%s', error: %v", constant.AnnoSpiderSubnets, subnets, err)
		}
	} else {
		// annotation: ipam.spidernet.io/subnet
		subnet, ok := podAnnotations[constant.AnnoSpiderSubnet]
		if ok {
			log.Sugar().Debugf("found SpiderSubnet feature annotation '%s' value '%s'", constant.AnnoSpiderSubnet, subnet)
			subnetAnnoConfig.SingleSubnet = new(types.AnnoSubnetItem)
			err := json.Unmarshal([]byte(subnet), &subnetAnnoConfig.SingleSubnet)
			if nil != err {
				return nil, fmt.Errorf("failed to parse anntation '%s' value '%s', error: %v", constant.AnnoSpiderSubnet, subnet, err)
			}
		} else {
			log.Debug("no SpiderSubnet feature annotation found, use default IPAM mode")
			return nil, nil
		}
	}

	var isFlexible bool
	var ipNum int
	var err error

	// annotation: ipam.spidernet.io/ippool-ip-number
	poolIPNum, ok := podAnnotations[constant.AnnoSpiderSubnetPoolIPNumber]
	if ok {
		log.Sugar().Debugf("use IPPool IP number '%s'", poolIPNum)
		isFlexible, ipNum, err = GetPoolIPNumber(poolIPNum)
		if nil != err {
			return nil, err
		}

		// check out negative number
		if ipNum < 0 {
			return nil, fmt.Errorf("subnet '%s' value must equal or greater than 0", constant.AnnoSpiderSubnetPoolIPNumber)
		}

		if isFlexible {
			subnetAnnoConfig.FlexibleIPNum = ptr.To(ipNum)
		} else {
			subnetAnnoConfig.AssignIPNum = ipNum
		}
	} else {
		log.Sugar().Debugf("no specified IPPool IP number, default to use cluster default subnet flexible IP number: %d", *ClusterSubnetDefaultFlexibleIPNumber)
		subnetAnnoConfig.FlexibleIPNum = ptr.To(*ClusterSubnetDefaultFlexibleIPNumber)
	}

	// annotation: "ipam.spidernet.io/reclaim-ippool", reclaim IPPool or not (default true)
	reclaimPool, err := ShouldReclaimIPPool(podAnnotations)
	if nil != err {
		return nil, err
	}
	subnetAnnoConfig.ReclaimIPPool = reclaimPool

	err = mutateAndValidateSubnetAnno(&subnetAnnoConfig)
	if nil != err {
		return nil, err
	}

	return &subnetAnnoConfig, nil
}

// mutateAndValidateSubnetAnno will filter multiple subnets you specified and only leaves you the first one to use.
// And it also checks Interface name or subnets you specified whether are duplicate.
func mutateAndValidateSubnetAnno(subnetConfig *types.PodSubnetAnnoConfig) error {
	// the present version, we just only support one SpiderSubnet object to choose
	if len(subnetConfig.MultipleSubnets) != 0 {
		var v4SubnetsArray, v6SubnetsArray []string
		var ifNameArray []string

		for index := range subnetConfig.MultipleSubnets {
			ifNameArray = append(ifNameArray, subnetConfig.MultipleSubnets[index].Interface)

			if len(subnetConfig.MultipleSubnets[index].IPv4) != 0 {
				subnetConfig.MultipleSubnets[index].IPv4 = []string{subnetConfig.MultipleSubnets[index].IPv4[0]}
				if subnetConfig.MultipleSubnets[index].IPv4[0] == "" {
					return fmt.Errorf("it's invalid to set an empty IPv4 subnet with mutilple interfaces")
				}
				v4SubnetsArray = append(v4SubnetsArray, subnetConfig.MultipleSubnets[index].IPv4[0])
			}
			if len(subnetConfig.MultipleSubnets[index].IPv6) != 0 {
				subnetConfig.MultipleSubnets[index].IPv6 = []string{subnetConfig.MultipleSubnets[index].IPv6[0]}
				if subnetConfig.MultipleSubnets[index].IPv6[0] == "" {
					return fmt.Errorf("it's invalid to set an empty IPv6 subnet with mutilple interfaces")
				}
				v6SubnetsArray = append(v6SubnetsArray, subnetConfig.MultipleSubnets[index].IPv6[0])
			}

			// all none
			if len(subnetConfig.MultipleSubnets[index].IPv4) == 0 && len(subnetConfig.MultipleSubnets[index].IPv6) == 0 {
				return fmt.Errorf("it's invalid to set dual empty subnet with multiple interfaces: %v", subnetConfig)
			}
		}

		// validate duplicate subnet
		if containsDuplicate(v4SubnetsArray) || containsDuplicate(v6SubnetsArray) {
			return fmt.Errorf("it's invalid to use the same subnet for multiple interfaces: %v", subnetConfig)
		}

		// validate duplicate interface
		if containsDuplicate(ifNameArray) {
			return fmt.Errorf("it's invalid to use the same Interface name for multiple interfaces: %v", subnetConfig)
		}
	} else if subnetConfig.SingleSubnet != nil {
		if len(subnetConfig.SingleSubnet.IPv4) != 0 {
			subnetConfig.SingleSubnet.IPv4 = []string{subnetConfig.SingleSubnet.IPv4[0]}
			if subnetConfig.SingleSubnet.IPv4[0] == "" {
				return fmt.Errorf("it's invalid to set an empty IPv4 subnet with single interface: %v", subnetConfig)
			}
		}
		if len(subnetConfig.SingleSubnet.IPv6) != 0 {
			subnetConfig.SingleSubnet.IPv6 = []string{subnetConfig.SingleSubnet.IPv6[0]}
			if subnetConfig.SingleSubnet.IPv6[0] == "" {
				return fmt.Errorf("it's invalid to set an empty IPv6 subnet with single interface: %v", subnetConfig)
			}
		}

		// all none
		if len(subnetConfig.SingleSubnet.IPv4) == 0 && len(subnetConfig.SingleSubnet.IPv6) == 0 {
			return fmt.Errorf("it's invalid to set dual empty subnet with single interface: %v", subnetConfig)
		}
		// specify 'eth0' as the default single interface if it's none.
		if subnetConfig.SingleSubnet.Interface == "" {
			subnetConfig.SingleSubnet.Interface = constant.ClusterDefaultInterfaceName
		}
	} else {
		return fmt.Errorf("no subnets specified: %v", subnetConfig)
	}

	return nil
}

// GetPoolIPNumber judges the given parameter is fixed or flexible
func GetPoolIPNumber(str string) (isFlexible bool, ipNum int, err error) {
	tmp := str

	// the '+' sign counts must be '0' or '1'
	plusSignNum := strings.Count(str, "+")
	if plusSignNum == 0 || plusSignNum == 1 {
		_, after, found := strings.Cut(str, "+")
		if found {
			tmp = after
		}

		ipNum, err = strconv.Atoi(tmp)
		if nil != err {
			return false, -1, fmt.Errorf("%w: %v", errInvalidInput(str), err)
		}

		return found, ipNum, nil
	}

	return false, -1, errInvalidInput(str)
}

// CalculateJobPodNum will calculate the job replicas
// once Parallelism and Completions are unset, the API-server will set them to 1
// reference: https://kubernetes.io/docs/concepts/workloads/controllers/job/
func CalculateJobPodNum(jobSpecParallelism, jobSpecCompletions *int32) int {
	switch {
	case jobSpecParallelism != nil && jobSpecCompletions == nil:
		// parallel Jobs with a work queue
		if *jobSpecParallelism == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecParallelism)

	case jobSpecParallelism == nil && jobSpecCompletions != nil:
		// non-parallel Jobs
		if *jobSpecCompletions == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecCompletions)

	case jobSpecParallelism != nil && jobSpecCompletions != nil:
		// parallel Jobs with a fixed completion count
		if *jobSpecCompletions == 0 {
			return 1
		}

		// ignore negative integer, cause API-server will refuse the job creation
		return int(*jobSpecCompletions)
	}

	return 1
}

// IsDefaultIPPoolMode judges whether we use subnet feature or not with the given parameter types.PodSubnetAnnoConfig
func IsDefaultIPPoolMode(subnetConfig *types.PodSubnetAnnoConfig) bool {
	if subnetConfig == nil {
		return true
	}

	// SpiderSubnet with multiple interfaces
	if len(subnetConfig.MultipleSubnets) != 0 {
		return false
	}

	// SpiderSubnet with single interface
	if subnetConfig.SingleSubnet != nil {
		return false
	}

	return false
}

// containsDuplicate checks whether the given string array has the duplicate element
func containsDuplicate(arr []string) bool {
	sort.Strings(arr)
	for i := 1; i < len(arr); i++ {
		if arr[i] == arr[i-1] {
			return true
		}
	}
	return false
}

// ShouldReclaimIPPool will check pod annotation "ipam.spidernet.io/ippool-reclaim"
func ShouldReclaimIPPool(anno map[string]string) (bool, error) {
	reclaimPool, ok := anno[constant.AnnoSpiderSubnetReclaimIPPool]
	if ok {
		parseBool, err := strconv.ParseBool(reclaimPool)
		if nil != err {
			return false, fmt.Errorf("failed to parse spider subnet '%s', error: %v", constant.AnnoSpiderSubnetReclaimIPPool, err)
		}
		return parseBool, nil
	}

	// no specified reclaim-IPPool, default to set it true
	return true, nil
}

// IsAppExist will check the application whether exists or not. If it exists, it will return the application corresponding UID
func IsAppExist(ctx context.Context, cacheClient client.Client, dynamicClient dynamic.Interface, appNamespacedName types.AppNamespacedName) (isExist bool, appUID apitypes.UID, err error) {
	var object client.Object
	isThird := false

	if slices.Contains(constant.K8sAPIVersions, appNamespacedName.APIVersion) {
		switch appNamespacedName.Kind {
		case constant.KindPod:
			object = &corev1.Pod{}
		case constant.KindDeployment:
			object = &appsv1.Deployment{}
		case constant.KindReplicaSet:
			object = &appsv1.ReplicaSet{}
		case constant.KindDaemonSet:
			object = &appsv1.DaemonSet{}
		case constant.KindStatefulSet:
			object = &appsv1.StatefulSet{}
		case constant.KindJob:
			object = &batchv1.Job{}
		case constant.KindCronJob:
			object = &batchv1.CronJob{}
		default:
			isThird = true
		}
	} else {
		isThird = true
	}

	var unstructuredObject *unstructured.Unstructured
	if isThird {
		gvr, e := GenerateGVR(appNamespacedName)
		if nil != e {
			return false, "", e
		}
		unstructuredObject, err = dynamicClient.Resource(gvr).Namespace(appNamespacedName.Namespace).Get(ctx, appNamespacedName.Name, metav1.GetOptions{})
	} else {
		err = cacheClient.Get(ctx, apitypes.NamespacedName{Namespace: appNamespacedName.Namespace, Name: appNamespacedName.Name}, object)
	}

	if nil != err {
		// if the application is no longer exist, we should delete the IPPool
		if apierrors.IsNotFound(err) {
			return false, "", nil
		}

		return false, "", err
	}

	if unstructuredObject != nil {
		appUID = unstructuredObject.GetUID()
	} else {
		appUID = object.GetUID()
	}

	return true, appUID, nil
}

func GenerateGVR(appNamespacedName types.AppNamespacedName) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(appNamespacedName.APIVersion)
	if nil != err {
		return schema.GroupVersionResource{}, err
	}

	gvk := gv.WithKind(appNamespacedName.Kind)
	gvrPlural, _ := meta.UnsafeGuessKindToResource(gvk)

	return gvrPlural, nil
}

func IsThirdController(appNamespacedName types.AppNamespacedName) bool {
	isThird := false
	if slices.Contains(constant.K8sAPIVersions, appNamespacedName.APIVersion) {
		if !slices.Contains(constant.K8sKinds, appNamespacedName.Kind) {
			isThird = true
		}
	} else {
		isThird = true
	}

	return isThird
}

func IsReclaimAutoPoolLabelValue(isReclaim bool) string {
	if isReclaim {
		return constant.True
	}

	return constant.False
}

func AutoPoolIPVersionLabelValue(ipVersion types.IPVersion) string {
	if ipVersion == constant.IPv4 {
		return constant.LabelValueIPVersionV4
	}

	return constant.LabelValueIPVersionV6
}
