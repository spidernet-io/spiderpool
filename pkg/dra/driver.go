package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	resourcev1beta1 "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	drapb "k8s.io/kubelet/pkg/apis/dra/v1beta1"
)

const (
	kubeletPluginRegistryPath = "/var/lib/kubelet/plugins_registry"
	kubeletPluginPath         = "/var/lib/kubelet/plugins"
)

type Driver struct {
	logger     *zap.Logger
	kubeClient kubernetes.Interface
	draPlugin  kubeletplugin.DRAPlugin
	clientSet  *kubernetes.Clientset
	state      *DeviceState
}

func NewDriver(ctx context.Context, clientSet *kubernetes.Clientset) (*Driver, error) {
	var err error
	d := &Driver{
		logger:    logutils.Logger.Named("dra"),
		clientSet: clientSet,
	}

	nodeName := os.Getenv(constant.ENV_SPIDERPOOL_NODENAME)
	if nodeName == "" {
		return nil, fmt.Errorf("env %s is not set", constant.ENV_SPIDERPOOL_NODENAME)
	}

	err = os.MkdirAll(constant.DRADriverPluginPath, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin path %s: %v", constant.DRADriverPluginSocketPath, err)
	}

	d.state, err = d.state.Init(d.logger)
	if err != nil {
		return nil, err
	}

	d.draPlugin, err = kubeletplugin.Start(ctx,
		[]any{d},
		kubeletplugin.NodeName(nodeName),
		kubeletplugin.KubeClient(clientSet),
		kubeletplugin.DriverName(constant.DRADriverName),
		kubeletplugin.RegistrarSocketPath(constant.DRAPluginRegistrationPath),
		kubeletplugin.PluginSocketPath(constant.DRADriverPluginSocketPath),
		kubeletplugin.KubeletPluginSocketPath(constant.DRADriverPluginSocketPath),
	)
	if err != nil {
		return nil, err
	}

	go d.PublishResources(ctx)

	return d, nil
}

func (d *Driver) NodePrepareResources(ctx context.Context, request *drapb.NodePrepareResourcesRequest) (*drapb.NodePrepareResourcesResponse, error) {
	d.logger.Info("NodePrepareResources is called", zap.Any("claims", request.Claims))
	resp := &drapb.NodePrepareResourcesResponse{
		Claims: make(map[string]*drapb.NodePrepareResourceResponse, len(request.Claims)),
	}
	for _, c := range request.Claims {
		devices, err := d.nodePrepareResource(ctx, c)
		if err != nil {
			resp.Claims[c.UID] = &drapb.NodePrepareResourceResponse{
				Error: err.Error(),
			}
		} else {
			resp.Claims[c.UID] = &drapb.NodePrepareResourceResponse{
				Devices: devices,
			}
		}
	}
	return resp, nil
}

func (d *Driver) NodeUnprepareResources(ctx context.Context, req *drapb.NodeUnprepareResourcesRequest) (*drapb.NodeUnprepareResourcesResponse, error) {
	d.logger.Info("NodeUnprepareResources is called", zap.Any("claims", req.Claims))
	resp := &drapb.NodeUnprepareResourcesResponse{
		Claims: make(map[string]*drapb.NodeUnprepareResourceResponse, len(req.Claims)),
	}

	for _, c := range req.Claims {
		resp.Claims[c.UID] = &drapb.NodeUnprepareResourceResponse{}
	}
	return resp, nil
}

func (d *Driver) nodePrepareResource(ctx context.Context, claim *drapb.Claim) (devices []*drapb.Device, err error) {
	resourceClaim, err := d.clientSet.ResourceV1beta1().ResourceClaims(claim.Namespace).Get(ctx, claim.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get resource claim '%s/%s': %v", claim.Namespace, claim.Name, err)
	}

	if resourceClaim.Status.Allocation == nil {
		return nil, fmt.Errorf("resource claim '%s/%s' is not allocated", claim.Namespace, claim.Name)
	}

	if claim.UID != string(resourceClaim.UID) {
		return nil, fmt.Errorf("request resource claim '%s/%s' uid is expect %s, but got uid %s", claim.Namespace, claim.Name, claim.UID, resourceClaim.UID)
	}

	// get the current pod
	// we expect one resourceClaim is only reserved for one pod
	// so we only need to get the first pod
	// var pod *corev1.Pod
	// for _, reserved := range resourceClaim.Status.ReservedFor {
	// 	if reserved.APIGroup == "" && reserved.Resource == "pods" {
	// 		pod, err = d.podManager.GetPodByName(ctx, claim.Namespace, reserved.Name, true)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("failed to get pod '%s/%s': %v", claim.Namespace, reserved.Name, err)
	// 		}
	// 		break
	// 	}
	// }

	// d.prepareMultusConfigs()

	return devices, nil
}

func (d *Driver) prepare() error {
	// parse the resourceclaim network config
	return nil

}

func (d *Driver) prepareMultusConfigs(pod *corev1.Pod, configs []resourcev1beta1.DeviceClaimConfiguration) error {
	multusConfig, err := ParseNetworkConfig(configs)
	if err != nil {
		return fmt.Errorf("failed to get network config from resource claim: %v", err)
	}

	if multusConfig.SecondaryNics != nil {
	}

	return nil
}

// PublishResources periodically publishes the available SR-IOV resources
func (d *Driver) PublishResources(ctx context.Context) {
	d.logger.Info("Starting to publish resources")
	devices := d.state.GetNetDevices()
	if err := d.draPlugin.PublishResources(ctx, kubeletplugin.Resources{Devices: devices}); err != nil {
		d.logger.Error("failed to publish resources", zap.Error(err))
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("receive ctx done, stop publishing resources")
			return
		case <-ticker.C:
			// TODO: we should use netlink.LinkSubscribe to watch any changes of the netlink
			// if one device is allocated/deallocated to a pod, we can update the device state in time
			// which make sure the same device will not be allocated to different pods
			// get the latest state of the netlink
			devices := d.state.GetNetDevices()
			if err := d.draPlugin.PublishResources(ctx, kubeletplugin.Resources{Devices: devices}); err != nil {
				d.logger.Error("failed to publish resources", zap.Error(err))
			} else {
				d.logger.Info("published resources")
			}
		}
	}
}

func (d *Driver) Stop() {
	if d.draPlugin == nil {
		return
	}
	d.draPlugin.Stop()
}
