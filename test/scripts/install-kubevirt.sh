#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

set -o errexit -o nounset -o pipefail -o xtrace

CURRENT_FILENAME=$( basename $0 )

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

if [ -z "${KUBEVIRT_VERSION}" ] ; then
  KUBEVIRT_VERSION=$( curl --retry 10 -s https://api.github.com/repos/kubevirt/kubevirt/releases/latest | jq '.tag_name' | tr -d '"' )
fi
[ -z "$KUBEVIRT_VERSION" ] && echo "error, miss KUBEVIRT_VERSION" && exit 1

# if network issues that make we get "null", just use 'v1.1.0' as default
if [ ${KUBEVIRT_VERSION} == "null" ]; then
  KUBEVIRT_VERSION="v1.1.0"
fi

echo "$CURRENT_FILENAME : KUBEVIRT_VERSION $KUBEVIRT_VERSION "

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

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
    docker pull ${IMAGE}
  fi
  echo "===> load image ${IMAGE} to kind ..."
  kind load docker-image ${IMAGE} --name $E2E_CLUSTER_NAME
done

kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml --kubeconfig ${E2E_KUBECONFIG}

kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml --kubeconfig ${E2E_KUBECONFIG}

kubectl rollout status deployment/virt-operator -n kubevirt --timeout 120s --kubeconfig ${E2E_KUBECONFIG}
echo "wait kubevirt related pod running ..."

# wait for the virt-operator to set up kubevirt component pods
sleep 60

kubectl wait --for=condition=ready -l app.kubernetes.io/component=kubevirt -n kubevirt --timeout=300s pod --kubeconfig ${E2E_KUBECONFIG}

# If the kind cluster runs on a virtual machine consider enabling nested virtualization.
# Enable the network Passt and LiveMigration feature.
# We need to wait for all kubevirt component pods ready(webhook ready) to submit the patch action.
# NOTE: set "disableSerialConsoleLog" to avoid the log of serial console issue, it leads to the kubevirt vm pod can't running
# see: https://github.com/kubevirt/kubevirt/issues/15355, https://github.com/spidernet-io/spiderpool/issues/5177
kubectl -n kubevirt patch kubevirt kubevirt --type=merge --patch '{"spec":{"configuration":{"developerConfiguration":{"useEmulation":true},"virtualMachineOptions": {"disableSerialConsoleLog": {},"featureGates": ["Passt"]}}}}' --kubeconfig ${E2E_KUBECONFIG}

sleep 1

echo -e "\033[35m Succeed to install kubevirt \033[0m"
