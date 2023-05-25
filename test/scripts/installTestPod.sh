#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

E2E_KUBECONFIG="$1"
E2E_MULTUS_ENABLED="$2"

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$TEST_IMAGE" ] && echo "error, miss TEST_IMAGE" && exit 1
echo "$CURRENT_FILENAME : TEST_IMAGE $TEST_IMAGE "

echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

docker pull ${TEST_IMAGE}
kind load docker-image ${TEST_IMAGE} --name $E2E_CLUSTER_NAME

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
        $(if [[ "${E2E_MULTUS_ENABLED}" == "true" ]];then
        echo "v1.multus-cni.io/default-network: ${MULTUS_CNI_NAMESPACE}/${MULTUS_DEFAULT_CNI_NAME}"
        fi)
      name: $NAME
      labels:
        app: $NAME
    spec:
      containers:
      - name: $NAME
        image: ${TEST_IMAGE}
        imagePullPolicy: IfNotPresent
        command:
        - "/bin/sh"
        args:
        - "-c"
        - "sleep infinity"
EOF

echo "waiting for deployment/${NAME} ready"
if ! kubectl rollout status  deployment/${NAME} --kubeconfig ${E2E_KUBECONFIG} -w --timeout=120s ; then
    echo "error, failed to create a test pod"
    exit 1
fi
echo "succeed to create a test pod"
# kubectl delete deployment/${NAME}
exit 0
