// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"fmt"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/utils"
)

func InitMultusDefaultCR(ctx context.Context, config *InitDefaultConfig, client *CoreClient) error {
	defaultCNIName, defaultCNIType, err := fetchDefaultCNIName(config.DefaultCNIName, config.DefaultCNIDir)
	if err != nil {
		return err
	}

	if err = client.WaitMultusCNIConfigCreated(ctx, getMultusCniConfig(defaultCNIName, defaultCNIType, config.DefaultCNINamespace)); err != nil {
		return err
	}

	return nil
}

func fetchDefaultCNIName(defaultCNIName, cniDir string) (cniName, cniType string, err error) {
	if defaultCNIName != "" {
		return defaultCNIName, constant.CustomCNI, nil
	}

	defaultCNIConfPath, err := utils.GetDefaultCNIConfPath(cniDir)
	if err != nil {
		logger.Sugar().Errorf("failed to findDefaultCNIConf: %v", err)
		return "", "", fmt.Errorf("failed to findDefaultCNIConf: %v", err)
	}
	return parseCNIFromConfig(defaultCNIConfPath)
}
