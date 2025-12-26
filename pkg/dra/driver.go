package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/dra/nri"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/utils"

	"go.uber.org/zap"

	resourcev1 "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeletPluginRegistryPath = "/var/lib/kubelet/plugins_registry"
	kubeletPluginPath         = "/var/lib/kubelet/plugins"
)

type Driver struct {
	nodeName   string
	logger     *zap.Logger
	kubeClient kubernetes.Interface
	draPlugin  *kubeletplugin.Helper
	client     client.Client
	state      *DeviceState
}

// NewDriver creates a new DRA driver.
func NewDriver(ctx context.Context, client client.Client, clientSet kubernetes.Interface, enableNri bool) (*Driver, error) {
	var err error
	d := &Driver{
		logger: logutils.Logger.Named("dra"),
		client: client,
		state:  &DeviceState{},
	}

	nodeName := utils.GetNodeName()
	if nodeName == "" {
		return nil, fmt.Errorf("env %s is not set", constant.ENV_SPIDERPOOL_NODENAME)
	}
	d.nodeName = nodeName

	err = os.MkdirAll(constant.DRADriverPluginPath, 0750)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin path %s: %v", constant.DRADriverPluginSocketPath, err)
	}

	d.state, err = d.state.Init(d.logger, client)
	if err != nil {
		return nil, err
	}

	d.draPlugin, err = kubeletplugin.Start(ctx,
		d,
		kubeletplugin.NodeName(nodeName),
		kubeletplugin.KubeClient(clientSet),
		kubeletplugin.DriverName(constant.DRADriverName),
		kubeletplugin.RegistrarDirectoryPath(kubeletPluginRegistryPath),
		kubeletplugin.PluginDataDirectoryPath(constant.DRADriverPluginPath),
	)
	if err != nil {
		return nil, err
	}
	go d.PublishResources(ctx)

	if enableNri {
		err = nri.Run(ctx, client, nodeName)
		if err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *Driver) PrepareResourceClaims(ctx context.Context, claims []*resourcev1.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	d.logger.Info("PrepareResourceClaims is called", zap.Any("claims", claims))
	nri.GetCache().WarmupNode(ctx, d.client, utils.GetNodeName(), utils.GetAgentNamespace())
	result := make(map[types.UID]kubeletplugin.PrepareResult)
	for _, c := range claims {
		nri.GetCache().SetResourceClaim(c)
		nri.GetCache().IndexPodClaimsFromResourceClaim(c)
		result[c.UID] = d.nodePrepareResource(ctx, c)
	}
	return result, nil
}

func (d *Driver) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	d.logger.Info("UnprepareResourceClaims is called", zap.Any("claims", claims))
	result := make(map[types.UID]error)
	for _, c := range claims {
		result[c.UID] = d.nodeUnprepareResource(ctx, c)
	}
	return result, nil
}

func (d *Driver) HandleError(ctx context.Context, err error, msg string) {
	// See: https://pkg.go.dev/k8s.io/apimachinery/pkg/util/runtime#HandleErrorWithContext
	runtime.HandleErrorWithContext(ctx, err, msg)
}

func (d *Driver) nodePrepareResource(ctx context.Context, claim *resourcev1.ResourceClaim) kubeletplugin.PrepareResult {
	if claim.Status.Allocation == nil {
		return kubeletplugin.PrepareResult{
			Err: fmt.Errorf("resource claim '%s/%s' is not allocated", claim.Namespace, claim.Name),
		}
	}

	return kubeletplugin.PrepareResult{}
}

func (d *Driver) nodeUnprepareResource(ctx context.Context, claim kubeletplugin.NamespacedObject) error {
	rc := &resourcev1.ResourceClaim{}
	if err := d.client.Get(ctx, client.ObjectKey{Name: claim.Name, Namespace: claim.Namespace}, rc); err == nil {
		for _, consumer := range rc.Status.ReservedFor {
			if consumer.Resource != "pods" {
				continue
			}
			if consumer.UID != "" {
				nri.GetCache().DeletePodClaimIndexByUID(string(consumer.UID))
			}
			if consumer.Name != "" {
				nri.GetCache().DeletePodClaimIndexByNSName(rc.Namespace, consumer.Name)
			}
		}
		nri.GetCache().DeleteResourceClaim(rc.Namespace, rc.Name)
	}
	return nil
}

// PublishResources periodically publishes the available SR-IOV resources
func (d *Driver) PublishResources(ctx context.Context) {
	devices := d.state.GetNetDevices()
	resources := resourceslice.DriverResources{
		Pools: map[string]resourceslice.Pool{
			"default": {
				Slices: []resourceslice.Slice{
					{
						Devices: devices,
					},
				},
			},
		},
	}
	if err := d.draPlugin.PublishResources(ctx, resources); err != nil {
		d.logger.Error("failed to publish resources", zap.Error(err))
	} else {
		d.logger.Info("Published DRA resources")
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
			resources := resourceslice.DriverResources{
				Pools: map[string]resourceslice.Pool{
					"default": {
						Slices: []resourceslice.Slice{
							{
								Devices: devices,
							},
						},
					},
				},
			}
			if err := d.draPlugin.PublishResources(ctx, resources); err != nil {
				d.logger.Error("failed to publish resources", zap.Error(err))
			}
		}
	}
}

func (d *Driver) Stop() {
	if d.draPlugin != nil {
		d.draPlugin.Stop()
	}
}
