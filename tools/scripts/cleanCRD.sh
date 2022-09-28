#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

kubectl get spiderippools | sed '1 d' | awk '{print $1}' | xargs -n 1 -i kubectl patch spiderippools {} --patch '{"metadata": {"finalizers": null}}' --type=merge

kubectl get spidersubnet | sed '1 d' | awk '{print $1}' | xargs -n 1 -i kubectl patch spidersubnet {} --patch '{"metadata": {"finalizers": null}}' --type=merge

ALL_EP_INFO=`kubectl get spiderendpoints -A | sed '1 d'`
while read namespace epName other ; do
  kubectl patch -n $namespace $epName --patch '{"metadata": {"finalizers": null}}' --type=merge
done <<< "$ALL_EP_INFO"

kubectl delete crd spiderippools
kubectl delete crd spiderreservedips
kubectl delete crd spiderendpoints
kubectl delete crd spidersubnets
