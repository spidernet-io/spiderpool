// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package draPlugin

import (
	"context"
	"fmt"
	"os"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/lock"
	"go.uber.org/zap"
)

type NodeDeviceState struct {
	lock.RWMutex
	cdi            *CDIHandler
	preparedClaims map[string]struct{}
}

func NewDeviceState(logger *zap.Logger, cdiRoot, so string) (*NodeDeviceState, error) {
	fileInfo, err := os.Stat(so)
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to stat so: %v", err)
	case fileInfo.IsDir():
		return nil, fmt.Errorf("libraryPath is not a file type")
	}

	cdi, err := NewCDIHandler(logger,
		WithCDIRoot(cdiRoot),
		WithClass(constant.DRACDIClass),
		WithVendor(constant.DRACDIVendor),
		WithSoPath(so),
	)
	if err != nil {
		return nil, err
	}

	return &NodeDeviceState{
		cdi:            cdi,
		preparedClaims: make(map[string]struct{}),
	}, nil
}

func (nds *NodeDeviceState) Prepare(ctx context.Context, claimUID string, scp *v2beta1.SpiderClaimParameter) ([]string, error) {
	nds.Lock()
	defer nds.Unlock()

	_, preprared := nds.preparedClaims[claimUID]
	if preprared {
		return nds.cdi.GetClaimDevices(claimUID), nil
	}

	if err := nds.cdi.CreateClaimSpecFile(claimUID, scp); err != nil {
		return nil, fmt.Errorf("unable to create CDI spec file for claim: %w", err)
	}

	nds.preparedClaims[claimUID] = struct{}{}
	return nds.cdi.GetClaimDevices(claimUID), nil
}

func (nds *NodeDeviceState) UnPrepare(ctx context.Context, claimUID string) error {
	nds.Lock()
	defer nds.Unlock()

	_, ok := nds.preparedClaims[claimUID]
	if ok {
		delete(nds.preparedClaims, claimUID)
	}

	return nds.cdi.DeleteClaimSpecFile(claimUID)
}
