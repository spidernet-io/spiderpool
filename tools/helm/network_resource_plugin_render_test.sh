#!/usr/bin/env bash
# Copyright 2026 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

CHART_DIR="${CHART_DIR:-charts/spiderpool}"

render() {
  helm template spiderpool "${CHART_DIR}" "$@"
}

render >/tmp/spiderpool-network-resource-disabled.yaml
if grep -q "kubelet-device-plugins" /tmp/spiderpool-network-resource-disabled.yaml; then
  echo "network resource plugin mounts rendered while disabled" >&2
  exit 1
fi

render \
  --set spiderpoolAgent.networkResourcePlugin.enabled=true \
  --set spiderpoolAgent.networkResourcePlugin.kubeletRootDir=/var/lib/custom-kubelet \
  --set-string spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[0].resourceName=spidernet.io/sub-eni \
  --set spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[0].defaultMaxCount=8 \
  --set spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI.rules[0].nodeSelector.key=value \
  --set spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules[0].defaultMaxCount=32 \
  --set spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules[0].nodeSelector.kubernetes\\.io/os=linux \
  --set-string 'spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules[0].includeInterfaces[0]=nrpdm*' \
  >/tmp/spiderpool-network-resource-enabled.yaml

grep -q "networkResourcePlugin:" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "/var/lib/custom-kubelet/device-plugins" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "/var/lib/custom-kubelet/plugins_registry" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "resourceName: spidernet.io/sub-eni" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "defaultMaxCount: 8" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "nodeSelector:" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "key: value" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "defaultMaxCount: 32" /tmp/spiderpool-network-resource-enabled.yaml
grep -q "kubernetes.io/os: linux" /tmp/spiderpool-network-resource-enabled.yaml
grep -q 'includeInterfaces: \["nrpdm\*"\]' /tmp/spiderpool-network-resource-enabled.yaml
grep -q 'excludeInterfaces: \[\]' /tmp/spiderpool-network-resource-enabled.yaml
