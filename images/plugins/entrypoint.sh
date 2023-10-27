# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

#!/bin/bash

set -e

function usage()
{
    echo -e "--install-cni enable install cni-plugins"
    echo -e "--install-ovs enable install ovs-plugin"
    echo -e "--install-rdma enable install rdma-plugin"
    echo -e "--copy-dst-dir specifies the path to these plugins installed"
}

COPY_DST_DIR="/host/opt/cni/bin"
CNI_SRC_DIR="/usr/src/cni/bin"
OVS_SRC_DIR="/usr/src/cni/ovs-cni/ovs"
RDMA_SRC_DIR="/usr/src/cni/rdma-cni/rdma"
INSTALL_CNI_PLUGINS=${INSTALL_CNI_PLUGINS:-false}
INSTALL_OVS_PLUGIN=${INSTALL_OVS_PLUGIN:-false}
INSTALL_RDMA_PLUGIN=${INSTALL_RDMA_PLUGIN:-false}

# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM=`echo $1 | awk -F= '{print $1}'`
    VALUE=`echo $1 | awk -F= '{print $2}'`
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
        --install-cni)
            INSTALL_CNI_PLUGINS=$VALUE
            ;;
        --install-ovs)
            INSTALL_OVS_PLUGIN=$VALUE
            ;;
        --install-rdma)
            INSTALL_RDMA_PLUGIN=$VALUE
            ;;
        --copy-dst-dir)
            COPY_DST_DIR=$VALUE
            ;;
        *)
            warn "unknown parameter \"$PARAM\""
            ;;
    esac
    shift
done

if [ -f "${SRC_DIR}" ]; then
  echo "source plugins dir: ${SRC_DIR} not exist"
  exit 1
fi

mkdir -p ${COPY_DST_DIR}

if [ "$INSTALL_CNI_PLUGINS" = "true" ]; then
    CNI_VERSION=$(cat VERSION.info | grep CNI_VERSION | awk '{print $2}')
    echo "Installing CNI-Plugins: ${CNI_VERSION} ..."
    for plugin in "${CNI_SRC_DIR}"/*; do
      ITEM=${plugin##*/}
      rm -f ${COPY_DST_DIR}/${ITEM}.old || true
      ( [ -f "${COPY_DST_DIR}/${ITEM}" ] && mv ${COPY_DST_DIR}/${ITEM} ${COPY_DST_DIR}/${ITEM}.old ) || true
      cp ${plugin} ${COPY_DST_DIR}
      rm -f ${COPY_DST_DIR}/${ITEM}.old &>/dev/null  || true
    done
fi

if [ "$INSTALL_OVS_PLUGIN" = "true" ]; then
   OVS_VERSION=$(cat VERSION.info | grep OVS_VERSION | awk '{print $2}')
   echo "Installing OVS-Plugin: ${OVS_VERSION} ..."
   rm -f ${COPY_DST_DIR}/ovs.old || true
   ( [ -f "${COPY_DST_DIR}/ovs" ] && mv ${COPY_DST_DIR}/ovs ${COPY_DST_DIR}/ovs.old ) || true
   cp ${OVS_SRC_DIR} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/ovs.old &>/dev/null  || true
fi

if [ "$INSTALL_RDMA_PLUGIN" = "true" ]; then
   RDMA_COMMIT_HASH=$(cat VERSION.info | grep RDMA_COMMIT_HASH | awk '{print $2}')
   echo "Installing RDMA-Plugin: ${RDMA_COMMIT_HASH} ..."
   rm -f ${COPY_DST_DIR}/rdma.old || true
   ( [ -f "${COPY_DST_DIR}/rdma" ] && mv ${COPY_DST_DIR}/rdma ${COPY_DST_DIR}/rdma.old ) || true
   cp ${RDMA_SRC_DIR} ${COPY_DST_DIR}
   rm -f ${COPY_DST_DIR}/rdma.old &>/dev/null  || true
fi

echo Done.
