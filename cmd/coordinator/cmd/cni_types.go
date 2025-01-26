// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/go-openapi/strfmt"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var (
	defaultLogPath          = "/var/log/spidernet/coordinator.log"
	defaultUnderlayVethName = "veth0"
	defaultMarkBit          = 0 // ox1
	// by default, k8s pod's first NIC is eth0
	defaultOverlayVethName  = "eth0"
	defaultPodRuleTable     = 100
	defaultHostRulePriority = 1000
	BinNamePlugin           = filepath.Base(os.Args[0])
)

type Mode string

const (
	ModeAuto     Mode = "auto"
	ModeUnderlay Mode = "underlay"
	ModeOverlay  Mode = "overlay"
	ModeDisable  Mode = "disable"
)

type Config struct {
	types.NetConf
	VethLinkAddress    string         `json:"vethLinkAddress,omitempty"`
	MacPrefix          string         `json:"podMACPrefix,omitempty"`
	MultusNicPrefix    string         `json:"multusNicPrefix,omitempty"`
	PodDefaultCniNic   string         `json:"podDefaultCniNic,omitempty"`
	OverlayPodCIDR     []string       `json:"overlayPodCIDR,omitempty"`
	ServiceCIDR        []string       `json:"serviceCIDR,omitempty"`
	HijackCIDR         []string       `json:"hijackCIDR,omitempty"`
	TunePodRoutes      *bool          `json:"tunePodRoutes,omitempty"`
	PodDefaultRouteNIC string         `json:"podDefaultRouteNic,omitempty"`
	Mode               Mode           `json:"mode,omitempty"`
	HostRuleTable      *int64         `json:"hostRuleTable,omitempty"`
	HostRPFilter       *int32         `json:"hostRPFilter,omitempty" `
	PodRPFilter        *int32         `json:"podRPFilter,omitempty" `
	DetectOptions      *DetectOptions `json:"detectOptions,omitempty"`
	LogOptions         *LogOptions    `json:"logOptions,omitempty"`
}

// DetectOptions enable ip conflicting check for pod's ip
type DetectOptions struct {
	Retry    int    `json:"retries,omitempty"`
	Interval string `json:"interval,omitempty"`
	TimeOut  string `json:"timeout,omitempty"`
}

type LogOptions struct {
	LogLevel        string `json:"logLevel"`
	LogFilePath     string `json:"logFile"`
	LogFileMaxSize  int    `json:"logMaxSize"`
	LogFileMaxAge   int    `json:"logMaxAge"`
	LogFileMaxCount int    `json:"logMaxCount"`
}

const (
	CniVersion030 = "0.3.0"
	CniVersion031 = "0.3.1"
	CniVersion040 = "0.4.0"
	CniVersion100 = "1.0.0"
)

// SupportCNIVersions indicate the CNI version that spiderpool support.
var SupportCNIVersions = []string{CniVersion030, CniVersion031, CniVersion040, CniVersion100}

// ParseConfig parses the supplied configuration (and prevResult) from stdin.
func ParseConfig(stdin []byte, coordinatorConfig *models.CoordinatorConfig) (*Config, error) {
	var err error
	conf := Config{}

	if err = json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}

	if err = version.ParsePrevResult(&conf.NetConf); err != nil {
		return nil, fmt.Errorf("failed to parse prevResult: %v", err)
	}

	if err = coordinatorConfig.Validate(strfmt.Default); err != nil {
		return nil, err
	}

	if conf.PodDefaultCniNic == "" {
		conf.PodDefaultCniNic = defaultOverlayVethName
	}

	if conf.LogOptions == nil {
		conf.LogOptions = &LogOptions{
			LogLevel: logutils.DebugLevel.String(),
		}
	}

	logLevel := logutils.ConvertLogLevel(conf.LogOptions.LogLevel)
	if logLevel == nil {
		return nil, fmt.Errorf("unsupported log level %s", conf.LogOptions.LogLevel)
	}

	if conf.LogOptions.LogFilePath == "" {
		conf.LogOptions.LogFilePath = defaultLogPath
	}

	if conf.MacPrefix == "" {
		conf.MacPrefix = coordinatorConfig.PodMACPrefix
	}

	if err = validateHwPrefix(conf.MacPrefix); err != nil {
		return nil, err
	}

	if err = ValidateRoutes(&conf, coordinatorConfig); err != nil {
		return nil, err
	}

	// value must be -1,0/1/2
	if conf.PodRPFilter, err = validateRPFilterConfig(conf.PodRPFilter, coordinatorConfig.PodRPFilter); err != nil {
		return nil, err
	}

	if conf.HostRuleTable == nil && coordinatorConfig.HostRuleTable > 0 {
		conf.HostRuleTable = ptr.To(int64(coordinatorConfig.HostRuleTable))
	}

	if conf.HostRuleTable == nil {
		conf.HostRuleTable = ptr.To(int64(500))
	}

	if conf.TunePodRoutes == nil {
		conf.TunePodRoutes = coordinatorConfig.TunePodRoutes
	}

	if conf.Mode == "" {
		conf.Mode = Mode(*coordinatorConfig.Mode)
	}

	if conf.PodDefaultRouteNIC == "" && coordinatorConfig.PodDefaultRouteNIC != "" {
		conf.PodDefaultRouteNIC = coordinatorConfig.PodDefaultRouteNIC
	}

	if conf.VethLinkAddress == "" {
		conf.VethLinkAddress = coordinatorConfig.VethLinkAddress
	}
	return &conf, nil
}

func validateHwPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	// prefix format like: 00:00、0a:1b
	matchRegexp, err := regexp.Compile("^" + "(" + "[a-fA-F0-9][a-fA-F,0,2-9][:-][a-fA-F0-9]{2}" + ")" + "$")
	if err != nil {
		return err
	}
	if !matchRegexp.MatchString(prefix) {
		return fmt.Errorf("mac_prefix format should be match regex: [a-fA-F0-9][a-fA-F,0,2-9][:][a-fA-F0-9]{2}, like '0a:1b'")
	}

	return nil
}

func ValidateRoutes(conf *Config, coordinatorConfig *models.CoordinatorConfig) error {
	if len(conf.ServiceCIDR) == 0 {
		conf.ServiceCIDR = coordinatorConfig.ServiceCIDR
	}

	if len(conf.OverlayPodCIDR) == 0 {
		conf.OverlayPodCIDR = coordinatorConfig.OverlayPodCIDR
	}

	if len(conf.HijackCIDR) == 0 {
		conf.HijackCIDR = coordinatorConfig.HijackCIDR
	}

	var err error
	err = validateRoutes(conf.ServiceCIDR)
	if err != nil {
		return err
	}

	err = validateRoutes(conf.OverlayPodCIDR)
	if err != nil {
		return err
	}

	err = validateRoutes(conf.HijackCIDR)
	if err != nil {
		return err
	}

	return nil
}

func validateRoutes(routes []string) error {
	result := make([]string, len(routes))
	for idx, route := range routes {
		result[idx] = strings.TrimSpace(route)
	}
	for _, route := range result {
		_, _, err := net.ParseCIDR(route)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateRPFilterConfig(rpfilter *int32, coordinatorConfig int64) (*int32, error) {
	if rpfilter == nil {
		rpfilter = ptr.To(int32(coordinatorConfig))
	}

	found := false
	// NOTE: negative number means disable
	if *rpfilter >= 0 {
		for _, value := range []int32{0, 1, 2} {
			if *rpfilter == value {
				found = true
				break
			}
		}
	} else {
		found = true
	}

	if !found {
		return nil, fmt.Errorf("invalid rp_filter value %v, available options: [-1,0,1,2]", rpfilter)
	}
	return rpfilter, nil
}
