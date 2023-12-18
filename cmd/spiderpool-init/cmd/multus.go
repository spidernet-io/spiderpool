// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	controller_client "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/utils"
)

// MultusNetConf for cni config file written in json
// Note: please keep this fields be consistent with multus configMap
// in charts/spiderpool/templates/multus/multus-daemonset.yaml
type MultusNetConf struct {
	CNIVersion   string          `json:"cniVersion,omitempty"`
	Name         string          `json:"name,omitempty"`
	Type         string          `json:"type,omitempty"`
	ConfDir      string          `json:"confDir"`
	LogLevel     string          `json:"logLevel"`
	LogFile      string          `json:"logFile"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	// Option to isolate the usage of CR's to the namespace in which a pod resides.
	NamespaceIsolation bool     `json:"namespaceIsolation"`
	ClusterNetwork     string   `json:"clusterNetwork"`
	DefaultNetworks    []string `json:"defaultNetworks"`
	// Option to set the namespace that multus-cni uses (clusterNetwork/defaultNetworks)
	MultusNamespace string `json:"multusNamespace"`
	// Option to set system namespaces (to avoid to add defaultNetworks)
	SystemNamespaces []string `json:"systemNamespaces"`
	Kubeconfig       string   `json:"kubeconfig"`
}

func InitMultusDefaultCR(ctx context.Context, config *InitDefaultConfig, client *CoreClient) error {
	defaultCNIName, defaultCNIType, err := fetchDefaultCNIName(config.DefaultCNIName, config.DefaultCNIDir)
	if err != nil {
		return err
	}

	if err = client.WaitMultusCNIConfigCreated(ctx, getMultusCniConfig(defaultCNIName, defaultCNIType, config.DefaultCNINamespace)); err != nil {
		return err
	}

	if !config.installMultusCNI {
		logger.Sugar().Infof("No install MultusCNI, Ignore update clusterNetwork for multus configMap")
		return nil
	}

	// get multus configMap
	cm, err := getConfigMap(ctx, client, config.DefaultCNINamespace, config.MultusConfigMap)
	if err != nil {
		logger.Sugar().Errorf("get configMap: %v", err)
		return err
	}

	var multusConfig MultusNetConf
	cniConfig := cm.Data["cni-conf.json"]
	if err := json.Unmarshal([]byte(cniConfig), &multusConfig); err != nil {
		return fmt.Errorf("failed to unmarshal multus config: %v", err)
	}

	if multusConfig.ClusterNetwork == defaultCNIName {
		// if clusterNetwork is expected, just return
		logger.Sugar().Infof("multus clusterNetwork is %s, don't need to update multus configMap", defaultCNIName)
		return nil
	}

	oldConfigMap := cm.DeepCopy()
	multusConfig.ClusterNetwork = defaultCNIName
	configDatas, err := json.Marshal(multusConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal multus config: %v", err)
	}
	cm.Data["cni-conf.json"] = string(configDatas)

	logger.Sugar().Infof("Try to patch multus configMap %s: %s", config.MultusConfigMap, configDatas)
	if err = client.Patch(ctx, cm, controller_client.MergeFrom(oldConfigMap)); err != nil {
		return fmt.Errorf("failed to patch multus configMap: %v", err)
	}

	// we need restart spideragent-pod after we patch the configmap, make sure these changes works immediately
	if err = restartSpiderAgent(ctx, client, config.AgentName, config.DefaultCNINamespace); err != nil {
		return err
	}

	logger.Sugar().Infof("successfully restart spiderpool-agent")
	return nil
}

func makeReadinessReady(config *InitDefaultConfig) error {
	// tell readness by writing to the file that the spiderpool is ready
	readinessDir := path.Dir(config.ReadinessFile)
	err := os.MkdirAll(readinessDir, 0644)
	if err != nil {
		return err
	}

	if err = os.WriteFile(config.ReadinessFile, []byte("ready"), 0777); err != nil {
		return err
	}
	logger.Sugar().Infof("success to make spiderpool-init pod's readiness to ready")
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

func getConfigMap(ctx context.Context, client *CoreClient, namespace, name string) (*corev1.ConfigMap, error) {
	var cm corev1.ConfigMap
	if err := client.Get(ctx, apitypes.NamespacedName{Name: name, Namespace: namespace}, &cm); err != nil {
		return nil, err
	}

	return &cm, nil
}

func restartSpiderAgent(ctx context.Context, client *CoreClient, name, ns string) error {
	logger.Sugar().Infof("Try to restart spiderpoo-agent daemonSet: %s/%s", ns, name)

	var spiderAgent v1.DaemonSet
	var err error
	if err = client.Get(ctx, apitypes.NamespacedName{Name: name, Namespace: ns}, &spiderAgent); err != nil {
		return err
	}

	if err = client.DeleteAllOf(ctx, &corev1.Pod{}, controller_client.InNamespace(ns), controller_client.MatchingLabels(spiderAgent.Spec.Template.Labels)); err != nil {
		return err
	}

	if err = client.WaitPodListReady(ctx, ns, spiderAgent.Spec.Template.Labels); err != nil {
		return err
	}
	return nil
}
