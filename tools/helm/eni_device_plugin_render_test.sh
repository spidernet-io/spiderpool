#!/usr/bin/env bash
# Copyright 2026 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "${SCRIPT_DIR}/../.." && pwd)
CHART_DIR="${REPO_ROOT}/charts/spiderpool"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is required" >&2
  exit 1
fi

disabled_output=$(helm template spiderpool "${CHART_DIR}" --namespace kube-system)
grep -q "eniDevPlugin:" <<<"${disabled_output}"
grep -q "resourceName: \"spidernet.io/eni-slot\"" <<<"${disabled_output}"
grep -q "kubeletRootDir: \"/var/lib/kubelet\"" <<<"${disabled_output}"
if grep -q "kubelet-device-plugins" <<<"${disabled_output}"; then
  echo "kubelet device plugin mount must not render when eniDevPlugin.enabled=false" >&2
  exit 1
fi
if grep -q "kubelet-plugins-registry" <<<"${disabled_output}"; then
  echo "kubelet plugins registry mount must not render when eniDevPlugin.enabled=false" >&2
  exit 1
fi

provider_disabled_output=$(helm template spiderpool "${CHART_DIR}" --namespace kube-system \
  --set iaasNetworkProvider.eniDevPlugin.enabled=true \
  --set iaasNetworkProvider.eniDevPlugin.maxSlotsPerNode=8)
grep -q "enabled: true" <<<"${provider_disabled_output}"
if grep -q "kubelet-device-plugins" <<<"${provider_disabled_output}"; then
  echo "kubelet device plugin mount must not render when iaasNetworkProvider.serverUrl is empty" >&2
  exit 1
fi
if grep -q "kubelet-plugins-registry" <<<"${provider_disabled_output}"; then
  echo "kubelet plugins registry mount must not render when iaasNetworkProvider.serverUrl is empty" >&2
  exit 1
fi

enabled_output=$(helm template spiderpool "${CHART_DIR}" --namespace kube-system \
  --set iaasNetworkProvider.serverUrl=http://provider.example \
  --set iaasNetworkProvider.eniDevPlugin.enabled=true \
  --set iaasNetworkProvider.eniDevPlugin.maxSlotsPerNode=8)
grep -q "enabled: true" <<<"${enabled_output}"
grep -q "maxSlotsPerNode: 8" <<<"${enabled_output}"
grep -q 'mountPath: "/var/lib/kubelet/device-plugins"' <<<"${enabled_output}"
grep -q 'mountPath: "/var/lib/kubelet/plugins_registry"' <<<"${enabled_output}"
grep -q 'path: "/var/lib/kubelet/device-plugins"' <<<"${enabled_output}"
grep -q 'path: "/var/lib/kubelet/plugins_registry"' <<<"${enabled_output}"

custom_root_output=$(helm template spiderpool "${CHART_DIR}" --namespace kube-system \
  --set iaasNetworkProvider.serverUrl=http://provider.example \
  --set iaasNetworkProvider.eniDevPlugin.enabled=true \
  --set iaasNetworkProvider.eniDevPlugin.maxSlotsPerNode=8 \
  --set iaasNetworkProvider.eniDevPlugin.kubeletRootDir=/var/log/kubelet)
grep -q "kubeletRootDir: \"/var/log/kubelet\"" <<<"${custom_root_output}"
grep -q 'mountPath: "/var/log/kubelet/device-plugins"' <<<"${custom_root_output}"
grep -q 'mountPath: "/var/log/kubelet/plugins_registry"' <<<"${custom_root_output}"
grep -q 'path: "/var/log/kubelet/device-plugins"' <<<"${custom_root_output}"
grep -q 'path: "/var/log/kubelet/plugins_registry"' <<<"${custom_root_output}"
