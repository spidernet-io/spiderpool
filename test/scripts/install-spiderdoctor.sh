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

[ -z "$SPIDERDOCTOR_VERSION" ] && echo "error, miss SPIDERDOCTOR_VERSION" && exit 1
echo "$CURRENT_FILENAME : SPIDERDOCTOR_VERSION $SPIDERDOCTOR_VERSION "

[ -z "$SPIDERDOCTOR_REPORT_PATH" ] && echo "error, miss SPIDERDOCTOR_REPORT_PATH" && exit 1
echo "$CURRENT_FILENAME : SPIDERDOCTOR_REPORT_PATH $SPIDERDOCTOR_REPORT_PATH "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

SPIDERDOCTOR_VERSION=${SPIDERDOCTOR_VERSION:-0.2.1}
E2E_SPIDERDOCTOR_IMAGE_REPO=${E2E_SPIDERDOCTOR_IMAGE_REPO:-"ghcr.io"}

INSTALL_TIME_OUT=300s

SPIDERDOCTOR_HELM_OPTIONS=" --set feature.aggregateReport.enabled=true \
                            --set feature.aggregateReport.controller.reportHostPath=${SPIDERDOCTOR_REPORT_PATH}  "
case ${E2E_IP_FAMILY} in
  ipv4)
    SPIDERDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=true \
    --set feature.enableIPv6=false "
    ;;
  ipv6)
    SPIDERDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=false \
    --set feature.enableIPv6=true "
    ;;
  dual)
    SPIDERDOCTOR_HELM_OPTIONS+=" --set feature.enableIPv4=true \
    --set feature.enableIPv6=true "
    ;;
  *)
    echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
esac

SPIDERDOCTOR_HELM_OPTIONS+=" --set spiderdoctorAgent.image.registry=${E2E_SPIDERDOCTOR_IMAGE_REPO} \
 --set spiderdoctorController.image.registry=${E2E_SPIDERDOCTOR_IMAGE_REPO} "

echo "SPIDERDOCTOR_HELM_OPTIONS: ${SPIDERDOCTOR_HELM_OPTIONS}"

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

helm repo add spiderdoctor https://spidernet-io.github.io/spiderdoctor
helm repo update
HELM_IMAGES_LIST=` helm template test spiderdoctor/spiderdoctor --version ${SPIDERDOCTOR_VERSION} ${SPIDERDOCTOR_HELM_OPTIONS} | grep " image: " | tr -d '"'| awk '{print $2}' | uniq `

[ -z "${HELM_IMAGES_LIST}" ] && echo "can't found image of SPIDERDOCTOR" && exit 1
LOCAL_IMAGE_LIST=`docker images | awk '{printf("%s:%s\n",$1,$2)}'`

for IMAGE in ${HELM_IMAGES_LIST}; do
  if ! grep ${IMAGE} <<< ${LOCAL_IMAGE_LIST}; then
      echo "===> docker pull ${IMAGE}... "
      docker pull ${IMAGE}
  fi
  echo "===> load image ${IMAGE} to kind..."
  kind load docker-image ${IMAGE} --name $E2E_CLUSTER_NAME
done

# Install SPIDERDOCTOR
helm upgrade --install spiderdoctor spiderdoctor/spiderdoctor -n kube-system --debug --kubeconfig ${E2E_KUBECONFIG} ${SPIDERDOCTOR_HELM_OPTIONS} --version ${SPIDERDOCTOR_VERSION} 
kubectl wait --for=condition=ready -l app.kubernetes.io/name=spiderdoctor --timeout=100s pod -n kube-system \
--kubeconfig ${E2E_KUBECONFIG} 

echo -e "\033[35m Succeed to install spiderdoctor \033[0m"