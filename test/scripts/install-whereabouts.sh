#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

kind load docker-image $IMAGE_WHEREABOUTS --name $1

# Install whereabouts
kubectl apply \
      -f $(pwd)/yamls/daemonset-install.yaml \
      -f $(pwd)/yamls/whereabouts.cni.cncf.io_ippools.yaml \
      -f $(pwd)/yamls/whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml \
      -f $(pwd)/yamls/ip-reconciler-job.yaml --kubeconfig $2
