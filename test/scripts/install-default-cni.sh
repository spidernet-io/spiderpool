#!/bin/bash

# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit -o nounset -o pipefail

OS=$(uname | tr 'A-Z' 'a-z')
SED_COMMAND=sed

CURRENT_FILENAME=$(basename $0)
CURRENT_DIR_PATH=$(
  cd $(dirname $0)
  pwd
)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../.. && pwd)

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

DEST_CALICO_YAML_DIR=${PROJECT_ROOT_PATH}/test/.tmp/yamls
rm -rf ${DEST_CALICO_YAML_DIR}
mkdir -p ${DEST_CALICO_YAML_DIR}

CALICO_YAML=${DEST_CALICO_YAML_DIR}/calico.yaml
CALICO_CONFIG=${DEST_CALICO_YAML_DIR}/calico_config.yaml
CALICO_NODE=${DEST_CALICO_YAML_DIR}/calico_node.yaml

export CALICO_VERSION=${CALICO_VERSION:-""}
export INSTALL_TIME_OUT=${INSTALL_TIME_OUT:-"600s"}
export CALICO_IMAGE_REPO=${CALICO_IMAGE_REPO:-"docker.io"}
export CALICO_AUTODETECTION_METHOD=${CALICO_AUTODETECTION_METHOD:-"kubernetes-internal-ip"}
export CALICO_IPV4POOL_CIDR=${CALICO_CLUSTER_POD_SUBNET_V4:-"10.243.64.0/18"}
export CALICO_IPV6POOL_CIDR=${CALICO_CLUSTER_POD_SUBNET_V6:-"fd00:10:243::/112"}

E2E_CILIUM_IMAGE_REPO=${E2E_CILIUM_IMAGE_REPO:-"quay.io"}
CILIUM_VERSION=${CILIUM_VERSION:-""}
DISABLE_KUBE_PROXY=${DISABLE_KUBE_PROXY:-"false"}
CILIUM_CLUSTER_POD_SUBNET_V4=${CILIUM_CLUSTER_POD_SUBNET_V4:-"10.244.64.0/18"}
CILIUM_CLUSTER_POD_SUBNET_V6=${CILIUM_CLUSTER_POD_SUBNET_V6:-"fd00:10:244::/112"}

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

function install_calico() {
  cp ${PROJECT_ROOT_PATH}/test/yamls/calico.yaml $CLUSTER_PATH/calico.yaml
  if [ -z "${CALICO_VERSION}" ]; then
    [ -n "${HTTP_PROXY}" ] && {
      CALICO_VERSION_INFO=$(curl --retry 3 --retry-delay 5 -x "${HTTP_PROXY}" -s https://api.github.com/repos/projectcalico/calico/releases/latest)
      echo ${CALICO_VERSION_INFO}
      CALICO_VERSION=$(echo ${CALICO_VERSION_INFO} | jq -r '.tag_name')
    }
    [ -z "${HTTP_PROXY}" ] && {
      CALICO_VERSION_INFO=$(curl --retry 3 --retry-delay 5 -s https://api.github.com/repos/projectcalico/calico/releases/latest)
      echo ${CALICO_VERSION_INFO}
      CALICO_VERSION=$(echo ${CALICO_VERSION_INFO} | jq -r '.tag_name')
    }
    [ "${CALICO_VERSION}" == "null" ] && {
      echo "failed to get the calico version, will try to use default version."
      CALICO_VERSION=${DEFAULT_CALICO_VERSION}
    }
  else
    CALICO_VERSION=${CALICO_VERSION}
  fi
  echo "install calico version ${CALICO_VERSION}"
  [ -n "${HTTP_PROXY}" ] && curl --retry 3 -x "${HTTP_PROXY}" -Lo ${CALICO_YAML} https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/calico.yaml
  [ -z "${HTTP_PROXY}" ] && curl --retry 3 -Lo ${CALICO_YAML} https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/calico.yaml

  # set registry
  if [ -n "${CALICO_IMAGE_REPO}" ]; then
    grep -q -e ".*image:.*docker.io" ${CALICO_YAML} || {
      echo "failed find image"
      exit 1
    }
    ${SED_COMMAND} -i -E 's?(.*image:.*)(docker.io)(.*)?\1'"${CALICO_IMAGE_REPO}"'\3?g' ${CALICO_YAML}
  fi

  # accelerate local cluster , in case that it times out to wait calico ready
  IMAGE_LIST=$(cat ${CALICO_YAML} | grep "image: " | awk '{print $2}' | sort | uniq | tr '\n' ' ' | tr '\r' ' ')
  echo "image: ${IMAGE_LIST}"
  for IMAGE in ${IMAGE_LIST}; do
    echo "load calico image ${IMAGE} to kind cluster"
    docker pull ${IMAGE}
    kind load docker-image ${IMAGE} --name ${E2E_CLUSTER_NAME}
  done

  export KUBECONFIG=${E2E_KUBECONFIG}
  kubectl apply -f ${CALICO_YAML}
  sleep 10

  kubectl wait --for=condition=ready -l k8s-app=calico-node --timeout=${INSTALL_TIME_OUT} pod -n kube-system
  kubectl get po -n kube-system
  echo -e "\033[35m Succeed to install Calico \033[0m"

  echo -e "\033[35m Patch Calico \033[0m"
  kubectl -n kube-system get cm calico-config -oyaml >${CALICO_CONFIG}
  kubectl -n kube-system get ds calico-node -oyaml >${CALICO_NODE}

  case ${E2E_IP_FAMILY} in
  ipv4)
    # set configmap
    configYaml=$(yq '.data.cni_network_config' ${CALICO_CONFIG} | yq '.plugins[0].ipam = {"type": "calico-ipam", "assign_ipv4": "true", "assign_ipv6": "false"}' --output-format=json)
    configYaml=$configYaml yq e '.data.cni_network_config |= strenv(configYaml)' -i ${CALICO_CONFIG}
    ${SED_COMMAND} -i 's/"mtu": "__CNI_MTU__"/"mtu": __CNI_MTU__/g' ${CALICO_CONFIG}
    kubectl -n kube-system patch cm calico-config --patch "$(cat ${CALICO_CONFIG})" || {
      echo "failed to patch calico configmap"
      exit 1
    }
    ;;
  ipv6)
    # set configmap
    configYaml=$(yq '.data.cni_network_config' ${CALICO_CONFIG} | yq '.plugins[0].ipam = {"type": "calico-ipam", "assign_ipv4": "false", "assign_ipv6": "true"}' --output-format=json)
    configYaml=$configYaml yq e '.data.cni_network_config |= strenv(configYaml)' -i ${CALICO_CONFIG}
    ${SED_COMMAND} -i 's/"mtu": "__CNI_MTU__"/"mtu": __CNI_MTU__/g' ${CALICO_CONFIG}
    kubectl -n kube-system patch cm calico-config --patch "$(cat ${CALICO_CONFIG})" || {
      echo "failed to patch calico configmap"
      exit 1
    }

    # set calico-node env
    grep -q "FELIX_IPV6SUPPORT" ${CALICO_NODE} || {
      echo "failed find FELIX_IPV6SUPPORT"
      exit 1
    }
    ${SED_COMMAND} -i -E '/FELIX_IPV6SUPPORT/{n;s/value: "false"/value: "true"/}' ${CALICO_NODE}

    grep -q "value: autodetect" ${CALICO_NODE} || {
      echo "failed find autodetect"
      exit 1
    }
    ${SED_COMMAND} -i '/value: autodetect/a\        - name: IP6\n\          value: autodetect' ${CALICO_NODE}
    kubectl -n kube-system patch ds calico-node --patch "$(cat ${CALICO_NODE})" || {
      echo "failed to patch calico-node"
      exit 1
    }
    ;;
  dual)
    # set configmap
    configYaml=$(yq '.data.cni_network_config' ${CALICO_CONFIG} | yq '.plugins[0].ipam = {"type": "calico-ipam", "assign_ipv4": "true", "assign_ipv6": "true"}' --output-format=json)
    configYaml=$configYaml yq e '.data.cni_network_config |= strenv(configYaml)' -i ${CALICO_CONFIG}
    ${SED_COMMAND} -i 's/"mtu": "__CNI_MTU__"/"mtu": __CNI_MTU__/g' ${CALICO_CONFIG}
    kubectl -n kube-system patch cm calico-config --patch "$(cat ${CALICO_CONFIG})" || {
      echo "failed to patch calico configmap"
      exit 1
    }

    # set calico-node env
    grep -q "FELIX_IPV6SUPPORT" ${CALICO_NODE} || {
      echo "failed find FELIX_IPV6SUPPORT"
      exit 1
    }
    ${SED_COMMAND} -i -E '/FELIX_IPV6SUPPORT/{n;s/value: "false"/value: "true"/}' ${CALICO_NODE}
    grep -q "value: autodetect" ${CALICO_NODE} || {
      echo "failed find autodetect"
      exit 1
    }
    ${SED_COMMAND} -i '/value: autodetect/a\        - name: IP6\n\          value: autodetect' ${CALICO_NODE}
    kubectl -n kube-system patch ds calico-node --patch "$(cat ${CALICO_NODE})" || {
      echo "failed to patch calico-node"
      exit 1
    }
    ;;
  *)
    echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
    ;;
  esac
  # there no default felixconfigurations.crd.projectcalico.org in latest calico version (https://github.com/projectcalico/calico/releases/tag/v3.29.0)
  kubectl patch felixconfigurations.crd.projectcalico.org default --type='merge' -p '{"spec":{"chainInsertMode":"Append"}}' || true

  # restart calico pod
  kubectl -n kube-system delete pod -l k8s-app=calico-node --force --grace-period=0 && sleep 3
  kubectl wait --for=condition=ready -l k8s-app=calico-node --timeout=${INSTALL_TIME_OUT} pod -n kube-system
  kubectl -n kube-system delete pod -l k8s-app=calico-kube-controllers --force --grace-period=0 && sleep 3
  kubectl wait --for=condition=ready -l k8s-app=calico-kube-controllers --timeout=${INSTALL_TIME_OUT} pod -n kube-system
  echo -e "\033[35m ===> Succeed to patch calico \033[0m"

  # Update calico's podcidr so that it is inconsistent with the cluster's podcidr.
  case ${E2E_IP_FAMILY} in
  ipv4)
    kubectl patch ippools default-ipv4-ippool --patch '{"spec": {"cidr": "'"${CALICO_IPV4POOL_CIDR}"'"}}' --type=merge
    ;;
  ipv6)
    kubectl delete ippools default-ipv4-ippool --force
    kubectl patch ippools default-ipv6-ippool --patch '{"spec": {"cidr": "'"${CALICO_IPV6POOL_CIDR}"'"}}' --type=merge
    ;;
  dual)
    kubectl patch ippools default-ipv4-ippool --patch '{"spec": {"cidr": "'"${CALICO_IPV4POOL_CIDR}"'"}}' --type=merge
    kubectl patch ippools default-ipv6-ippool --patch '{"spec": {"cidr": "'"${CALICO_IPV6POOL_CIDR}"'"}}' --type=merge
    ;;
  *)
    echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
    ;;
  esac

  echo -e "\033[35m ===> clean tmp \033[0m"
  rm -rf ${DEST_CALICO_YAML_DIR}
}

function install_cilium() {
  echo -e "\033[35m ===> Start to install cilium \033[0m"
  # cni.exclusive using multus-cni need close
  # kubeProxyReplacement Enhance kube-proxy (value probe static default: probe)
  # k8sServiceHost api-server address
  # k8sServicePort api-service port
  # bpf.vlanBypass allow vlan traffic to pass
  # cilium ipamMode: multi-pool required routingMode=native and kubeProxyReplacement
  CILIUM_HELM_OPTIONS=" --set cni.exclusive=false \
    --set k8sServiceHost=${E2E_CLUSTER_NAME}-control-plane \
    --set k8sServicePort=6443 \
    --set bpf.vlanBypass={0} "
  if [ "$DISABLE_KUBE_PROXY" = "true" ]; then
    CILIUM_HELM_OPTIONS+=" --set kubeProxyReplacement=true \
        --set routingMode=native \
        --set ipam.mode=multi-pool \
        --set nodeinit.enabled=true \
        --set autoDirectNodeRoutes=true \
        --set bpf.masquerade=true \
        --set endpointRoutes.enabled=true\
    "
  fi
  case ${E2E_IP_FAMILY} in
  ipv4)
    CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv4PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V4} \
                                   --set ipv4.enabled=true \
                                   --set ipv6.enabled=false "
    if [ "$DISABLE_KUBE_PROXY" = "true" ]; then
      # run for multi-pool mode
      CILIUM_HELM_OPTIONS+=" --set ipv4NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V4} \
      --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.cidrs=${CILIUM_CLUSTER_POD_SUBNET_V4} \
      --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.maskSize=26 \
      --set enableIPv4Masquerade=true "
    fi
    ;;
  ipv6)
    CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv6PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V6} \
    --set ipv4.enabled=false \
    --set ipv6.enabled=true \
    --set routingMode=native \
    --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv6.cidrs=${CILIUM_CLUSTER_POD_SUBNET_V6}  \
    --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv6.maskSize=124 \
    --set ipv6NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V6} \
    --set autoDirectNodeRoutes=true \
    --set enableIPv6Masquerade=true  "
    ;;
  dual)
    CILIUM_HELM_OPTIONS+=" --set ipam.operator.clusterPoolIPv4PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V4} \
      --set ipam.operator.clusterPoolIPv6PodCIDRList=${CILIUM_CLUSTER_POD_SUBNET_V6} \
      --set ipv4.enabled=true \
      --set ipv6.enabled=true "
    if [ "$DISABLE_KUBE_PROXY" = "true" ]; then
      # run for multi-pool mode
      CILIUM_HELM_OPTIONS+=" --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.cidrs=${CILIUM_CLUSTER_POD_SUBNET_V4} \
      --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv4.maskSize=26 \
      --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv6.cidrs=${CILIUM_CLUSTER_POD_SUBNET_V6} \
      --set ipam.operator.autoCreateCiliumPodIPPools.default.ipv6.maskSize=124 \
      --set ipv6NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V6} \
      --set enableIPv4Masquerade=true \
      --set enableIPv6Masquerade=true \
      --set ipv4NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V4} \
      --set ipv6NativeRoutingCIDR=${CILIUM_CLUSTER_POD_SUBNET_V6} "
    fi
    ;;
  *)
    echo "the value of E2E_IP_FAMILY: ipv4 or ipv6 or dual"
    exit 1
    ;;
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

  if [ -n "${CILIUM_VERSION}" ]; then
    CILIUM_HELM_OPTIONS+=" --version ${CILIUM_VERSION} "
  fi

  HELM_IMAGES_LIST=$(helm template test cilium/cilium ${CILIUM_HELM_OPTIONS} | grep " image: " | tr -d '"' | awk '{print $2}' | awk -F "@" '{print $1}' | uniq)
  [ -z "${HELM_IMAGES_LIST}" ] && echo "can't found image of cilium" && exit 1
  LOCAL_IMAGE_LIST=$(docker images | awk '{printf("%s:%s\n",$1,$2)}')

  for CILIUM_IMAGE in ${HELM_IMAGES_LIST}; do
    if ! grep ${CILIUM_IMAGE} <<<${LOCAL_IMAGE_LIST}; then
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
  kubectl wait --for=condition=ready -l app.kubernetes.io/part-of=cilium --timeout=${INSTALL_TIME_OUT} pod -n kube-system \
    --kubeconfig ${E2E_KUBECONFIG} || {
    kubectl get pod -n kube-system -l app.kubernetes.io/part-of=cilium --kubeconfig ${E2E_KUBECONFIG}
    kubectl describe pod -n kube-system -l app.kubernetes.io/part-of=cilium --kubeconfig ${E2E_KUBECONFIG}
    kubectl get po -n kube-system -l app.kubernetes.io/part-of=cilium --kubeconfig ${E2E_KUBECONFIG} --no-headers | grep CrashLoopBackOff | awk '{print $1}' | xargs -I {} kubectl --kubeconfig ${E2E_KUBECONFIG} logs -n kube-system {}
    exit 1
  }

  sleep 10

  echo -e "\033[35m ===> Succeed to install cilium \033[0m"
}

if [ "${INSTALL_CALICO}" == "true" ]; then
  install_calico
fi

if [ "${INSTALL_CILIUM}" == "true" ]; then
  install_cilium
fi

kubectl get po -n kube-system --kubeconfig ${E2E_KUBECONFIG} -owide
