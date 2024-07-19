# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

function usage()
{
    echo -e "--install-cni          enable install cni-plugins"
    echo -e "--install-ovs          enable install ovs-plugin"
    echo -e "--install-ib-sriov     enable install ib-sriov"
    echo -e "--install-ipoib        enable install ipoib"
    echo -e "--install-rdma         enable install rdma-plugin"
    echo -e "--copy-dst-dir         specifies the path to these plugins installed"
}

COPY_DST_DIR="/host/opt/cni/bin"

echo "OVS_BIN_PATH=${OVS_BIN_PATH}"
echo "IB_SRIOV_BIN_PATH=${IB_SRIOV_BIN_PATH}"
echo "CNI_BIN_DIR=${CNI_BIN_DIR}"
echo "IPOIB_BIN_PATH=${IPOIB_BIN_PATH}"
echo "VERSION_FILE_PATH=${VERSION_FILE_PATH}"
echo "RDMA_BIN_PATH=${RDMA_BIN_PATH}"

[ -f "${RDMA_BIN_PATH}" ] || { echo "error, failed to find ${RDMA_BIN_PATH}" ; exit 1 ; }
[ -f "${OVS_BIN_PATH}" ] || { echo "error, failed to find ${OVS_BIN_PATH}" ; exit 1 ; }
[ -f "${IB_SRIOV_BIN_PATH}" ] || { echo "error, failed to find ${IB_SRIOV_BIN_PATH}" ; exit 1 ; }
[ -f "${IPOIB_BIN_PATH}" ] || { echo "error, failed to find ${IPOIB_BIN_PATH}" ; exit 1 ; }
[ -f "${VERSION_FILE_PATH}" ] || { echo "error, failed to find ${VERSION_FILE_PATH}" ; exit 1 ; }
[ -d "${CNI_BIN_DIR}" ] || { echo "error, failed to find ${CNI_BIN_DIR}" ; exit 1 ; }

source ${VERSION_FILE_PATH}

INSTALL_OVS_PLUGIN=${INSTALL_OVS_PLUGIN:-false}
INSTALL_RDMA_PLUGIN=${INSTALL_RDMA_PLUGIN:-false}
INSTALL_IB_SRIOV_PLUGIN=${INSTALL_IB_SRIOV_PLUGIN:-false}
INSTALL_IPOIB_PLUGIN=${INSTALL_IPOIB_PLUGIN:-false}
INSTALL_CNI_PLUGINS=${INSTALL_CNI_PLUGINS:-false}

mkdir -p ${COPY_DST_DIR} || true

if [ "$INSTALL_CNI_PLUGINS" = "true" ]; then
    echo "Installing CNI-Plugins: ${CNI_VERSION}"
    for plugin in "${CNI_BIN_DIR}"/*; do
      ITEM=${plugin##*/}
      rm -f ${COPY_DST_DIR}/${ITEM}.old || true
      ( [ -f "${COPY_DST_DIR}/${ITEM}" ] && mv ${COPY_DST_DIR}/${ITEM} ${COPY_DST_DIR}/${ITEM}.old ) || true
      cp ${plugin} ${COPY_DST_DIR}
      rm -f ${COPY_DST_DIR}/${ITEM}.old &>/dev/null  || true
    done
else
    echo "skip installing cni plugins"
fi

if [ "$INSTALL_OVS_PLUGIN" = "true" ]; then
   echo "Installing OVS-Plugin: ${OVS_VERSION}"
   rm -f ${COPY_DST_DIR}/ovs.old || true
   ( [ -f "${COPY_DST_DIR}/ovs" ] && mv ${COPY_DST_DIR}/ovs ${COPY_DST_DIR}/ovs.old ) || true
   cp ${OVS_BIN_PATH} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/ovs.old &>/dev/null  || true
else
    echo "skip installing ovs"
fi

if [ "$INSTALL_RDMA_PLUGIN" = "true" ]; then
   echo "Installing RDMA-Plugin: ${RDMA_VERSION}"
   rm -f ${COPY_DST_DIR}/rdma.old || true
   ( [ -f "${COPY_DST_DIR}/rdma" ] && mv ${COPY_DST_DIR}/rdma ${COPY_DST_DIR}/rdma.old ) || true
   cp ${RDMA_BIN_PATH} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/rdma.old &>/dev/null  || true
else
    echo "skip installing rdma"
fi

if [ "$INSTALL_IB_SRIOV_PLUGIN" = "true" ]; then
   echo "Installing ib-sriov: ${IB_SRIOV_VERSION}"
   rm -f ${COPY_DST_DIR}/ib-sriov.old || true
   ( [ -f "${COPY_DST_DIR}/ib-sriov" ] && mv ${COPY_DST_DIR}/ib-sriov ${COPY_DST_DIR}/ib-sriov.old ) || true
   cp ${IB_SRIOV_BIN_PATH} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/ib-sriov.old &>/dev/null  || true
else
    echo "skip installing ib-sriov"
fi

if [ "$INSTALL_IPOIB_PLUGIN" = "true" ]; then
   echo "Installing ipoib: ${IPOIB_VERSION}"
   rm -f ${COPY_DST_DIR}/ipoib.old || true
   ( [ -f "${COPY_DST_DIR}/ipoib" ] && mv ${COPY_DST_DIR}/ipoib ${COPY_DST_DIR}/ipoib.old ) || true
   cp ${IPOIB_BIN_PATH} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/ipoib.old &>/dev/null  || true
else
    echo "skip installing ipoib"
fi

echo Done.
