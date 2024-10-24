#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

set -o errexit -o nounset -o pipefail

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY" && exit 1
echo "$CURRENT_FILENAME : E2E_IP_FAMILY $E2E_IP_FAMILY "

[ -z "$KDOCTOR_VERSION" ] && echo "error, miss KDOCTOR_VERSION" && exit 1
echo "$CURRENT_FILENAME : KDOCTOR_VERSION $KDOCTOR_VERSION "

[ -z "$KDOCTOR_REPORT_PATH" ] && echo "error, miss KDOCTOR_REPORT_PATH" && exit 1
echo "$CURRENT_FILENAME : KDOCTOR_REPORT_PATH $KDOCTOR_REPORT_PATH "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

KDOCTOR_VERSION=${KDOCTOR_VERSION:-0.2.2}
E2E_KDOCTOR_IMAGE_REPO=${E2E_KDOCTOR_IMAGE_REPO:-"ghcr.io"}

INSTALL_TIME_OUT=300s

KDOCTOR_HELM_OPTIONS=" --set feature.aggregateReport.enabled=true \
                            --set feature.aggregateReport.controller.reportHostPath=${KDOCTOR_REPORT_PATH} \
                            --set feature.nethttp_defaultConcurrency=5 \
                            --set feature.netdns_defaultConcurrency=5 "

case ${E2E_IP_FAMILY} in
  ipv4)
    KDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=true \
    --set feature.enableIPv6=false "
    ;;
  ipv6)
    KDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=false \
    --set feature.enableIPv6=true "
    ;;
  dual)
    KDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=true \
    --set feature.enableIPv6=true "
    ;;
  *)
    echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
esac

KDOCTOR_HELM_OPTIONS+=" --set kdoctorAgent.image.registry=${E2E_KDOCTOR_IMAGE_REPO} \
 --set kdoctorController.image.registry=${E2E_KDOCTOR_IMAGE_REPO} "

echo "KDOCTOR_HELM_OPTIONS: ${KDOCTOR_HELM_OPTIONS}"

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

helm repo add kdoctor https://kdoctor-io.github.io/kdoctor
helm repo update
HELM_IMAGES_LIST=` helm template test kdoctor/kdoctor --version ${KDOCTOR_VERSION} ${KDOCTOR_HELM_OPTIONS} | grep " image: " | tr -d '"'| awk '{print $2}' | uniq `

[ -z "${HELM_IMAGES_LIST}" ] && echo "can't found image of KDOCTOR" && exit 1
LOCAL_IMAGE_LIST=`docker images | awk '{printf("%s:%s\n",$1,$2)}'`

for IMAGE in ${HELM_IMAGES_LIST}; do
  if ! grep ${IMAGE} <<< ${LOCAL_IMAGE_LIST}; then
      echo "===> docker pull ${IMAGE}... "
      docker pull ${IMAGE}
  fi
  echo "===> load image ${IMAGE} to kind..."
  kind load docker-image ${IMAGE} --name $E2E_CLUSTER_NAME
done

# Install KDOCTOR
helm upgrade --install kdoctor kdoctor/kdoctor -n kube-system --debug --kubeconfig ${E2E_KUBECONFIG} ${KDOCTOR_HELM_OPTIONS} --version ${KDOCTOR_VERSION}

# no matching resources found
sleep 3

kubectl wait --for=condition=ready -l app.kubernetes.io/instance=kdoctor --timeout=300s pod -n kube-system \
--kubeconfig ${E2E_KUBECONFIG} 

echo -e "\033[35m Succeed to install kdoctor \033[0m"