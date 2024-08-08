// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package draPlugin

import (
	"fmt"
	"os"
	"path"

	"github.com/spidernet-io/spiderpool/pkg/constant"
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

	return devices
}

// CreateClaimSpecFile create CDI file for the claim
func (cdi *CDIHandler) CreateClaimSpecFile(claimUID string, scp *v2beta1.SpiderClaimParameter) error {
	cdiSpec := cdispec.Spec{
		Version: cdi.getCdiVersion(scp.Annotations),
		Kind:    cdi.cdiKind(),
		Devices: []cdispec.Device{{
			Name:           claimUID,
			ContainerEdits: cdi.getContaineEdits(claimUID, false),
		}},
	}

	specName, err := cdiapi.GenerateNameForTransientSpec(&cdiSpec, claimUID)
	if err != nil {
		return fmt.Errorf("failed to generate CDI Spec name: %w", err)
	}

	specFileName := fmt.Sprintf("%s.%s", specName, "yaml")
	if err = cdi.registry.SpecDB().WriteSpec(&cdiSpec, specName+".yaml"); err != nil {
		return fmt.Errorf("failed to write CDI spec for claim %s: %v", claimUID, err)
	}

	if err := os.Chmod(path.Join(cdi.cdiRoot, specFileName), 0600); err != nil {
		return fmt.Errorf("failed to set permissions on spec file: %w", err)
	}
	return nil
}

func (cdi *CDIHandler) DeleteClaimSpecFile(claimUID string) error {
	spec := &cdispec.Spec{
		Kind: cdi.cdiKind(),
	}

	specName, err := cdiapi.GenerateNameForTransientSpec(spec, claimUID)
	if err != nil {
		return fmt.Errorf("failed to generate CDI Spec name: %w", err)
	}

	return cdi.registry.SpecDB().RemoveSpec(specName + ".yaml")
}

// nolint: all
func (cdi *CDIHandler) getContaineEdits(claim string, rdma bool) cdispec.ContainerEdits {
	ce := cdispec.ContainerEdits{
		// why do we need this?
		// a device MUST be have at lease a ContainerEdits, so if rdma is false:
		// the device have empty ContainerEdits, which cause the container can't
		// be started.
		Env: []string{
			fmt.Sprintf("DRA_CLAIM_UID=%s", claim),
		},
	}

	if rdma {
		soName := path.Base(cdi.so)
		ce.Env = append(ce.Env, fmt.Sprintf("LD_PRELOAD=%s", soName))
		ce.Mounts = []*cdispec.Mount{
			{
				HostPath:      cdi.so,
				ContainerPath: fmt.Sprintf("/usr/lib/%s", soName),
				Options:       []string{"ro", "nosuid", "nodev", "bind"},
			},
			{
				HostPath:      cdi.so,
				ContainerPath: fmt.Sprintf("/usr/lib64/%s", soName),
				Options:       []string{"ro", "nosuid", "nodev", "bind"},
			},
		}
	}

	return ce
}

func (cdi *CDIHandler) cdiKind() string {
	return cdi.vendor + "/" + cdi.class
}

// getCdiVersion return the cdi version, it can be configure by
// spiderclaimparameter's annotation: ipam.spidernet.io/cdi-version
func (cdi CDIHandler) getCdiVersion(annotations map[string]string) string {
	version := cdiapi.CurrentVersion
	if annotations == nil {
		return version
	}

	v, ok := annotations[constant.AnnoDraCdiVersion]
	if ok {
		return v
	}

	return version
}
