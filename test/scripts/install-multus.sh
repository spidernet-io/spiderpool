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

[ -z "$TEST_IMAGE" ] && echo "error, miss TEST_IMAGE" && exit 1
echo "$CURRENT_FILENAME : TEST_IMAGE $TEST_IMAGE "

[ -z "$CLUSTER_PATH" ] && echo "error, miss CLUSTER_PATH" && exit 1
echo "$CURRENT_FILENAME : CLUSTER_PATH $CLUSTER_PATH "


[ -z "$INSTALL_SPIDER" ] && echo "error, miss INSTALL_SPIDER" && exit 1
echo "$CURRENT_FILENAME : INSTALL_SPIDER $INSTALL_SPIDER "

[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY" && exit 1
echo "$CURRENT_FILENAME : E2E_IP_FAMILY $E2E_IP_FAMILY "

echo "$CURRENT_FILENAME : INSTALL_BRIDGE_CNI_FOR_SERVICE $INSTALL_BRIDGE_CNI_FOR_SERVICE "


MULTUS_DEFAULT_CNI_NAME=${MULTUS_DEFAULT_CNI_NAME:-"macvlan-cni-default"}
MULTUS_ADDITIONAL_CNI_NAME=${MULTUS_ADDITIONAL_CNI_NAME:-"macvlan-cni2"}
MULTUS_CNI_NAMESPACE=${MULTUS_CNI_NAMESPACE:-"kube-system"}

#==============

echo "load $IMAGE_MULTUS to kind cluster"
kind load docker-image $IMAGE_MULTUS --name ${E2E_CLUSTER_NAME}

echo "load $TEST_IMAGE to kind cluster"
kind load docker-image $TEST_IMAGE --name ${E2E_CLUSTER_NAME}



#==============

InstallCNI::Spiderpool(){
    echo "install $MULTUS_DEFAULT_CNI_NAME  cni : macvlan + spiderpool"

    if [ "${E2E_IP_FAMILY}" == "ipv4" ] ; then
        DEFAULT_IPPOOL='"default_ipv4_ippool": ["default-v4-ippool"],'
    elif [ "${E2E_IP_FAMILY}" == "ipv6" ] ; then
        DEFAULT_IPPOOL='"default_ipv6_ippool": ["default-v6-ippool"],'
    else
        DEFAULT_IPPOOL='"default_ipv4_ippool": ["default-v4-ippool"],
            "default_ipv6_ippool": ["default-v6-ippool"],'
    fi

    cat <<EOF | kubectl  create --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: ${MULTUS_DEFAULT_CNI_NAME}
  namespace: ${MULTUS_CNI_NAMESPACE}
spec:
  config: '{
      "cniVersion": "0.3.1",
      "type": "macvlan",
      "mode": "bridge",
      "master": "eth0",
      "name": "${MULTUS_DEFAULT_CNI_NAME}",
      "ipam": {
          ${DEFAULT_IPPOOL}
          "type": "spiderpool",
          "log_level" : "DEBUG",
          "log_file_path" : "/var/log/spidernet/spiderpool.log",
          "log_file_max_size" : 100,
          "log_file_max_age": 30,
          "log_file_max_count": 10
       }
    }'
EOF

    cat <<EOF | kubectl   create --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: ${MULTUS_ADDITIONAL_CNI_NAME}
  namespace: ${MULTUS_CNI_NAMESPACE}
spec:
  config: '{
      "cniVersion": "0.3.1",
      "type": "macvlan",
      "mode": "bridge",
      "master": "eth0",
      "name": "${MULTUS_ADDITIONAL_CNI_NAME}",
      "ipam": {
          ${DEFAULT_IPPOOL}
          "type": "spiderpool",
          "log_level" : "DEBUG",
          "log_file_path" : "/var/log/spidernet/spiderpool.log",
          "log_file_max_size" : 100,
          "log_file_max_age": 30,
          "log_file_max_count": 10
       }
    }'
EOF

}

InstallCNI::Whereabout(){
    echo "install $MULTUS_DEFAULT_CNI_NAME  cni : macvlan + whereabout"

if [ "${E2E_IP_FAMILY}" == "ipv4" ] ; then
    DEFAULT_CNI_CONF='"range": "172.19.1.10-172.19.1.254/16",
           "gateway": "172.19.0.1",
           "routes": [ { "dst": "0.0.0.0/0" }],'

    ADD_CNI_CONF='"range": "172.20.1.10-172.20.1.254/16",
           "gateway": "172.20.0.1",
           "routes": [ { "dst": "0.0.0.0/0" }],'

elif [ "${E2E_IP_FAMILY}" == "ipv6" ] ; then
    DEFAULT_CNI_CONF='"range": "fc00::/64",
            "exclude": [ "fc00::1/128" ],
            "gateway": "fc00::1",
            "routes": [{ "dst": "::/0" }],'

    ADD_CNI_CONF='"range": "fc01::/64",
            "exclude": [ "fc01::1/128" ],
            "gateway": "fc01::1",
            "routes": [{ "dst": "::/0" }],'
else
    DEFAULT_CNI_CONF='"range": "172.19.1.10-172.19.1.254/16",
           "gateway": "172.19.0.1",
           "addresses": [
              {
                "address": "fc00:f853::100/64",
                "gateway": "fc00:f853::1"
              }],
           "routes": [ { "dst": "0.0.0.0/0" }],'

    ADD_CNI_CONF='"range": "172.20.1.10-172.20.1.254/16",
           "gateway": "172.20.0.1",
           "addresses": [
              {
                "address": "fc01:f853::100/64",
                "gateway": "fc01:f853::1"
              }],
           "routes": [ { "dst": "0.0.0.0/0" }],'
fi

    cat <<EOF | kubectl   create --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: ${MULTUS_DEFAULT_CNI_NAME}
  namespace: ${MULTUS_CNI_NAMESPACE}
spec:
  config: '{
      "cniVersion": "0.3.1",
      "type": "macvlan",
      "mode": "bridge",
      "master": "eth0",
      "name": "${MULTUS_DEFAULT_CNI_NAME}",
      "ipam": {
           ${DEFAULT_CNI_CONF}
           "type": "whereabouts",
           "log_level": "debug",
           "log_file": "/var/log/whereabout.log"
       }
    }'
EOF

    cat <<EOF | kubectl   create --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: ${MULTUS_ADDITIONAL_CNI_NAME}
  namespace: ${MULTUS_CNI_NAMESPACE}
spec:
  config: '{
      "cniVersion": "0.3.1",
      "type": "macvlan",
      "mode": "bridge",
      "master": "eth0",
      "name": "${MULTUS_ADDITIONAL_CNI_NAME}",
      "ipam": {
           ${ADD_CNI_CONF}
           "type": "whereabouts",
           "log_level": "debug",
           "log_file": "/var/log/whereabout.log"
       }
    }'
EOF

}


E2E_BRIDGE_CNI="bridge-pod-network"
# install bridge cni to forward service traffic
InstallCNI::Bridge(){

  if [ "$INSTALL_BRIDGE_CNI_FOR_SERVICE" != "true" ] ; then
      echo "no bridge cni"
      return
  fi

  echo "install bridge cni $E2E_BRIDGE_CNI , cidr $E2E_BRIDGE_V4_CIDR $E2E_BRIDGE_V6_CIDR , K8S_IPV4_SERVICE_CIDR=${K8S_IPV4_SERVICE_CIDR}, K8S_IPV6_SERVICE_CIDR=${K8S_IPV6_SERVICE_CIDR}"

if [ "${E2E_IP_FAMILY}" == "ipv4" ] ; then
    RANGES="\"subnet\": \"${E2E_BRIDGE_V4_CIDR}\""
    ROUTES="\"routes\": [{ \"dst\": \"${K8S_IPV4_SERVICE_CIDR}\", \"gw\": \"${E2E_BRIDGE_V4_GW}\" }]"

elif [ "${E2E_IP_FAMILY}" == "ipv6" ] ; then
    RANGES="\"subnet\": \"${E2E_BRIDGE_V6_CIDR}\""
    ROUTES="\"routes\": [{ \"dst\": \"${K8S_IPV6_SERVICE_CIDR}\", \"gw\": \"${E2E_BRIDGE_V6_GW}\" }]"

else
    RANGES="\"ranges\": [ [{ \"subnet\": \"${E2E_BRIDGE_V4_CIDR}\" }], [{ \"subnet\": \"${E2E_BRIDGE_V6_CIDR}\" }] ]"
    ROUTES="\"routes\": [{ \"dst\": \"${K8S_IPV4_SERVICE_CIDR}\", \"gw\": \"${E2E_BRIDGE_V4_GW}\" }, { \"dst\": \"${K8S_IPV6_SERVICE_CIDR}\", \"gw\": \"${E2E_BRIDGE_V6_GW}\" }]"

fi

    cat <<EOF | kubectl   create --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: ${E2E_BRIDGE_CNI}
  namespace: ${MULTUS_CNI_NAMESPACE}
spec:
  config: |
    {
      "cniVersion": "0.3.1",
      "type": "bridge",
      "name": "${E2E_BRIDGE_CNI}",
      "bridge": "bridge0",
      "isDefaultGateway": false,
      "forceAddress": false,
      "ipMasq": true,
      "isGateway": true,
      "ipam": {
         "type": "host-local",
         ${RANGES},
         ${ROUTES}
      }
    }
EOF

}


#==============

# tmplate
sed 's?<<IMAGE_MULTUS>>?'"${IMAGE_MULTUS}"'?'   ${CURRENT_DIR_PATH}/../yamls/multus-daemonset-thick-plugin.tmpl > ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
sed -i 's?<<MULTUS_DEFAULT_CNI_NAME>>?'"${MULTUS_DEFAULT_CNI_NAME}"'?' ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
sed -i 's?<<MULTUS_CNI_NAMESPACE>>?'"${MULTUS_CNI_NAMESPACE}"'?' ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
if [ "$INSTALL_BRIDGE_CNI_FOR_SERVICE" == "true" ] ; then
    sed -i 's?<<E2E_BRIDGE_CNI>>?'"${E2E_BRIDGE_CNI}"'?' ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
else
    sed -i 's?<<E2E_BRIDGE_CNI>>??' ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml
fi

kubectl apply -f ${CLUSTER_PATH}/multus-daemonset-thick-plugin.yml --kubeconfig ${E2E_KUBECONFIG}
# for CRD is applied
sleep 5

InstallCNI::Bridge
if [ "$INSTALL_SPIDER"x == "true"x ] ; then
    InstallCNI::Spiderpool
else
    InstallCNI::Whereabout
fi

echo "waiting for daemonset/kube-multus-ds ready"
kubectl rollout status --kubeconfig ${E2E_KUBECONFIG} -n kube-system  daemonset/kube-multus-ds  -w --timeout=60s

echo "$CURRENT_FILENAME : done"
