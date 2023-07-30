#!/bin/bash

# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$E2E_IP_FAMILY" ] && echo "error, miss E2E_IP_FAMILY " && exit 1
echo "$CURRENT_FILENAME : E2E_IP_FAMILY $E2E_IP_FAMILY "

DEFAULT_INTERFACE=eth0
VLAN_GATEWAY_CONTAINER=${VLAN_GATEWAY_CONTAINER:-"vlan-gateway"}
VLANID1=100
VLANID2=200
E2E_VLAN_GATEWAY_IMAGE=${E2E_VLAN_GATEWAY_IMAGE:-"docker.io/centos/tools:latest"}


# run a test container as a vlan gateway and client
# note: ip address of this container should be consist with spiderpool's gateway
docker stop ${VLAN_GATEWAY_CONTAINER}  &>/dev/null || true
docker rm ${VLAN_GATEWAY_CONTAINER}  &>/dev/null || true

containerID=`docker run -itd  --name ${VLAN_GATEWAY_CONTAINER} --network kind --cap-add=NET_ADMIN --privileged ${E2E_VLAN_GATEWAY_IMAGE}`
docker exec ${containerID} ip link add link ${DEFAULT_INTERFACE} name ${DEFAULT_INTERFACE}.${VLANID1} type vlan id ${VLANID1}
docker exec ${containerID} ip link add link ${DEFAULT_INTERFACE} name ${DEFAULT_INTERFACE}.${VLANID2} type vlan id ${VLANID2}
docker exec ${containerID} ip link set ${DEFAULT_INTERFACE}.${VLANID1} up
docker exec ${containerID} ip link set ${DEFAULT_INTERFACE}.${VLANID2} up
if [ ${E2E_IP_FAMILY} == "ipv4" ]; then
    docker exec ${containerID} ip addr add 172.100.0.1/16 dev ${DEFAULT_INTERFACE}.${VLANID1}
    docker exec ${containerID} ip addr add 172.200.0.1/16 dev ${DEFAULT_INTERFACE}.${VLANID2}
elif [ ${E2E_IP_FAMILY} == "ipv6" ]; then
    docker exec ${containerID}  sysctl -w net.ipv6.conf.all.disable_ipv6=0
    docker exec ${containerID}  sysctl -w net.ipv6.conf.all.forwarding=1
    docker exec ${containerID} ip addr add fd00:172:100::1/64 dev ${DEFAULT_INTERFACE}.${VLANID1}
    docker exec ${containerID} ip addr add fd00:172:200::1/64 dev ${DEFAULT_INTERFACE}.${VLANID2}
elif [ ${E2E_IP_FAMILY} == "dual" ]; then
    docker exec ${containerID}  sysctl -w net.ipv6.conf.all.disable_ipv6=0
    docker exec ${containerID}  sysctl -w net.ipv6.conf.all.forwarding=1
    docker exec ${containerID} ip addr add 172.100.0.1/16 dev ${DEFAULT_INTERFACE}.${VLANID1}
    docker exec ${containerID} ip addr add 172.200.0.1/16 dev ${DEFAULT_INTERFACE}.${VLANID2}
    docker exec ${containerID} ip addr add fd00:172:100::1/64 dev ${DEFAULT_INTERFACE}.${VLANID1}
    docker exec ${containerID} ip addr add fd00:172:200::1/64 dev ${DEFAULT_INTERFACE}.${VLANID2}
else
    echo "error ip family, the value of IP_FAMILY must be of ipv4,ipv6 or dual." && exit 1
fi

echo -e "\033[35m Succeed to create vlan interface: ${DEFAULT_INTERFACE}.${VLANID1}、 ${DEFAULT_INTERFACE}.${VLANID2} in kind-node \033[0m"
