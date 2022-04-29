#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

IMAGE_NAME=ghcr.io/k8snetworkplumbingwg/multus-cni:thick
IMAGE=alpine:latest

kind load docker-image $IMAGE_NAME --name $1
kind load docker-image $IMAGE --name $1
kubectl apply -f $(pwd)/yamls/multus-daemonset-thick-plugin.yml --kubeconfig $2