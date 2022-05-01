#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

# Copy 10-macvlan.tmpl to kind-node

set -o errexit
set -o nounset
set -o pipefail

E2E_CLUSTER_NAME="$1"

DOWNLOAD_DIR="$2"
[ ! -d "$DOWNLOAD_DIR" ] && echo "error, does not exist $DOWNLOAD_DIR " && exit 1

CNI_TAR_NAME=$( cd $DOWNLOAD_DIR && ls cni-plugins-linux-* )
[ -z "$CNI_TAR_NAME" ] && echo "error, failed to find cni package under $DOWNLOAD_DIR " && exit 2
CNI_TAR_PATH="${DOWNLOAD_DIR}/${CNI_TAR_NAME}"
[ ! -f  "$CNI_TAR_PATH" ] &&  echo "error, failed to find cni path $CNI_TAR_PATH " && exit 3
echo "find $CNI_TAR_PATH "

NODES=$(docker ps | grep -E "kindest/node.* ${E2E_CLUSTER_NAME}-(control|worker)" | awk '{print $1}')
for node in ${NODES} ; do
  echo "install cni to node ${node} "
  docker cp ${CNI_TAR_PATH} $node:/root/
  docker exec $node mkdir -p /host/opt/cni/bin
  docker exec $node tar xvfzp /root/${CNI_TAR_NAME} -C /opt/cni/bin
done
