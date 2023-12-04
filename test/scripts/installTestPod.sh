#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

E2E_KUBECONFIG="$1"

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$TEST_IMAGE_NAME" ] && echo "error, miss TEST_IMAGE_NAME" && exit 1
echo "$CURRENT_FILENAME : TEST_IMAGE_NAME $TEST_IMAGE_NAME "

echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

docker pull ${TEST_IMAGE_NAME}
kind load docker-image ${TEST_IMAGE_NAME} --name $E2E_CLUSTER_NAME

NAME=test-pod
cat <<EOF | kubectl apply --kubeconfig ${E2E_KUBECONFIG} -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${NAME}
  namespace: default
  labels:
    app: $NAME
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $NAME
  template:
    metadata:
      annotations:
        $(if [[ "${INSTALL_OVERLAY_CNI}" == "true" ]];then
        echo "k8s.v1.cni.cncf.io/networks: ${RELEASE_NAMESPACE}/${MULTUS_DEFAULT_CNI_NAME}"
        fi)
      name: $NAME
      labels:
        app: $NAME
    spec:
      containers:
      - name: $NAME
        image: ${TEST_IMAGE_NAME}
        imagePullPolicy: IfNotPresent
        command:
        - "/bin/sh"
        args:
        - "-c"
        - "sleep infinity"
EOF

echo "waiting for deployment/${NAME} ready"
if ! kubectl rollout status  deployment/${NAME} --kubeconfig ${E2E_KUBECONFIG} -w --timeout=120s ; then
    kubectl describe po --kubeconfig ${E2E_KUBECONFIG}
    echo "error, failed to create a test pod"
    exit 1
fi
echo "succeed to create a test pod"
# kubectl delete deployment/${NAME}
exit 0
