// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/containernetworking/cni/pkg/types"
	netv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	coordinatorcmd "github.com/spidernet-io/spiderpool/cmd/coordinator/cmd"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	spiderpoolcmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/election"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	informers "github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions/spiderpool.spidernet.io/v2beta1"
	listers "github.com/spidernet-io/spiderpool/pkg/k8s/client/listers/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var informerLogger *zap.Logger

type MultusConfigController struct {
	MultusConfigControllerConfig
	client                client.Client
	multusConfigLister    listers.SpiderMultusConfigLister
	multusConfigSynced    cache.InformerSynced
	multusConfigWorkqueue workqueue.RateLimitingInterface
}

type MultusConfigControllerConfig struct {
	ControllerWorkers             int
	WorkQueueMaxRetries           int
	WorkQueueRequeueDelayDuration time.Duration
	LeaderRetryElectGap           time.Duration
	ResyncPeriod                  time.Duration
}

func NewMultusConfigController(multusConfigControllerConfig MultusConfigControllerConfig, client client.Client) *MultusConfigController {
	informerLogger = logutils.Logger.Named("MultusConfig-Informer")

	m := &MultusConfigController{
		MultusConfigControllerConfig: multusConfigControllerConfig,
		client:                       client,
	}

	return m
}

func (mcc *MultusConfigController) SetupInformer(ctx context.Context, client crdclientset.Interface, leader election.SpiderLeaseElector) error {
	if leader == nil {
		return fmt.Errorf("controller leader %w", constant.ErrMissingRequiredParam)
	}

	informerLogger.Info("try to register MultusConfig informer")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if !leader.IsElected() {
				time.Sleep(mcc.LeaderRetryElectGap)
				continue
			}

			innerCtx, innerCancel := context.WithCancel(ctx)
			go func() {
				for {
					select {
					case <-innerCtx.Done():
						return
					default:
					}

					if !leader.IsElected() {
						informerLogger.Warn("Leader lost, stop MultusConfig informer")
						innerCancel()
						return
					}
					time.Sleep(mcc.LeaderRetryElectGap)
				}
			}()

			informerLogger.Info("create MultusConfig informer")
			factory := externalversions.NewSharedInformerFactory(client, mcc.ResyncPeriod)
			err := mcc.addEventHandlers(factory.Spiderpool().V2beta1().SpiderMultusConfigs())
			if nil != err {
				informerLogger.Error(err.Error())
				continue
			}
			factory.Start(innerCtx.Done())

			if err := mcc.Run(innerCtx.Done()); nil != err {
				informerLogger.Sugar().Errorf("failed to run MultusConfig controller, error: %v", err)
			}
			informerLogger.Error("SpiderMultusConfig informer broken")
		}
	}()

	return nil
}

func (mcc *MultusConfigController) addEventHandlers(multusConfigInformer informers.SpiderMultusConfigInformer) error {
	mcc.multusConfigLister = multusConfigInformer.Lister()
	mcc.multusConfigSynced = multusConfigInformer.Informer().HasSynced

	mcc.multusConfigWorkqueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SpiderMultusConfigs")

	_, err := multusConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: mcc.enqueueMultusConfig,
		UpdateFunc: func(oldObj, newObj interface{}) {
			mcc.enqueueMultusConfig(newObj)
		},
		DeleteFunc: nil,
	})
	if nil != err {
		return err
	}

	return nil
}

func (mcc *MultusConfigController) enqueueMultusConfig(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if nil != err {
		informerLogger.Sugar().Errorf("failed to parse object %+v meta key", obj)
		return
	}

	mcc.multusConfigWorkqueue.Add(key)
	informerLogger.Sugar().Debugf("added %s to MultusConfig workqueue", key)
}

func (mcc *MultusConfigController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer mcc.multusConfigWorkqueue.ShutDown()

	informerLogger.Debug("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, mcc.multusConfigSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < mcc.ControllerWorkers; i++ {
		informerLogger.Sugar().Debugf("Starting MultusConfig processing worker %d", i)
		go wait.Until(mcc.runWorker, 1*time.Second, stopCh)
	}

	<-stopCh
	informerLogger.Error("Shutting down MultusConfig controller workers")
	return nil
}

func (mcc *MultusConfigController) runWorker() {
	for mcc.processNextWorkItem() {
	}
}

func (mcc *MultusConfigController) processNextWorkItem() bool {
	obj, shutdown := mcc.multusConfigWorkqueue.Get()
	if shutdown {
		informerLogger.Error("MultusConfig workqueue is already shutdown!")
		return false
	}

	process := func(obj interface{}) error {
		defer mcc.multusConfigWorkqueue.Done(obj)
		key, ok := obj.(string)
		if !ok {
			mcc.multusConfigWorkqueue.Forget(obj)
			informerLogger.Sugar().Errorf("expected string in workQueue but got %+v", obj)
			return nil
		}

		ns, name, err := cache.SplitMetaNamespaceKey(key)
		if nil != err {
			mcc.multusConfigWorkqueue.Forget(obj)
			informerLogger.Sugar().Errorf("failed to split meta namespace key %s, error: %v", key, err)
			return nil
		}

		multusConfig, err := mcc.multusConfigLister.SpiderMultusConfigs(ns).Get(name)
		if nil != err {
			if apierrors.IsNotFound(err) {
				mcc.multusConfigWorkqueue.Forget(obj)
				informerLogger.Sugar().Debugf("MultusConfig %s in workqueue no longer exists", key)
				return nil
			}

			mcc.multusConfigWorkqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing %s: %s, requeing", key, err.Error())
		}

		err = mcc.syncHandler(context.TODO(), multusConfig.DeepCopy())
		if nil != err {
			// discard some wrong input items
			if errors.Is(err, constant.ErrWrongInput) {
				mcc.multusConfigWorkqueue.Forget(key)
				return fmt.Errorf("failed to process MultusConfig '%s', error:%v, discarding it", key, err)
			}

			if apierrors.IsConflict(err) {
				mcc.multusConfigWorkqueue.AddRateLimited(key)
				informerLogger.Sugar().Warnf("encountered MultusConfig informer update conflict '%v', retrying...", err)
				return nil
			}

			if mcc.WorkQueueRequeueDelayDuration >= 0 {
				if mcc.multusConfigWorkqueue.NumRequeues(key) < mcc.WorkQueueMaxRetries {
					informerLogger.Sugar().Errorf("encountered  MultusConfig informer error '%v', requeue it after '%v'", err, mcc.WorkQueueRequeueDelayDuration)
					mcc.multusConfigWorkqueue.AddAfter(key, mcc.WorkQueueRequeueDelayDuration)
					return nil
				}

				informerLogger.Sugar().Warnf("out of workqueue max retries, drop MultusConfig '%s'", key)
			}

			mcc.multusConfigWorkqueue.Forget(key)
			return fmt.Errorf("error syncing '%s': %s, discarding it", key, err.Error())
		}

		mcc.multusConfigWorkqueue.Forget(obj)
		return nil
	}

	err := process(obj)
	if nil != err {
		informerLogger.Error(err.Error())
	}

	return true
}

func (mcc *MultusConfigController) syncHandler(ctx context.Context, multusConfig *spiderpoolv2beta1.SpiderMultusConfig) error {
	if multusConfig.DeletionTimestamp != nil {
		informerLogger.Sugar().Debugf("MultusConfig %s/%s is terminating, no need to sync", multusConfig.Namespace, multusConfig.Name)
		return nil
	}

	isExist := true

	// use the annotation specified name as the CNI configuration name if set
	netAttachName := multusConfig.Name
	if tmpName, ok := multusConfig.Annotations[constant.AnnoNetAttachConfName]; ok {
		netAttachName = tmpName
	}

	netAttachDef := &netv1.NetworkAttachmentDefinition{}
	err := mcc.client.Get(ctx, ktypes.NamespacedName{
		Namespace: multusConfig.Namespace,
		Name:      netAttachName,
	}, netAttachDef)
	if nil != err {
		if apierrors.IsNotFound(err) {
			isExist = false
		} else {
			return err
		}
	}

	newNetAttachDef, err := generateNetAttachDef(netAttachName, multusConfig)
	if nil != err {
		return fmt.Errorf("failed to generate net-attach-def, error: %w", err)
	}

	err = controllerutil.SetControllerReference(multusConfig, newNetAttachDef, mcc.client.Scheme())
	if nil != err {
		return fmt.Errorf("failed to set net-attach-def %s owner reference with MultusConfig %s/%s, error: %w",
			newNetAttachDef.Name, multusConfig.Namespace, multusConfig.Name, err)
	}

	if isExist {
		// we need to wait and let the kubernetes delete this Net-Attach-Def first.
		if netAttachDef.DeletionTimestamp != nil {
			return fmt.Errorf("the old net-attach-def %s/%s is terminating, wait for a while", netAttachDef.Namespace, netAttachDef.Name)
		}

		isNeedUpdate := false

		// the annotations updated
		if !reflect.DeepEqual(netAttachDef.Annotations, newNetAttachDef.Annotations) {
			informerLogger.Sugar().Debugf("MultusConfig %s/%s annotation changed, the old one is %v, and the new one is %v",
				multusConfig.Namespace, multusConfig.Name, netAttachDef.Annotations, newNetAttachDef.Annotations)
			netAttachDef.SetAnnotations(newNetAttachDef.Annotations)
			isNeedUpdate = true
		}

		// the MultusConfig CNI configuration changed
		if netAttachDef.Spec.Config != newNetAttachDef.Spec.Config {
			informerLogger.Sugar().Debugf("MultusConfig %s/%s CNI configuration changed, the old one is %v, and the new one is %v",
				multusConfig.Namespace, multusConfig.Name, netAttachDef.Spec.Config, newNetAttachDef.Spec.Config)
			netAttachDef.Spec.Config = newNetAttachDef.Spec.Config
			isNeedUpdate = true
		}

		// the net-attach-def ownerRef was removed
		if !metav1.IsControlledBy(netAttachDef, multusConfig) {
			informerLogger.Sugar().Debugf("net-attach-def ownerReference was removed, try to add it")
			netAttachDef.SetOwnerReferences(newNetAttachDef.GetOwnerReferences())
			isNeedUpdate = true
		}

		if isNeedUpdate {
			informerLogger.Sugar().Infof("try to update net-attach-def %v", netAttachDef)
			err := mcc.client.Update(ctx, netAttachDef)
			if nil != err {
				return fmt.Errorf("failed to update net-attach-def %v, error: %w", netAttachDef, err)
			}
		}

		return nil
	}

	informerLogger.Sugar().Infof("try to create net-attach-def %v for MultusConfg %s/%s", newNetAttachDef, multusConfig.Namespace, multusConfig.Name)
	err = mcc.client.Create(ctx, newNetAttachDef)
	if nil != err {
		return fmt.Errorf("failed to create net-attach-def %v, error: %w", newNetAttachDef, err)
	}
	return nil
}

func generateNetAttachDef(netAttachName string, multusConf *spiderpoolv2beta1.SpiderMultusConfig) (*netv1.NetworkAttachmentDefinition, error) {
	multusConfSpec := multusConf.Spec.DeepCopy()
	anno := multusConf.Annotations

	var plugins []interface{}

	// with Kubernetes OpenAPI validation, multusConfSpec.EnableCoordinator must not be nil
	hasCoordinator := *multusConfSpec.EnableCoordinator
	if hasCoordinator {
		coordinatorCNIConf := generateCoordinatorCNIConf(multusConfSpec.CoordinatorConfig)
		// head insertion later
		plugins = append(plugins, coordinatorCNIConf)
	}

	switch multusConfSpec.CniType {
	case MacVlanType:
		macvlanCNIConf := generateMacvlanCNIConf(*multusConfSpec)
		// head insertion
		plugins = append([]interface{}{macvlanCNIConf}, plugins...)
		if multusConfSpec.MacvlanConfig.VlanID != nil && *multusConfSpec.MacvlanConfig.VlanID != 0 {
			// we need to set Subvlan as first at the CNI plugin chain
			subVlanCNIConf := generateIfacer(multusConfSpec.MacvlanConfig.Master,
				*multusConfSpec.MacvlanConfig.VlanID,
				multusConfSpec.MacvlanConfig.Bond)
			plugins = append([]interface{}{subVlanCNIConf}, plugins...)
		}
	case IpVlanType:
		ipvlanCNIConf := generateIPvlanCNIConf(*multusConfSpec)
		// head insertion
		plugins = append([]interface{}{ipvlanCNIConf}, plugins...)
		if multusConfSpec.IPVlanConfig.VlanID != nil && *multusConfSpec.IPVlanConfig.VlanID != 0 {
			// we need to set Subvlan as first at the CNI plugin chain
			subVlanCNIConf := generateIfacer(multusConfSpec.IPVlanConfig.Master,
				*multusConfSpec.IPVlanConfig.VlanID,
				multusConfSpec.IPVlanConfig.Bond)
			plugins = append([]interface{}{subVlanCNIConf}, plugins...)
		}
	case SriovType:
		// SRIOV special annotation
		anno[resourceNameAnnot] = multusConfSpec.SriovConfig.ResourceName

		sriovCNIConf := generateSriovCNIConf(*multusConfSpec)
		// head insertion
		plugins = append([]interface{}{sriovCNIConf}, plugins...)
	case CustomType:

	default:
		// It's impossible get into the default branch
		return nil, fmt.Errorf("%w: unrecognized CNI type %s", constant.ErrWrongInput, multusConfSpec.CniType)
	}

	cniVersion, ok := anno[constant.AnnoMultusConfigCNIVersion]
	if !ok {
		// we'll use the default CNI version 0.3.1 if the annotation doesn't have it.
		cniVersion = cmd.CniVersion031
	} else {
		if !slices.Contains(cmd.SupportCNIVersions, cniVersion) {
			return nil, fmt.Errorf("%w: unsupported CNI version %s", constant.ErrWrongInput, cniVersion)
		}
	}

	fmt.Printf("Length: %d, Cap: %d\n", len(plugins), cap(plugins))

	var confStr string
	if multusConfSpec.CniType != CustomType {
		rawList := map[string]interface{}{
			"name":       netAttachName,
			"cniVersion": cniVersion,
			"plugins":    plugins,
		}
		bytes, err := json.Marshal(rawList)
		if nil != err {
			return nil, err
		}
		confStr = string(bytes)
	} else {
		if multusConfSpec.CustomCNIConfig != nil && len(*multusConfSpec.CustomCNIConfig) > 0 {
			confStr = *multusConfSpec.CustomCNIConfig
		}
	}

	netAttachDef := &netv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        netAttachName,
			Namespace:   multusConf.Namespace,
			Annotations: anno,
		},
	}
	if len(confStr) > 0 {
		netAttachDef.Spec = netv1.NetworkAttachmentDefinitionSpec{
			Config: confStr,
		}
	}

	return netAttachDef, nil
}

func generateMacvlanCNIConf(multusConfSpec spiderpoolv2beta1.MultusCNIConfigSpec) interface{} {
	var masterName string

	// choose interface basement name
	if len(multusConfSpec.MacvlanConfig.Master) == 1 {
		masterName = multusConfSpec.MacvlanConfig.Master[0]
	} else {
		masterName = multusConfSpec.MacvlanConfig.Bond.Name
	}

	// set vlanID for interface basement name
	if multusConfSpec.MacvlanConfig.VlanID != nil {
		if *multusConfSpec.MacvlanConfig.VlanID != 0 {
			masterName = fmt.Sprintf("%s.%d", masterName, *multusConfSpec.MacvlanConfig.VlanID)
		}
	}

	// TODO(Icarus9913): customize the macvlan mode
	netConf := MacvlanNetConf{
		Type: string(MacVlanType),
		IPAM: spiderpoolcmd.IPAMConfig{
			Type: constant.Spiderpool,
		},
		Master: masterName,
		Mode:   "bridge",
	}

	// set default IPPools for spiderpool cni configuration
	if multusConfSpec.MacvlanConfig.SpiderpoolConfigPools != nil {
		netConf.IPAM.DefaultIPv4IPPool = multusConfSpec.MacvlanConfig.SpiderpoolConfigPools.IPv4IPPool
		netConf.IPAM.DefaultIPv6IPPool = multusConfSpec.MacvlanConfig.SpiderpoolConfigPools.IPv6IPPool
	}

	return netConf
}

func generateIPvlanCNIConf(multusConfSpec spiderpoolv2beta1.MultusCNIConfigSpec) interface{} {
	var masterName string

	// choose interface basement name
	if len(multusConfSpec.IPVlanConfig.Master) == 1 {
		masterName = multusConfSpec.IPVlanConfig.Master[0]
	} else {
		masterName = multusConfSpec.IPVlanConfig.Bond.Name
	}

	if multusConfSpec.IPVlanConfig.VlanID != nil {
		if *multusConfSpec.IPVlanConfig.VlanID != 0 {
			masterName = fmt.Sprintf("%s.%d", masterName, *multusConfSpec.IPVlanConfig.VlanID)
		}
	}

	netConf := IPvlanNetConf{
		Type: string(IpVlanType),
		IPAM: spiderpoolcmd.IPAMConfig{
			Type: constant.Spiderpool,
		},
		Master: masterName,
	}

	// set default IPPools for spiderpool cni configuration
	if multusConfSpec.IPVlanConfig.SpiderpoolConfigPools != nil {
		netConf.IPAM.DefaultIPv4IPPool = multusConfSpec.IPVlanConfig.SpiderpoolConfigPools.IPv4IPPool
		netConf.IPAM.DefaultIPv6IPPool = multusConfSpec.IPVlanConfig.SpiderpoolConfigPools.IPv6IPPool
	}

	return netConf
}

func generateSriovCNIConf(multusConfSpec spiderpoolv2beta1.MultusCNIConfigSpec) interface{} {
	netConf := SRIOVNetConf{
		Type: string(SriovType),
		IPAM: spiderpoolcmd.IPAMConfig{
			Type: constant.Spiderpool,
		},
	}

	if multusConfSpec.SriovConfig.VlanID != nil {
		netConf.Vlan = pointer.Int(int(*multusConfSpec.SriovConfig.VlanID))
	}

	// set default IPPools for spiderpool cni configuration
	if multusConfSpec.SriovConfig.SpiderpoolConfigPools != nil {
		netConf.IPAM.DefaultIPv4IPPool = multusConfSpec.SriovConfig.SpiderpoolConfigPools.IPv4IPPool
		netConf.IPAM.DefaultIPv6IPPool = multusConfSpec.SriovConfig.SpiderpoolConfigPools.IPv6IPPool
	}

	return netConf
}

func generateIfacer(master []string, vlanID int32, bond *spiderpoolv2beta1.BondConfig) interface{} {
	netConf := IfacerNetConf{
		NetConf: types.NetConf{
			Type: ifacerBinName,
		},
		Interfaces: master,
		VlanID:     int(vlanID),
	}

	if bond != nil {
		netConf.Bond.Name = bond.Name
		netConf.Bond.Mode = int(bond.Mode)

		if bond.Options != nil {
			netConf.Bond.Options = *bond.Options
		}
	}

	return netConf
}

func generateCoordinatorCNIConf(coordinatorSpec *spiderpoolv2beta1.CoordinatorSpec) interface{} {
	coordinatorNetConf := coordinatorcmd.Config{
		NetConf: types.NetConf{
			Type: coordinatorBinName,
		},
	}

	// coordinatorSpec could be nil, and we just need the coorinator CNI specified and use the default configuration
	if coordinatorSpec != nil {
		if coordinatorSpec.TuneMode != nil {
			coordinatorNetConf.TuneMode = coordinatorcmd.Mode(*coordinatorSpec.TuneMode)
		}
		if len(coordinatorSpec.ExtraCIDR) != 0 {
			coordinatorNetConf.ExtraCIDR = coordinatorSpec.ExtraCIDR
		}
		if coordinatorSpec.PodMACPrefix != nil {
			coordinatorNetConf.MacPrefix = *coordinatorSpec.PodMACPrefix
		}
		if coordinatorSpec.TunePodRoutes != nil {
			coordinatorNetConf.TunePodRoutes = coordinatorSpec.TunePodRoutes
		}
		if coordinatorSpec.PodDefaultRouteNIC != nil {
			coordinatorNetConf.PodDefaultRouteNIC = *coordinatorSpec.PodDefaultRouteNIC
		}
		if coordinatorSpec.HostRuleTable != nil {
			coordinatorNetConf.HostRuleTable = pointer.Int64(int64(*coordinatorSpec.HostRuleTable))
		}
		if coordinatorSpec.HostRPFilter != nil {
			coordinatorNetConf.RPFilter = int32(*coordinatorSpec.HostRPFilter)
		}
		if coordinatorSpec.DetectIPConflict != nil {
			coordinatorNetConf.IPConflict = coordinatorSpec.DetectIPConflict
		}
		if coordinatorSpec.DetectGateway != nil {
			coordinatorNetConf.DetectGateway = coordinatorSpec.DetectGateway
		}
	}

	return coordinatorNetConf
}
