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

[ -z "$CLUSTER_PATH" ] && echo "error, miss CLUSTER_PATH" && exit 1
echo "$CURRENT_FILENAME : CLUSTER_PATH $CLUSTER_PATH "

[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY" && exit 1
echo "$CURRENT_FILENAME : E2E_IP_FAMILY $E2E_IP_FAMILY "

[ -z "$MULTUS_DEFAULT_CNI_CALICO" ] && echo "error, miss MULTUS_DEFAULT_CNI_CALICO" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_CALICO $MULTUS_DEFAULT_CNI_CALICO "

[ -z "$MULTUS_DEFAULT_CNI_CILIUM" ] && echo "error, miss MULTUS_DEFAULT_CNI_CILIUM" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_CILIUM $MULTUS_DEFAULT_CNI_CILIUM "

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

Install::MultusCR(){

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_CALICO}
    namespace: ${RELEASE_NAMESPACE}
EOF

  cat << EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
  apiVersion: k8s.cni.cncf.io/v1
  kind: NetworkAttachmentDefinition
  metadata:
    name: ${MULTUS_DEFAULT_CNI_CILIUM}
    namespace: ${RELEASE_NAMESPACE}
EOF

MACVLAN_CR_TEMPLATE='
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: <<CNI_NAME>>
  namespace: <<NAMESPACE>>
spec:
  config: |-
    {
        "cniVersion": "0.3.1",
        "name": "<<CNI_NAME>>",
        "plugins": [
            {
                "type": "macvlan",
                "master": "<<MASTER>>",
                "mode": "bridge",
                "ipam": {
                    "type": "spiderpool"
                }
            },{
                "type": "coordinator",
                "tuneMode": "<<MODE>>"
            }
        ]
    }
'

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_NAME}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<MODE>>?underlay?g' \
    | sed 's?<<MASTER>>?eth0?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_VLAN100}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<MODE>>?underlay?g' \
    | sed 's?<<MASTER>>?eth0.100?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -


  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_ADDITIONAL_CNI_VLAN100}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<MODE>>?overlay?g' \
    | sed 's?<<MASTER>>?eth0.100?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_ADDITIONAL_CNI_VLAN200}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<MODE>>?overlay?g' \
    | sed 's?<<MASTER>>?eth0.200?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_VLAN200}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<MODE>>?underlay?g' \
    | sed 's?<<MASTER>>?eth0.200?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

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

    if [ "${E2E_SPIDERPOOL_ENABLE_SUBNET}" == "true" ] ; then
        CR_KIND="SpiderSubnet"
        echo "spiderpool subnet feature is on , install SpiderSubnet CR"
    else
        CR_KIND="SpiderIPPool"
        echo "spiderpool subnet feature is off , install SpiderIPPool CR"
    fi

    INSTALL_V4_CR(){
        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: ${CR_KIND}
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
        kind: ${CR_KIND}
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
    }

    INSTALL_V6_CR(){
        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: ${CR_KIND}
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
        kind: ${CR_KIND}
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
    }


  case ${E2E_IP_FAMILY} in
    ipv4)
      INSTALL_V4_CR
      ;;

    ipv6)
      INSTALL_V6_CR
      ;;

    dual)
      INSTALL_V4_CR
      INSTALL_V6_CR
      ;;

    *)
      echo "the value of IP_FAMILY: ipv4 or ipv6 or dual"
      exit 1
  esac
}

Install::MultusCR
Install::SpiderpoolCR


echo "$CURRENT_FILENAME : done"