#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

# Copy 10-macvlan.tmpl to kind-node
NODES=($(docker ps | grep -w $1 | awk '{print $1}'))
for node in ${NODES[@]}
do
  echo "docker cp $2 $node:/etc/cni/net.d"
  docker cp $2 $node:/etc/cni/net.d
done
