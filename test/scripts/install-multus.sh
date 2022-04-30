#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider


kind load docker-image $IMAGE_MULTUS --name $1
kind load docker-image $TEST_IMAGE --name $1

# tmplate
IMAGE_TAG=$IMAGE_MULTUS p2ctl -t $(pwd)/yamls/multus-daemonset-thick-plugin.tmpl > $CLUSTER_PATH/multus-daemonset-thick-plugin.yml
kubectl apply -f $CLUSTER_PATH/multus-daemonset-thick-plugin.yml --kubeconfig $2