package draPlugin

import (
	"fmt"
	"os"
	"path"

	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"go.uber.org/zap"
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdiparser "tags.cncf.io/container-device-interface/pkg/parser"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

type CDIHandler struct {
	cdiRoot string
	vendor  string
	class   string
	so      string

	registry cdiapi.Registry
	logger   *zap.Logger
}

type cdiOption func(*CDIHandler)

func WithCDIRoot(cdiRoot string) cdiOption {
	return func(c *CDIHandler) {
		c.cdiRoot = cdiRoot
	}
}

func WithVendor(vendor string) cdiOption {
	return func(c *CDIHandler) {
		c.vendor = vendor
	}
}

func WithClass(class string) cdiOption {
	return func(c *CDIHandler) {
		c.class = class
	}
}

func WithSoPath(so string) cdiOption {
	return func(c *CDIHandler) {
		c.so = so
	}
}

func NewCDIHandler(logger *zap.Logger, opts ...cdiOption) (*CDIHandler, error) {
	cdi := &CDIHandler{logger: logger}
	for _, opt := range opts {
		opt(cdi)
	}

	registry := cdiapi.GetRegistry(
		cdiapi.WithSpecDirs(cdi.cdiRoot),
	)
	err := registry.Refresh()
	if err != nil {
		return nil, fmt.Errorf("unable to refresh the CDI registry: %w", err)
	}
	cdi.registry = registry

	return cdi, nil
}

func (cdi *CDIHandler) GetDevice(device string) *cdiapi.Device {
	return cdi.registry.DeviceDB().GetDevice(device)
}

func (cdi *CDIHandler) GetClaimDevices(claimUID string) []string {
	devices := []string{
		cdiparser.QualifiedName(cdi.vendor, cdi.class, claimUID),
	}

	cdi.registry.DeviceDB().ListDevices()
	return devices
}

func (cdi *CDIHandler) CreateClaimSpecFile(claimUID string, scp *v2beta1.SpiderClaimParameter) error {
	cdiSpec := cdispec.Spec{
		// TODO(@cyclinder): should be make it to configureable?
		Version: cdiapi.CurrentVersion,
		Kind:    fmt.Sprintf("%s/%s", cdi.vendor, cdi.class),
		Devices: []cdispec.Device{{
			Name: claimUID,
		}},
	}

	if scp.Spec.Rdma {
		cdiSpec.ContainerEdits = cdi.getContaineEdits()
	}

	specName, err := cdiapi.GenerateNameForTransientSpec(&cdiSpec, claimUID)
	if err != nil {
		return fmt.Errorf("failed to generate CDI Spec name: %w", err)
	}

	specFileName := fmt.Sprintf("%s.%s", specName, "yaml")
	if err = cdi.registry.SpecDB().WriteSpec(&cdiSpec, specName+".yaml"); err != nil {
		return fmt.Errorf("failed to write CDI spec for claim %s: %v", claimUID, err)
	}

	if err := os.Chmod(cdi.cdiRoot+specFileName, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on spec file: %w", err)
	}
	return nil
}

func (cdi *CDIHandler) getContaineEdits() cdispec.ContainerEdits {
	soName := path.Base(cdi.so)
	return cdispec.ContainerEdits{
		Env: []string{
			fmt.Sprintf("LD_PRELOAD=%s", soName),
		},
		Mounts: []*cdispec.Mount{
			{
				HostPath:      cdi.so,
				ContainerPath: fmt.Sprintf("/usr/lib/%s", soName),
			},
			{
				HostPath:      cdi.so,
				ContainerPath: fmt.Sprintf("/usr/lib64/%s", soName),
			},
		},
	}
}
