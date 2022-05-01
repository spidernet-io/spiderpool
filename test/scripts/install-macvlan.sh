#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

set -o errexit
set -o nounset
set -o pipefail

E2E_CLUSTER_NAME="$1"
CNI_CONF_PATH="$2"

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
[ -z "$CNI_CONF_PATH" ] && echo "error, miss CNI_CONF_PATH " && exit 1
[ ! -f "$CNI_CONF_PATH" ] && echo "error, could not find file $CNI_CONF_PATH " && exit 1

echo ""

# Copy 10-macvlan.conflist to kind-node
NODES=$(docker ps | grep -E "kindest/node.* ${E2E_CLUSTER_NAME}-(control|worker)" | awk '{print $1}')
for node in ${NODES} ; do
  echo "docker cp ${CNI_CONF_PATH} $node:/etc/cni/net.d"
  docker cp ${CNI_CONF_PATH} $node:/etc/cni/net.d
done
