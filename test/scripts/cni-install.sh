#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

# Copy 10-macvlan.tmpl to kind-node
NODES=($(docker ps | grep -w $1 | awk '{print $1}'))
for node in ${NODES[@]}
do
  echo "docker cp $2/tmp/cni-plugins-linux-amd64-v0.8.5.tgz $node:/root"
  docker cp $2/tmp/cni-plugins-linux-amd64-v0.8.5.tgz $node:/root/
  docker exec $node mkdir -p /host/opt/cni/bin
  docker exec $node tar xvfzp /root/cni-plugins-linux-amd64-v0.8.5.tgz -C /opt/cni/bin
done
