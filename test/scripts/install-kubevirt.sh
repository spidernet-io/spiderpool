#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

set -o errexit -o nounset -o pipefail -o xtrace

CURRENT_FILENAME=$( basename $0 )

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

KUBEVIRT_VERSION_AUTO_DETECTED=false
if [ -z "${KUBEVIRT_VERSION}" ] ; then
  KUBEVIRT_VERSION=$( curl --retry 10 -s https://api.github.com/repos/kubevirt/kubevirt/releases/latest | jq '.tag_name' | tr -d '"' )
  KUBEVIRT_VERSION_AUTO_DETECTED=true
fi
[ -z "$KUBEVIRT_VERSION" ] && echo "error, miss KUBEVIRT_VERSION" && exit 1

# if network issues that make we get "null", just use 'v1.1.0' as default
if [ ${KUBEVIRT_VERSION} == "null" ]; then
  KUBEVIRT_VERSION="v1.1.0"
fi

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

# Each KubeVirt release only supports the latest three Kubernetes minor releases at its release
# time (see https://github.com/kubevirt/sig-release/blob/main/releases/k8s-support-matrix.md).
# For example, KubeVirt v1.9 requires Kubernetes >= 1.34 and VMs fail to start on older clusters
# (the VMI hangs in the Scheduled phase and never turns Running).
# When the version is auto-detected as the latest release, check the cluster's Kubernetes version
# and fall back to the newest KubeVirt release known to work with the old Kubernetes versions
# used in the CI matrix (v1.8.x works down to at least Kubernetes v1.27).
KUBEVIRT_FALLBACK_VERSION=${KUBEVIRT_FALLBACK_VERSION:-v1.8.4}
# The minimum Kubernetes minor version supported by the latest KubeVirt release (v1.9.x => 1.34).
# Bump this value together with KUBEVIRT_FALLBACK_VERSION when new KubeVirt releases come out.
KUBEVIRT_MIN_K8S_MINOR=${KUBEVIRT_MIN_K8S_MINOR:-34}
if [ "${KUBEVIRT_VERSION_AUTO_DETECTED}" == "true" ]; then
  K8S_MINOR_VERSION=$( kubectl version --kubeconfig ${E2E_KUBECONFIG} -o json | jq -r '.serverVersion.minor' | tr -cd '0-9' )
  if [ -n "${K8S_MINOR_VERSION}" ] && [ "${K8S_MINOR_VERSION}" -lt "${KUBEVIRT_MIN_K8S_MINOR}" ]; then
    echo "cluster Kubernetes minor version 1.${K8S_MINOR_VERSION} is older than 1.${KUBEVIRT_MIN_K8S_MINOR} required by the latest kubevirt ${KUBEVIRT_VERSION}, fall back to kubevirt ${KUBEVIRT_FALLBACK_VERSION}"
    KUBEVIRT_VERSION=${KUBEVIRT_FALLBACK_VERSION}
  fi
fi

echo "$CURRENT_FILENAME : KUBEVIRT_VERSION $KUBEVIRT_VERSION "

echo "E2E_KUBEVIRT_IMAGE_REPO=${E2E_KUBEVIRT_IMAGE_REPO}"

KUBEVIRT_OPERATOR_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/virt-operator:${KUBEVIRT_VERSION}
KUBEVIRT_API_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/virt-api:${KUBEVIRT_VERSION}
KUBEVIRT_CONTROLLER_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/virt-controller:${KUBEVIRT_VERSION}
KUBEVIRT_HANDLER_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/virt-handler:${KUBEVIRT_VERSION}
KUBEVIRT_LAUNCHER_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/virt-launcher:${KUBEVIRT_VERSION}
KUBEVIRT_TEST_IMAGE=${E2E_KUBEVIRT_IMAGE_REPO}/kubevirt/cirros-container-disk-demo:latest
KUBEVIRT_IMAGE_LIST="${KUBEVIRT_OPERATOR_IMAGE} ${KUBEVIRT_API_IMAGE} ${KUBEVIRT_CONTROLLER_IMAGE} ${KUBEVIRT_HANDLER_IMAGE} ${KUBEVIRT_LAUNCHER_IMAGE} ${KUBEVIRT_TEST_IMAGE}"

LOCAL_IMAGE_LIST=`docker images | awk '{printf("%s:%s\n",$1,$2)}'`

for IMAGE in ${KUBEVIRT_IMAGE_LIST}; do
  if ! grep ${IMAGE} <<< ${LOCAL_IMAGE_LIST}; then
    echo "===> docker pull ${IMAGE}... "
    for RETRY in 1 2 3; do
      if docker pull ${IMAGE}; then
        break
      fi

      if [ ${RETRY} -eq 3 ]; then
        echo "error, failed to docker pull ${IMAGE} after ${RETRY} attempts"
        exit 1
      fi

      SLEEP_SECONDS=$(( RETRY * 5 ))
      echo "docker pull ${IMAGE} failed, retry ${RETRY}/3 after ${SLEEP_SECONDS}s"
      sleep ${SLEEP_SECONDS}
    done
  fi
  echo "===> load image ${IMAGE} to kind ..."
  kind load docker-image ${IMAGE} --name $E2E_CLUSTER_NAME
done

for RETRY in 1 2 3; do
  if curl --retry 10 --retry-delay 5 -sSfL "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml" | kubectl apply -f - --kubeconfig ${E2E_KUBECONFIG}; then
    break
  fi
  if [ ${RETRY} -eq 3 ]; then
    echo "error, failed to apply kubevirt-operator.yaml after ${RETRY} attempts"
    exit 1
  fi
  SLEEP_SECONDS=$(( (RETRY - 1) * 10 ))
  echo "apply kubevirt-operator.yaml failed, retry ${RETRY}/3 after ${SLEEP_SECONDS}s"
  sleep ${SLEEP_SECONDS}
done

for RETRY in 1 2 3; do
  if curl --retry 10 --retry-delay 5 -sSfL "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml" | kubectl apply -f - --kubeconfig ${E2E_KUBECONFIG}; then
    break
  fi
  if [ ${RETRY} -eq 3 ]; then
    echo "error, failed to apply kubevirt-cr.yaml after ${RETRY} attempts"
    exit 1
  fi
  SLEEP_SECONDS=$(( (RETRY - 1) * 10 ))
  echo "apply kubevirt-cr.yaml failed, retry ${RETRY}/3 after ${SLEEP_SECONDS}s"
  sleep ${SLEEP_SECONDS}
done

KUBEVIRT_OPERATOR_ROLLOUT_TIMEOUT=${KUBEVIRT_OPERATOR_ROLLOUT_TIMEOUT:-300s}
kubectl rollout status deployment/virt-operator -n kubevirt --timeout ${KUBEVIRT_OPERATOR_ROLLOUT_TIMEOUT} --kubeconfig ${E2E_KUBECONFIG}
echo "wait kubevirt related pod running ..."

# wait for the virt-operator to set up kubevirt component pods
sleep 60

kubectl wait --for=condition=ready -l app.kubernetes.io/component=kubevirt -n kubevirt --timeout=300s pod --kubeconfig ${E2E_KUBECONFIG}

# If the kind cluster runs on a virtual machine consider enabling nested virtualization.
# We need to wait for all kubevirt component pods ready(webhook ready) to submit the patch action.
# NOTE: set "useEmulation" to allow software emulation when /dev/kvm is unavailable (kind clusters)
# NOTE: set "disableSerialConsoleLog" to avoid the log of serial console issue, it leads to the kubevirt vm pod can't running
# see: https://github.com/kubevirt/kubevirt/issues/15355, https://github.com/spidernet-io/spiderpool/issues/5177
# NOTE: disable the "ImageVolume" feature gate (Beta and enabled by default since KubeVirt v1.9.0).
# When enabled, virt-controller renders containerDisk volumes as native Kubernetes image volumes
# (spec.volumes[].image, added in k8s.io/api v0.31). The CI matrix still runs Kubernetes versions
# without image volume support, where the unknown "image" field is silently dropped by the
# apiserver and the volume is defaulted to an empty emptyDir; virt-launcher then finds no
# container disk and crash-loops, leaving the VMI stuck in Scheduling. Additionally, Spiderpool's
# pod mutating webhook (built on k8s.io/api v0.29) round-trips virt-launcher pods and strips the
# "image" volume source for the same reason. Disabling the gate falls back to the classic
# containerDisk architecture used through KubeVirt v1.8.x.
kubectl -n kubevirt patch kubevirt kubevirt --type=merge --patch '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true,"disabledFeatureGates":["ImageVolume"]},"virtualMachineOptions": {"disableSerialConsoleLog": {}}}}}' --kubeconfig ${E2E_KUBECONFIG}

# After patching the KubeVirt CR, virt-operator rolls out configuration changes to virt-handler/virt-controller.
# Wait for the rollout to complete so VMs can use the updated config (e.g., useEmulation).
sleep 10
kubectl wait --for=condition=ready -l app.kubernetes.io/component=kubevirt -n kubevirt --timeout=300s pod --kubeconfig ${E2E_KUBECONFIG}

echo -e "\033[35m Succeed to install kubevirt \033[0m"
