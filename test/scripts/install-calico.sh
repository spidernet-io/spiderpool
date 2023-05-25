# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

set -o errexit -o nounset -o pipefail

OS=$(uname | tr 'A-Z' 'a-z')
SED_COMMAND=sed

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$( cd ${CURRENT_DIR_PATH}/../.. && pwd )

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY " && exit 1

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1

[ -z "$CLUSTER_PATH" ] && echo "error, miss CLUSTER_PATH" && exit 1
echo "$CURRENT_FILENAME : CLUSTER_PATH $CLUSTER_PATH "

export CALICO_VERSION=${CALICO_VERSION:-"v3.25.0"}
export INSTALL_TIME_OUT=${INSTALL_TIME_OUT:-"600s"}
export CALICO_IMAGE_REPO=${CALICO_IMAGE_REPO:-"docker.io"}
export CALICO_AUTODETECTION_METHOD=${CALICO_AUTODETECTION_METHOD:-"kubernetes-internal-ip"}

function install_calico() {
    cp ${PROJECT_ROOT_PATH}/test/yamls/calico.yaml $CLUSTER_PATH/calico.yaml

    case ${E2E_IP_FAMILY} in
      ipv4)
          export CALICO_CNI_ASSIGN_IPV4=true
          export CALICO_CNI_ASSIGN_IPV6=false
          export CALICO_IP_AUTODETECT=autodetect
          export CALICO_IP6_AUTODETECT=autodetect
          export CALICO_FELIX_IPV6SUPPORT=false
          export CALICO_IPV6POOL_VXLAN=Never
        ;;
      ipv6)
          export CALICO_CNI_ASSIGN_IPV4=false
          export CALICO_CNI_ASSIGN_IPV6=true
          export CALICO_IP_AUTODETECT=autodetect
          export CALICO_IP6_AUTODETECT=autodetect
          export CALICO_FELIX_IPV6SUPPORT=true
          export CALICO_IPV6POOL_VXLAN=Never
        ;;
      dual)
          export CALICO_CNI_ASSIGN_IPV4=true
          export CALICO_CNI_ASSIGN_IPV6=true
          export CALICO_IP_AUTODETECT=autodetect
          export CALICO_IP6_AUTODETECT=autodetect
          export CALICO_FELIX_IPV6SUPPORT=true
          export CALICO_IPV6POOL_VXLAN=Never
        ;;
      *)
        echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
        exit 1
    esac

    if [ ${OS} == "darwin" ]; then SED_COMMAND=gsed ; fi

    ENV_LIST=`env | egrep "^CALICO_" `
    for env in ${ENV_LIST}; do
        KEY="${env%%=*}"
        VALUE="${env#*=}"
        echo $KEY $VALUE
        ${SED_COMMAND} -i "s/<<${KEY}>>/${VALUE}/g" ${CLUSTER_PATH}/calico.yaml
    done


    CALICO_IMAGE_LIST=`cat ${CLUSTER_PATH}/calico.yaml | grep 'image: ' | tr -d '"' | awk '{print $2}'`
    [ -z "${CALICO_IMAGE_LIST}" ] && echo "can't found image of calico" && exit 1
    LOCAL_IMAGE_LIST=`docker images | awk '{printf("%s:%s\n",$1,$2)}'`

    for CALICO_IMAGE in ${CALICO_IMAGE_LIST}; do
      if ! grep ${CALICO_IMAGE} <<< ${LOCAL_IMAGE_LIST} ; then
        echo "===> docker pull ${CALICO_IMAGE} "
        docker pull ${CALICO_IMAGE}
      fi
      echo "===> load image ${CALICO_IMAGE} to kind..."
      kind load docker-image ${CALICO_IMAGE} --name ${E2E_CLUSTER_NAME}
    done

    kubectl apply -f  ${CLUSTER_PATH}/calico.yaml --kubeconfig ${E2E_KUBECONFIG}

    sleep 5

    kubectl wait --for=condition=ready -l k8s-app=calico-node --timeout=${INSTALL_TIME_OUT} pod -n kube-system --kubeconfig ${E2E_KUBECONFIG}

    echo -e "\033[35m Succeed to install calico \033[0m"
}

install_calico

kubectl get po -n kube-system --kubeconfig ${E2E_KUBECONFIG} -owide