package draPlugin

import (
	"fmt"
	"os"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"go.uber.org/zap"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
)

func StartDRAPlugin(logger *zap.Logger, cdiRoot, so string) (kubeletplugin.DRAPlugin, error) {
	err := os.MkdirAll(constant.DRADriverPluginPath, 0750)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(cdiRoot)
	switch {
	case err != nil && os.IsNotExist(err):
		if err = os.MkdirAll(cdiRoot, 0750); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	case !fileInfo.IsDir():
		return nil, fmt.Errorf("cdi path %s isn't a directory", cdiRoot)
	}

	driver, err := NewDriver(logger.Named("DRA"), cdiRoot, so)
	if err != nil {
		return nil, err
	}

	dp, err := kubeletplugin.Start(driver,
		kubeletplugin.DriverName(constant.DRADriverName),
		kubeletplugin.RegistrarSocketPath(constant.DRAPluginRegistrationPath),
		kubeletplugin.PluginSocketPath(constant.DRADriverPluginSocketPath),
		kubeletplugin.KubeletPluginSocketPath(constant.DRADriverPluginSocketPath),
	)
	if err != nil {
		return nil, err
	}

	return dp, nil
}
