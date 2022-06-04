#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net



E2E_CLUSTER_NAME="$1"
[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1

ISSUER_NAME="$2"
[ -z "$ISSUER_NAME" ] && echo "error, miss ISSUER_NAME " && exit 1

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

# cert-manager-v1.8.0
# this yaml is modified to be hostnetwork, and added lots tolerations, to make sure it starts before IPAM
CERT_MANAGER_CHART_PATH=${CURRENT_DIR_PATH}/../yamls/cert-manager.yaml
[ -f "$CERT_MANAGER_CHART_PATH" ] || { echo "error, find to find directory $CERT_MANAGER_CHART_PATH" ; exit 1 ; }


[ -z "$IMAGE_CERT_MANAGER" ] && echo "error, miss IMAGE_CERT_MANAGER" && exit 1
echo "$CURRENT_FILENAME : IMAGE_CERT_MANAGER $IMAGE_CERT_MANAGER "

echo "load images to kind cluster"
for IMAGE in $IMAGE_CERT_MANAGER ; do
    kind load docker-image $IMAGE --name ${E2E_CLUSTER_NAME}
done

for (( CNT=0 ; CNT<3 ; CNT++ )) ; do
  kubectl apply -f ${CERT_MANAGER_CHART_PATH} && break
  sleep 5
done

# must be ready and apply Issuer
NAMESPACE=kube-system
for (( CNT=0 ; CNT<3 ; CNT++ )) ; do
sleep 10
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ${ISSUER_NAME}
  namespace: ${NAMESPACE}
spec:
  selfSigned: {}
EOF
RESULT=$?
(( $RESULT == 0 )) && exit 0
done

exit 1
