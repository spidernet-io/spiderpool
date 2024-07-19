#!/bin/bash

# Copyright 2024 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$( dirname $0 )
CURRENT_DIR_PATH=$(cd ${CURRENT_DIR_PATH} ; pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../.. ; pwd)

OUTPUT_FILE_PATH=${1:-"${PROJECT_ROOT_PATH}/images/spiderpool-plugins/version.sh"}

echo "output shell path: ${OUTPUT_FILE_PATH} "

set -x

if [ -n "${http_proxy}" ]; then
  CURL_OPITON=" -x ${http_proxy} "
fi

echo "checking the version of cni plugin "
VERSION=$( curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json"  https://api.github.com/repos/containernetworking/plugins/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of cni: ${VERSION}"
  export CNI_VERSION=${VERSION}
else
  echo "error, failed to latest version of cni"
  exit 1
fi

echo "checking the version of ovs plugin "
VERSION=$(curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json" https://api.github.com/repos/k8snetworkplumbingwg/ovs-cni/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of ovs: ${VERSION}"
  export OVS_VERSION=${VERSION}
else
  echo "error, failed to latest version of ovs"
  exit 1
fi

echo "checking the version of rdma plugin "
VERSION=$(curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json" https://api.github.com/repos/k8snetworkplumbingwg/rdma-cni/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of rdma: ${VERSION}"
  export RDMA_VERSION=${VERSION}
else
  echo "error, failed to latest version of rdma"
  exit 1
fi

echo "checking the version of sriov plugin "
VERSION=$(curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json" https://api.github.com/repos/k8snetworkplumbingwg/sriov-cni/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of sriov: ${VERSION}"
  export SRIOV_VERSION=${VERSION}
else
  echo "error, failed to latest version of sriov"
  exit 1
fi

echo "checking the version of ib-sriov plugin "
VERSION=$(curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json" https://api.github.com/repos/k8snetworkplumbingwg/ib-sriov-cni/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of ib-sriov: ${VERSION}"
  export IB_SRIOV_VERSION=${VERSION}
else
  echo "error, failed to latest version of ib-sriov"
  exit 1
fi

echo "checking the version of ipoib plugin "
VERSION=$(curl ${CURL_OPITON} --retry 10  -H "Accept: application/vnd.github+json" https://api.github.com/repos/Mellanox/ipoib-cni/releases/latest | jq '.tag_name' | tr -d '"')
if [ -n "${VERSION}" ] ; then
  echo "latest version of ipoib: ${VERSION}"
  export IPOIB_VERSION=${VERSION}
else
  echo "error, failed to latest version of ipoib"
  exit 1
fi

echo "-------- generate version script--------"
cat <<EOF > ${OUTPUT_FILE_PATH}
#!/bin/bash
# this file is generated by ${CURRENT_FILENAME} , please do not edit

# https://github.com/containernetworking/plugins
export CNI_VERSION=\${CNI_VERSION:-"${CNI_VERSION}"}
# https://github.com/k8snetworkplumbingwg/ovs-cni
export OVS_VERSION=\${OVS_VERSION:-"${OVS_VERSION}"}
# https://github.com/k8snetworkplumbingwg/rdma-cni
export RDMA_VERSION=\${RDMA_VERSION:-"${RDMA_VERSION}"}
# https://github.com/k8snetworkplumbingwg/sriov-cni
export SRIOV_VERSION=\${SRIOV_VERSION:-"${SRIOV_VERSION}"}
# https://github.com/k8snetworkplumbingwg/ib-sriov-cni
export IB_SRIOV_VERSION=\${IB_SRIOV_VERSION:-"${IB_SRIOV_VERSION}"}
# https://github.com/Mellanox/ipoib-cni
export IPOIB_VERSION=\${IPOIB_VERSION:-"${IPOIB_VERSION}"}
EOF

