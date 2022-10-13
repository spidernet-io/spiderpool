#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

kubectl get spiderippools | sed '1 d' | awk '{print $1}' | xargs -n 1 -i kubectl patch spiderippools {} --patch '{"metadata": {"finalizers": null}}' --type=merge

kubectl get spidersubnets | sed '1 d' | awk '{print $1}' | xargs -n 1 -i kubectl patch spidersubnets {} --patch '{"metadata": {"finalizers": null}}' --type=merge

ALL_EP_INFO=`kubectl get spiderendpoints -A | sed '1 d'`
while read namespace epName other ; do
  kubectl patch -n $namespace spiderendpoints $epName --patch '{"metadata": {"finalizers": null}}' --type=merge
done <<< "$ALL_EP_INFO"

kubectl delete crd spiderendpoints.spiderpool.spidernet.io
kubectl delete crd spiderippools.spiderpool.spidernet.io
kubectl delete crd spiderreservedips.spiderpool.spidernet.io
kubectl delete crd spidersubnets.spiderpool.spidernet.io
