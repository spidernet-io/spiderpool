#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

set -o errexit
set -o nounset
set -o pipefail

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

E2E_CLUSTER_NAME="$1"
E2E_KUBECONFIG="$2"

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

[ -z "$IMAGE_MULTUS" ] && echo "error, miss IMAGE_MULTUS" && exit 1
echo "$CURRENT_FILENAME : IMAGE_MULTUS $IMAGE_MULTUS "

[ -z "$CLUSTER_PATH" ] && echo "error, miss CLUSTER_PATH" && exit 1
echo "$CURRENT_FILENAME : CLUSTER_PATH $CLUSTER_PATH "

[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY" && exit 1
echo "$CURRENT_FILENAME : E2E_IP_FAMILY $E2E_IP_FAMILY "

[ -z "$MULTUS_DEFAULT_CNI_CALICO" ] && echo "error, miss MULTUS_DEFAULT_CNI_CALICO" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_CALICO $MULTUS_DEFAULT_CNI_CALICO "

[ -z "$MULTUS_DEFAULT_CNI_NAME" ] && echo "error, miss MULTUS_DEFAULT_CNI_NAME" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_NAME $MULTUS_DEFAULT_CNI_NAME "

[ -z "$MULTUS_DEFAULT_CNI_VLAN100" ] && echo "error, miss MULTUS_DEFAULT_CNI_VLAN100" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN100 $MULTUS_DEFAULT_CNI_VLAN100 "

[ -z "$MULTUS_DEFAULT_CNI_VLAN200" ] && echo "error, miss MULTUS_DEFAULT_CNI_VLAN200" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN200 $MULTUS_DEFAULT_CNI_VLAN200 "

[ -z "$MULTUS_ADDITIONAL_CNI_VLAN100" ] && echo "error, miss MULTUS_ADDITIONAL_CNI_VLAN100" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN100 $MULTUS_ADDITIONAL_CNI_VLAN100 "

[ -z "$MULTUS_ADDITIONAL_CNI_VLAN200" ] && echo "error, miss MULTUS_ADDITIONAL_CNI_VLAN200" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN200 $MULTUS_ADDITIONAL_CNI_VLAN200 "

#==============
OS=$(uname | tr 'A-Z' 'a-z')
SED_COMMAND=sed
if [ ${OS} == "darwin" ]; then SED_COMMAND=gsed ; fi

echo "load $IMAGE_MULTUS to kind cluster"
kind load docker-image $IMAGE_MULTUS --name ${E2E_CLUSTER_NAME}

# tmplate
${SED_COMMAND}  's?<<IMAGE_MULTUS>>?'"${IMAGE_MULTUS}"'?'   ${CURRENT_DIR_PATH}/../yamls/multus-daemonset-thick-plugin.tmpl > ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
${SED_COMMAND} -i 's?<<MULTUS_DEFAULT_CNI_NAME>>?'"${MULTUS_DEFAULT_CNI_NAME}"'?' ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml

kubectl apply -f ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml --kubeconfig ${E2E_KUBECONFIG}
# for CRD is applied
sleep 5

echo "waiting for daemonset/kube-multus-ds ready"
kubectl rollout status --kubeconfig ${E2E_KUBECONFIG} -n kube-system  daemonset/kube-multus-ds  -w --timeout=60s

Install::MultusCR(){

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_CALICO}
    namespace: ${MULTUS_CNI_NAMESPACE}
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_NAME}
    namespace: ${MULTUS_CNI_NAMESPACE}
  spec:
    config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-vlan0-underlay",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool"
                  }
              },{
                  "type": "coordinator",
                  "tune_mode": "underlay",
                  "detect_gateway": false
              }
          ]
      }
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_VLAN100}
    namespace: ${MULTUS_CNI_NAMESPACE}
  spec:
    config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-vlan100-underlay",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0.100",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool"
                  }
              },{
                  "type": "coordinator",
                  "tune_mode": "underlay",
                  "tune_pod_routes": false,
                  "detect_gateway": false
              }
          ]
      }
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_ADDITIONAL_CNI_VLAN100}
    namespace: ${MULTUS_CNI_NAMESPACE}
  spec:
    config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-vlan100-overlay",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0.100",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool"
                  }
              },{
                  "type": "coordinator",
                  "tune_mode": "overlay",
                  "detect_gateway": false,
                  "tune_pod_routes": false
              }
          ]
      }
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_ADDITIONAL_CNI_VLAN200}
    namespace: ${MULTUS_CNI_NAMESPACE}
  spec:
    config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-vlan200-overlay",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0.200",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool"
                  }
              },{
                  "type": "coordinator",
                  "tune_mode": "overlay",
                  "tune_pod_routes": false
              }
          ]
      }
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_VLAN200}
    namespace: ${MULTUS_CNI_NAMESPACE}
  spec:
    config: |-
      {
          "cniVersion": "0.3.1",
          "name": "macvlan-vlan200-underlay",
          "plugins": [
              {
                  "type": "macvlan",
                  "master": "eth0.200",
                  "mode": "bridge",
                  "ipam": {
                      "type": "spiderpool"
                  }
              },{
                  "type": "coordinator",
                  "tune_mode": "underlay"
              }
          ]
      }
EOF
}


Install::SpiderpoolCR(){
  SPIDERPOOL_VLAN100_POOL_V4=172.100.0.0/16
  SPIDERPOOL_VLAN100_POOL_V6=fd00:172:100::/64
  SPIDERPOOL_VLAN100_RANGES_V4=172.100.0.201-172.100.10.199
  SPIDERPOOL_VLAN100_RANGES_V6=fd00:172:100::201-fd00:172:100::fff1
  SPIDERPOOL_VLAN100_GATEWAY_V4=172.100.0.1
  SPIDERPOOL_VLAN100_GATEWAY_V6=fd00:172:100::1
  SPIDERPOOL_VLAN200_POOL_V4=172.200.0.0/16
  SPIDERPOOL_VLAN200_POOL_V6=fd00:172:200::/64
  SPIDERPOOL_VLAN200_RANGES_V4=172.200.0.201-172.200.10.199
  SPIDERPOOL_VLAN200_RANGES_V6=fd00:172:200::201-fd00:172:200::fff1
  SPIDERPOOL_VLAN200_GATEWAY_V4=172.200.0.1
  SPIDERPOOL_VLAN200_GATEWAY_V6=fd00:172:200::1

  case ${E2E_IP_FAMILY} in
    ipv4)
    cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: vlan100-v4
    spec:
      gateway: ${SPIDERPOOL_VLAN100_GATEWAY_V4}
      vlan: 100
      ipVersion: 4
      ips:
      - ${SPIDERPOOL_VLAN100_RANGES_V4}
      subnet: ${SPIDERPOOL_VLAN100_POOL_V4}
EOF

    cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
    apiVersion: spiderpool.spidernet.io/v2beta1
    kind: SpiderSubnet
    metadata:
      name: vlan200-v4
    spec:
      gateway: ${SPIDERPOOL_VLAN200_GATEWAY_V4}
      vlan: 200
      ipVersion: 4
      ips:
      - ${SPIDERPOOL_VLAN200_RANGES_V4}
      subnet: ${SPIDERPOOL_VLAN200_POOL_V4}
EOF
      ;;
    ipv6)
  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan100-v6
  spec:
    gateway: ${SPIDERPOOL_VLAN100_GATEWAY_V6}
    ipVersion: 6
    ips:
    - ${SPIDERPOOL_VLAN100_RANGES_V6}
    subnet: ${SPIDERPOOL_VLAN100_POOL_V6}
    vlan: 100
EOF

  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan200-v6
  spec:
    gateway: ${SPIDERPOOL_VLAN200_GATEWAY_V6}
    ipVersion: 6
    ips:
    - ${SPIDERPOOL_VLAN200_RANGES_V6}
    subnet: ${SPIDERPOOL_VLAN200_POOL_V6}
    vlan: 200
EOF
      ;;
    dual)
  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan100-v4
  spec:
    gateway: ${SPIDERPOOL_VLAN100_GATEWAY_V4}
    vlan: 100
    ipVersion: 4
    ips:
    - ${SPIDERPOOL_VLAN100_RANGES_V4}
    subnet: ${SPIDERPOOL_VLAN100_POOL_V4}
EOF

  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan200-v4
  spec:
    gateway: ${SPIDERPOOL_VLAN200_GATEWAY_V4}
    vlan: 200
    ipVersion: 4
    ips:
    - ${SPIDERPOOL_VLAN200_RANGES_V4}
    subnet: ${SPIDERPOOL_VLAN200_POOL_V4}
EOF

  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan100-v6
  spec:
    gateway: ${SPIDERPOOL_VLAN100_GATEWAY_V6}
    ipVersion: 6
    ips:
    - ${SPIDERPOOL_VLAN100_RANGES_V6}
    subnet: ${SPIDERPOOL_VLAN100_POOL_V6}
    vlan: 100
EOF

  cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
  apiVersion: spiderpool.spidernet.io/v2beta1
  kind: SpiderSubnet
  metadata:
    name: vlan200-v6
  spec:
    gateway: ${SPIDERPOOL_VLAN200_GATEWAY_V6}
    ipVersion: 6
    ips:
    - ${SPIDERPOOL_VLAN200_RANGES_V6}
    subnet: ${SPIDERPOOL_VLAN200_POOL_V6}
    vlan: 200
EOF
      ;;
    *)
      echo "the value of IP_FAMILY: ipv4 or ipv6 or dual"
      exit 1
  esac
}

Install::MultusCR

if [ ${E2E_SPIDERPOOL_ENABLE_SUBNET} == "true" ]; then
  Install::SpiderpoolCR
fi

echo "$CURRENT_FILENAME : done"