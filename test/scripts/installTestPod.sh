#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

E2E_KUBECONFIG="$1"

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "


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
        k8s.v1.cni.cncf.io/networks: ${MULTUS_CNI_NAMESPACE}/${MULTUS_ADDITIONAL_CNI_NAME}
      name: $NAME
      labels:
        app: $NAME
    spec:
      containers:
      - name: $NAME
        image: ${TEST_IMAGE}
        imagePullPolicy: IfNotPresent
        command:
        - "/bin/ash"
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
