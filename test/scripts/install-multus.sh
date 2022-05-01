#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

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


echo "load $IMAGE_MULTUS to kind cluster"
kind load docker-image $IMAGE_MULTUS --name ${E2E_CLUSTER_NAME}

echo "load $TEST_IMAGE to kind cluster"
kind load docker-image $TEST_IMAGE --name ${E2E_CLUSTER_NAME}

# tmplate
IMAGE_TAG=$IMAGE_MULTUS  p2ctl -t ${CURRENT_DIR_PATH}/../yamls/multus-daemonset-thick-plugin.tmpl \
    > $CLUSTER_PATH/multus-daemonset-thick-plugin.yml

kubectl apply -f $CLUSTER_PATH/multus-daemonset-thick-plugin.yml --kubeconfig ${E2E_KUBECONFIG}

echo "$CURRENT_FILENAME : done"