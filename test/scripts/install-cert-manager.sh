#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

set -o errexit
set -o nounset
set -o pipefail

E2E_CLUSTER_NAME="$1"
[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

# cert-manager-v1.8.0
CERT_MANAGER_CHART_PATH=$( cd ${CURRENT_DIR_PATH}/../yamls/cert-manager && pwd )
[ -d "$CERT_MANAGER_CHART_PATH" ] || { echo "error, find to find directory $CERT_MANAGER_CHART_PATH" ; exit 1 ; }


[ -z "$IMAGE_CERT_MANAGER" ] && echo "error, miss IMAGE_CERT_MANAGER" && exit 1
echo "$CURRENT_FILENAME : IMAGE_CERT_MANAGER $IMAGE_CERT_MANAGER "

echo "load images to kind cluster"
for IMAGE in $IMAGE_CERT_MANAGER ; do
    kind load docker-image $IMAGE --name ${E2E_CLUSTER_NAME}
done

helm install cert-manager ${CERT_MANAGER_CHART_PATH} --namespace kube-system --set installCRDs=true
