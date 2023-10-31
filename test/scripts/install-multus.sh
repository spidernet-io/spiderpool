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

[ -z "$MULTUS_DEFAULT_CNI_NAME" ] && echo "error, miss MULTUS_DEFAULT_CNI_NAME" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_NAME $MULTUS_DEFAULT_CNI_NAME "

[ -z "$MULTUS_DEFAULT_CNI_VLAN100" ] && echo "error, miss MULTUS_DEFAULT_CNI_VLAN100" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN100 $MULTUS_DEFAULT_CNI_VLAN100 "

[ -z "$MULTUS_DEFAULT_CNI_VLAN200" ] && echo "error, miss MULTUS_DEFAULT_CNI_VLAN200" && exit 1
echo "$CURRENT_FILENAME : MULTUS_DEFAULT_CNI_VLAN200 $MULTUS_DEFAULT_CNI_VLAN200 "

[ -z "$MULTUS_KUBEVIRT_CNI_VLAN30" ] && echo "error, miss MULTUS_KUBEVIRT_CNI_VLAN30" && exit 1
echo "$CURRENT_FILENAME : MULTUS_KUBEVIRT_CNI_VLAN30 $MULTUS_KUBEVIRT_CNI_VLAN30 "

[ -z "$MULTUS_KUBEVIRT_CNI_VLAN40" ] && echo "error, miss MULTUS_KUBEVIRT_CNI_VLAN40" && exit 1
echo "$CURRENT_FILENAME : MULTUS_KUBEVIRT_CNI_VLAN40 $MULTUS_KUBEVIRT_CNI_VLAN40 "

#==============
OS=$(uname | tr 'A-Z' 'a-z')
SED_COMMAND=sed
if [ ${OS} == "darwin" ]; then SED_COMMAND=gsed ; fi

Install::MultusCR(){

MACVLAN_CR_TEMPLATE='
apiVersion: spiderpool.spidernet.io/v2beta1
kind: SpiderMultusConfig
metadata:
  name: <<CNI_NAME>>
  namespace: <<NAMESPACE>>
spec:
  cniType: macvlan
  enableCoordinator: <<ENABLE_COORDINATOR>>
  macvlan:
    master: ["<<MASTER>>"]
    vlanID: <<VLAN>>
    ippools:
      ipv4: [<<DEFAULT_IPV4_IPPOOLS>>]
      ipv6: [<<DEFAULT_IPV6_IPPOOLS>>]
  coordinator:
    mode: "<<MODE>>"
'

  case ${E2E_IP_FAMILY} in
    ipv4)
      DEFAULT_IPV4_IPPOOLS=\"default-v4-ippool\"
      DEFAULT_IPV6_IPPOOLS=""
      VLAN100_IPV4_IPPOOLS=vlan100-v4
      VLAN100_IPV6_IPPOOLS=""
      VLAN200_IPV4_IPPOOLS=vlan200-v4
      VLAN200_IPV6_IPPOOLS=""
      KUBEVIRT_VLAN30_IPV4_IPPOOLS=kubevirt-vlan30-v4
      KUBEVIRT_VLAN30_IPV6_IPPOOLS=""
      KUBEVIRT_VLAN40_IPV4_IPPOOLS=kubevirt-vlan40-v4
      KUBEVIRT_VLAN40_IPV6_IPPOOLS=""
      ;;

    ipv6)
      DEFAULT_IPV4_IPPOOLS=''
      DEFAULT_IPV6_IPPOOLS=\"default-v6-ippool\"
      VLAN100_IPV4_IPPOOLS=''
      VLAN100_IPV6_IPPOOLS=vlan100-v6
      VLAN200_IPV4_IPPOOLS=''
      VLAN200_IPV6_IPPOOLS=vlan200-v6
      KUBEVIRT_VLAN30_IPV4_IPPOOLS=''
      KUBEVIRT_VLAN30_IPV6_IPPOOLS=kubevirt-vlan30-v6
      KUBEVIRT_VLAN40_IPV4_IPPOOLS=''
      KUBEVIRT_VLAN40_IPV6_IPPOOLS=kubevirt-vlan30-v6
      ;;

    dual)
      DEFAULT_IPV4_IPPOOLS=\"default-v4-ippool\"
      DEFAULT_IPV6_IPPOOLS=\"default-v6-ippool\"
      VLAN100_IPV4_IPPOOLS=vlan100-v4
      VLAN100_IPV6_IPPOOLS=vlan100-v6
      VLAN200_IPV4_IPPOOLS=vlan200-v4
      VLAN200_IPV6_IPPOOLS=vlan200-v6
      KUBEVIRT_VLAN30_IPV4_IPPOOLS=kubevirt-vlan30-v4
      KUBEVIRT_VLAN30_IPV6_IPPOOLS=kubevirt-vlan30-v6
      KUBEVIRT_VLAN40_IPV4_IPPOOLS=kubevirt-vlan40-v4
      KUBEVIRT_VLAN40_IPV6_IPPOOLS=kubevirt-vlan40-v6
      ;;

    *)
      echo "the value of IP_FAMILY: ipv4 or ipv6 or dual"
      exit 1
  esac

  ENABLE_COORDINATOR=true
  if [ "${DISABLE_KUBE_PROXY}" == "true" ] ; then
      ENABLE_COORDINATOR=false
      echo "DISABLE_KUBE_PROXY is true , disable coordinator config"
  fi
  kubectl delete spidermultusconfig ${MULTUS_DEFAULT_CNI_NAME} -n ${RELEASE_NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} || true

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_NAME}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<ENABLE_COORDINATOR>>?'${ENABLE_COORDINATOR}'?g' \
    | sed 's?<<MODE>>?auto?g' \
    | sed 's?<<MASTER>>?eth0?g' \
    | sed 's?<<VLAN>>?0?g' \
    | sed 's?<<DEFAULT_IPV4_IPPOOLS>>?'""${DEFAULT_IPV4_IPPOOLS}""'?g' \
    | sed 's?<<DEFAULT_IPV6_IPPOOLS>>?'""${DEFAULT_IPV6_IPPOOLS}""'?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_VLAN100}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<ENABLE_COORDINATOR>>?'${ENABLE_COORDINATOR}'?g' \
    | sed 's?<<MODE>>?auto?g' \
    | sed 's?<<MASTER>>?eth0?g' \
    | sed 's?<<VLAN>>?100?g' \
    | sed 's?<<DEFAULT_IPV4_IPPOOLS>>?'""${VLAN100_IPV4_IPPOOLS}""'?g' \
    | sed 's?<<DEFAULT_IPV6_IPPOOLS>>?'""${VLAN100_IPV6_IPPOOLS}""'?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  echo "${MACVLAN_CR_TEMPLATE}" \
    | sed 's?<<CNI_NAME>>?'""${MULTUS_DEFAULT_CNI_VLAN200}""'?g' \
    | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
    | sed 's?<<ENABLE_COORDINATOR>>?'${ENABLE_COORDINATOR}'?g' \
    | sed 's?<<MODE>>?auto?g' \
    | sed 's?<<MASTER>>?eth0?g' \
    | sed 's?<<VLAN>>?200?g' \
    | sed 's?<<DEFAULT_IPV4_IPPOOLS>>?'""${VLAN200_IPV4_IPPOOLS}""'?g' \
    | sed 's?<<DEFAULT_IPV6_IPPOOLS>>?'""${VLAN200_IPV6_IPPOOLS}""'?g' \
    | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

  if [ ${INSTALL_KUBEVIRT} == "true" ]; then
    echo "${MACVLAN_CR_TEMPLATE}" \
      | sed 's?<<CNI_NAME>>?'""${MULTUS_KUBEVIRT_CNI_VLAN30}""'?g' \
      | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
      | sed 's?<<ENABLE_COORDINATOR>>?'${ENABLE_COORDINATOR}'?g' \
      | sed 's?<<MODE>>?auto?g' \
      | sed 's?<<MASTER>>?eth0?g' \
      | sed 's?<<VLAN>>?30?g' \
      | sed 's?<<DEFAULT_IPV4_IPPOOLS>>?'""${KUBEVIRT_VLAN30_IPV4_IPPOOLS}""'?g' \
      | sed 's?<<DEFAULT_IPV6_IPPOOLS>>?'""${KUBEVIRT_VLAN30_IPV6_IPPOOLS}""'?g' \
      | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -

    echo "${MACVLAN_CR_TEMPLATE}" \
      | sed 's?<<CNI_NAME>>?'""${MULTUS_KUBEVIRT_CNI_VLAN40}""'?g' \
      | sed 's?<<NAMESPACE>>?'"${RELEASE_NAMESPACE}"'?g' \
      | sed 's?<<ENABLE_COORDINATOR>>?'${ENABLE_COORDINATOR}'?g' \
      | sed 's?<<MODE>>?auto?g' \
      | sed 's?<<MASTER>>?eth0?g' \
      | sed 's?<<VLAN>>?40?g' \
      | sed 's?<<DEFAULT_IPV4_IPPOOLS>>?'""${KUBEVIRT_VLAN40_IPV4_IPPOOLS}""'?g' \
      | sed 's?<<DEFAULT_IPV6_IPPOOLS>>?'""${KUBEVIRT_VLAN40_IPV6_IPPOOLS}""'?g' \
      | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
    fi

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

    SPIDERPOOL_VLAN30_POOL_V4=172.30.0.0/16
    SPIDERPOOL_VLAN30_POOL_V6=fd00:172:30::/64
    SPIDERPOOL_VLAN30_RANGES_V4=172.30.0.201-172.30.10.199
    SPIDERPOOL_VLAN30_RANGES_V6=fd00:172:30::201-fd00:172:30::fff1
    SPIDERPOOL_VLAN40_POOL_V4=172.40.0.0/16
    SPIDERPOOL_VLAN40_POOL_V6=fd00:172:40::/64
    SPIDERPOOL_VLAN40_RANGES_V4=172.40.0.201-172.40.10.199
    SPIDERPOOL_VLAN40_RANGES_V6=fd00:172:40::201-fd00:172:40::fff1

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

    INSTALL_KUBEVIRT_V4_CR(){
        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: kubevirt-vlan30-v4
        spec:
          vlan: 30
          ipVersion: 4
          ips:
          - ${SPIDERPOOL_VLAN30_RANGES_V4}
          subnet: ${SPIDERPOOL_VLAN30_POOL_V4}
EOF

        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: kubevirt-vlan40-v4
        spec:
          vlan: 40
          ipVersion: 4
          ips:
          - ${SPIDERPOOL_VLAN40_RANGES_V4}
          subnet: ${SPIDERPOOL_VLAN40_POOL_V4}
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

    INSTALL_KUBEVIRT_V6_CR(){
        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: kubevirt-vlan30-v6
        spec:
          ipVersion: 6
          ips:
          - ${SPIDERPOOL_VLAN30_RANGES_V6}
          subnet: ${SPIDERPOOL_VLAN30_POOL_V6}
          vlan: 30
EOF

        cat <<EOF | kubectl --kubeconfig ${E2E_KUBECONFIG} apply -f -
        apiVersion: spiderpool.spidernet.io/v2beta1
        kind: SpiderIPPool
        metadata:
          name: kubevirt-vlan40-v6
        spec:
          ipVersion: 6
          ips:
          - ${SPIDERPOOL_VLAN40_RANGES_V6}
          subnet: ${SPIDERPOOL_VLAN40_POOL_V6}
          vlan: 40
EOF
    }


  case ${E2E_IP_FAMILY} in
    ipv4)
      INSTALL_V4_CR
      if [ ${INSTALL_KUBEVIRT} == "true" ]; then
        INSTALL_KUBEVIRT_V4_CR
      fi
      ;;

    ipv6)
      INSTALL_V6_CR
      if [ ${INSTALL_KUBEVIRT} == "true" ]; then
        INSTALL_KUBEVIRT_V6_CR
      fi
      ;;

    dual)
      INSTALL_V4_CR
      INSTALL_V6_CR
      if [ ${INSTALL_KUBEVIRT} == "true" ]; then
        INSTALL_KUBEVIRT_V4_CR;
        INSTALL_KUBEVIRT_V6_CR;
      fi
      ;;

    *)
      echo "the value of IP_FAMILY: ipv4 or ipv6 or dual"
      exit 1
  esac
}

kubectl wait --for=condition=ready -l app.kubernetes.io/name=spiderpool --timeout=100s pod -n kube-system --kubeconfig ${E2E_KUBECONFIG} || ( kubectl describe po -n kube-system  -l app.kubernetes.io/name=spiderpool  --kubeconfig ${E2E_KUBECONFIG} ; exit 1 )

Install::MultusCR
Install::SpiderpoolCR

kubectl get spidercoordinator default -o yaml --kubeconfig ${E2E_KUBECONFIG}
kubectl get sp -o wide --kubeconfig ${E2E_KUBECONFIG}
kubectl get spidermultusconfig -n kube-system --kubeconfig ${E2E_KUBECONFIG}
kubectl get network-attachment-definitions.k8s.cni.cncf.io --kubeconfig ${E2E_KUBECONFIG} -n kube-system -o yaml

echo "$CURRENT_FILENAME : done"