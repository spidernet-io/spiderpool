#!/bin/bash

# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

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

[ -z "$INSTALL_CALICO" ] && echo "error, miss INSTALL_CALICO " && exit 1
echo "$CURRENT_FILENAME : INSTALL_CALICO $INSTALL_CALICO "

[ -z "$INSTALL_CILIUM" ] && echo "error, miss INSTALL_CILIUM " && exit 1
echo "$CURRENT_FILENAME : INSTALL_CILIUM $INSTALL_CILIUM "

[ -z "$CLUSTER_PATH" ] && echo "error, miss CLUSTER_PATH" && exit 1
echo "$CURRENT_FILENAME : CLUSTER_PATH $CLUSTER_PATH "

export CALICO_VERSION=${CALICO_VERSION:-"v3.25.0"}
export INSTALL_TIME_OUT=${INSTALL_TIME_OUT:-"600s"}
export CALICO_IMAGE_REPO=${CALICO_IMAGE_REPO:-"docker.io"}
export CALICO_AUTODETECTION_METHOD=${CALICO_AUTODETECTION_METHOD:-"kubernetes-internal-ip"}

E2E_CILIUM_IMAGE_REPO=${E2E_CILIUM_IMAGE_REPO:-"quay.io"}
CILIUM_VERSION=${CILIUM_VERSION:-""}
CILIUM_CLUSTER_POD_SUBNET_V4=${CILIUM_CLUSTER_POD_SUBNET_V4:-"10.244.64.0/18"}
CILIUM_CLUSTER_POD_SUBNET_V6=${CILIUM_CLUSTER_POD_SUBNET_V6:-"fd00:10:244::/112"}

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

function install_calico() {
    cp ${PROJECT_ROOT_PATH}/test/yamls/calico.yaml $CLUSTER_PATH/calico.yaml

    case ${E2E_IP_FAMILY} in
      ipv4)
          export CALICO_CNI_ASSIGN_IPV4=true
          export CALICO_CNI_ASSIGN_IPV6=false
          export CALICO_IP_AUTODETECT=autodetect
          export CALICO_IP6_AUTODETECT=none
          export CALICO_FELIX_IPV6SUPPORT=false
          export CALICO_IPV6POOL_VXLAN=Never
        ;;
      ipv6)
          export CALICO_CNI_ASSIGN_IPV4=false
          export CALICO_CNI_ASSIGN_IPV6=true
          export CALICO_IP_AUTODETECT=none
          export CALICO_IP6_AUTODETECT=autodetect
          export CALICO_FELIX_IPV6SUPPORT=true
          export CALICO_IPV6POOL_VXLAN=Always
        ;;
      dual)
          export CALICO_CNI_ASSIGN_IPV4=true
          export CALICO_CNI_ASSIGN_IPV6=true
          export CALICO_IP_AUTODETECT=autodetect
          export CALICO_IP6_AUTODETECT=autodetect
          export CALICO_FELIX_IPV6SUPPORT=true
          export CALICO_IPV6POOL_VXLAN=Always
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

    echo -e "\033[35m ===> Succeed to install calico \033[0m"
}

function install_cilium() {
      echo -e "\033[35m ===> Start to install cilium \033[0m"
      # cni.exclusive using multus-cni need close
      # kubeProxyReplacement Enhance kube-proxy (value probe static default: probe)
      # k8sServiceHost api-server address
      # k8sServicePort api-service port
      # bpf.vlanBypass allow vlan traffic to pass
      CILIUM_HELM_OPTIONS=" --set cni.exclusive=false \
                            --set kubeProxyReplacement=disabled \
                            --set k8sServiceHost=${E2E_CLUSTER_NAME}-control-plane \
                            --set k8sServicePort=6443 \
                            --set bpf.vlanBypass=0 "
      case ${E2E_IP_FAMILY} in
        ipv4)
            CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv4PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V4} \
                                   --set ipv4.enabled=true \
                                   --set ipv6.enabled=false "
          ;;
        ipv6)
            CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv6PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V6} \
                                   --set ipv4.enabled=false \
                                   --set ipv6.enabled=true \
                                   --set tunnel=disabled \
                                   --set ipv6NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V6} \
                                   --set autoDirectNodeRoutes=true \
                                   --set enableIPv6Masquerade=true \
                                   --set routingMode=native "
          ;;
        dual)
            CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv4PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V4} \
                                   --set ipam.operator.clusterPoolIPv6PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V6} \
                                   --set ipv4.enabled=true \
                                   --set ipv6.enabled=true "
          ;;
        *)
          echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
          exit 1
      esac

    CILIUM_HELM_OPTIONS+=" \
      --set image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/cilium \
      --set image.useDigest=false \
      --set certgen.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/certgen \
      --set hubble.relay.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/hubble-relay \
      --set hubble.relay.image.useDigest=false \
      --set hubble.ui.backend.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/hubble-ui-backend \
      --set hubble.ui.frontend.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/hubble-ui \
      --set etcd.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/cilium-etcd-operator \
      --set operator.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/operator  \
      --set operator.image.useDigest=false  \
      --set preflight.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/cilium \
      --set preflight.image.useDigest=false \
      --set nodeinit.image.repository=${E2E_CILIUM_IMAGE_REPO}/cilium/startup-script "

    echo "CILIUM_HELM_OPTIONS: ${CILIUM_HELM_OPTIONS}"

    helm repo remove cilium &>/dev/null || true
    helm repo add cilium https://helm.cilium.io
    helm repo update

    if [ -n "${CILIUM_VERSION}" ] ; then
        CILIUM_HELM_OPTIONS+=" --version ${CILIUM_VERSION} "
    fi

    HELM_IMAGES_LIST=` helm template test cilium/cilium ${CILIUM_HELM_OPTIONS} | grep " image: " | tr -d '"'| awk '{print $2}' | awk -F "@" '{print $1}' | uniq `
    [ -z "${HELM_IMAGES_LIST}" ] && echo "can't found image of cilium" && exit 1
    LOCAL_IMAGE_LIST=`docker images | awk '{printf("%s:%s\n",$1,$2)}'`

    for CILIUM_IMAGE in ${HELM_IMAGES_LIST}; do
      if ! grep ${CILIUM_IMAGE} <<< ${LOCAL_IMAGE_LIST} ; then
        echo "===> docker pull ${CILIUM_IMAGE} "
        docker pull ${CILIUM_IMAGE}
      fi
      echo "===> load image ${CILIUM_IMAGE} to kind..."
      kind load docker-image ${CILIUM_IMAGE} --name ${E2E_CLUSTER_NAME}
    done

    # Install cilium
    helm upgrade --install cilium cilium/cilium --wait -n kube-system --debug --kubeconfig ${E2E_KUBECONFIG} ${CILIUM_HELM_OPTIONS}

    # no matching resources found
    sleep 3
    kubectl wait --for=condition=ready -l k8s-app=cilium --timeout=${INSTALL_TIME_OUT} pod -n kube-system \
    --kubeconfig ${E2E_KUBECONFIG}

    sleep 10

    echo -e "\033[35m ===> Succeed to install cilium \033[0m"
}

if [ "${INSTALL_CALICO}" == "true" ] ; then
  install_calico
fi

if [ "${INSTALL_CILIUM}" == "true" ] ; then
  install_cilium
fi


kubectl get po -n kube-system --kubeconfig ${E2E_KUBECONFIG} -owide
