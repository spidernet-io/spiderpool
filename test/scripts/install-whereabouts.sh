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


kind load docker-image $IMAGE_WHEREABOUTS --name ${E2E_CLUSTER_NAME}

# Install whereabouts
kubectl apply --kubeconfig ${E2E_KUBECONFIG} \
      -f ${CURRENT_DIR_PATH}/../yamls/daemonset-install.yaml \
      -f ${CURRENT_DIR_PATH}/../yamls/whereabouts.cni.cncf.io_ippools.yaml \
      -f ${CURRENT_DIR_PATH}/../yamls/whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml \
      -f ${CURRENT_DIR_PATH}/../yamls/ip-reconciler-job.yaml
