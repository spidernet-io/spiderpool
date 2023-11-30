#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

set -o errexit -o nounset

CURRENT_FILENAME=$( basename $0 )

[ -z "${HTTP_PROXY}" ] || export https_proxy=${HTTP_PROXY}

[ -z "$E2E_CLUSTER_NAME" ] && echo "error, miss E2E_CLUSTER_NAME " && exit 1
echo "$CURRENT_FILENAME : E2E_CLUSTER_NAME $E2E_CLUSTER_NAME "

[ -z "$DOCKER_ADDITIONAL_NETWORK" ] && echo "error, miss DOCKER_ADDITIONAL_NETWORK " && exit 1
echo "$CURRENT_FILENAME : DOCKER_ADDITIONAL_NETWORK $DOCKER_ADDITIONAL_NETWORK "

[ -z "$BRIDGE_INTERFACE" ] && echo "error, miss BRIDGE_INTERFACE " && exit 1
echo "$CURRENT_FILENAME : BRIDGE_INTERFACE $BRIDGE_INTERFACE "

[ -z "$HOST_ADDITIONAL_INTERFACE" ] && echo "error, miss HOST_ADDITIONAL_INTERFACE " && exit 1
echo "$CURRENT_FILENAME : HOST_ADDITIONAL_INTERFACE $HOST_ADDITIONAL_INTERFACE "

# add secondary network nic for Node spider-control-plane and spider-worker to build ovs bridge
echo "try to add secondary network nic for ovs bridge preparation"
IS_DOCKER_NETWORK_EXIST=$(docker network ls | grep ${DOCKER_ADDITIONAL_NETWORK} | wc -l)
if [ "${IS_DOCKER_NETWORK_EXIST}" -eq 0 ]; then
  echo "try to create docker network ${DOCKER_ADDITIONAL_NETWORK}"
  docker network create ${DOCKER_ADDITIONAL_NETWORK} --driver bridge
fi

# try to configure vlan gateway
VLAN30=30
VLAN40=40
VLAN_GATEWAY_CONTAINER=${VLAN_GATEWAY_CONTAINER:-"vlan-gateway"}
echo "=========try to configure vlan gateway: ${VLAN30} and ${VLAN40}"
docker network connect ${DOCKER_ADDITIONAL_NETWORK} ${VLAN_GATEWAY_CONTAINER}
docker exec ${VLAN_GATEWAY_CONTAINER} ip link add link ${HOST_ADDITIONAL_INTERFACE} name ${HOST_ADDITIONAL_INTERFACE}.${VLAN30} type vlan id ${VLAN30}
docker exec ${VLAN_GATEWAY_CONTAINER} ip link add link ${HOST_ADDITIONAL_INTERFACE} name ${HOST_ADDITIONAL_INTERFACE}.${VLAN40} type vlan id ${VLAN40}
docker exec ${VLAN_GATEWAY_CONTAINER} ip link set ${HOST_ADDITIONAL_INTERFACE}.${VLAN30} up
docker exec ${VLAN_GATEWAY_CONTAINER} ip link set ${HOST_ADDITIONAL_INTERFACE}.${VLAN40} up

if [ ${E2E_IP_FAMILY} == "ipv4" ]; then
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add 172.30.0.1/16 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN30}
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add 172.40.0.1/16 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN40}
elif [ ${E2E_IP_FAMILY} == "ipv6" ]; then
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add fd00:172:30::1/64 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN30}
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add fd00:172:40::1/64 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN40}
elif [ ${E2E_IP_FAMILY} == "dual" ]; then
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add 172.30.0.1/16 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN30}
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add 172.40.0.1/16 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN40}
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add fd00:172:30::1/64 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN30}
  docker exec ${VLAN_GATEWAY_CONTAINER} ip addr add fd00:172:40::1/64 dev ${HOST_ADDITIONAL_INTERFACE}.${VLAN40}
else
    echo "error ip family, the value of IP_FAMILY must be of ipv4,ipv6 or dual." && exit 1
fi

echo -e "\033[35m Succeed to create vlan interface: ${HOST_ADDITIONAL_INTERFACE}.${VLAN30}、 ${HOST_ADDITIONAL_INTERFACE}.${VLAN40} in kind-node ${VLAN_GATEWAY_CONTAINER} \033[0m"

KIND_NODES=`kind get nodes --name ${E2E_CLUSTER_NAME}`
for NODE in $KIND_NODES ; do
  echo "=========connect node ${NODE} to additional docker network ${DOCKER_ADDITIONAL_NETWORK}"
  docker network connect ${DOCKER_ADDITIONAL_NETWORK} ${NODE}

  echo "=========install openvswitch"
  # docker exec ${NODE} apt-get update > /dev/null
  docker exec ${NODE} apt-get install -y openvswitch-switch > /dev/null
  docker exec ${NODE} systemctl start openvswitch-switch
  docker exec ${NODE} ovs-vsctl add-br ${BRIDGE_INTERFACE}
  docker exec ${NODE} ovs-vsctl add-port ${BRIDGE_INTERFACE} ${HOST_ADDITIONAL_INTERFACE}

  echo "=========configure bridge ${BRIDGE_INTERFACE}"
  eth1_ip=$(docker exec ${NODE} ip addr show dev ${HOST_ADDITIONAL_INTERFACE} | grep -Po 'inet \K[\d.]+')
  docker exec ${NODE} ip addr add ${eth1_ip}/16 dev ${BRIDGE_INTERFACE}
  docker exec ${NODE} ip addr flush ${HOST_ADDITIONAL_INTERFACE}
  docker exec ${NODE} ip link set ${BRIDGE_INTERFACE} up

  if [ ${E2E_IP_FAMILY} == "ipv4" ]; then
    docker exec ${NODE} ip route add 172.30.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.30.0.0/16 via 172.30.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.40.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.40.0.0/16 via 172.40.0.1 dev ${BRIDGE_INTERFACE}
  elif [ ${E2E_IP_FAMILY} == "ipv6" ]; then
    docker exec ${NODE} ip route add fd00:172:30::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:30::/64 via fd00:172:30::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:40::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:40::/64 via fd00:172:40::1 dev ${BRIDGE_INTERFACE}
  elif [ ${E2E_IP_FAMILY} == "dual" ]; then
    docker exec ${NODE} ip route add 172.30.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.30.0.0/16 via 172.30.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.40.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add 172.40.0.0/16 via 172.40.0.1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:30::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:30::/64 via fd00:172:30::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:40::1 dev ${BRIDGE_INTERFACE}
    docker exec ${NODE} ip route add fd00:172:40::/64 via fd00:172:40::1 dev ${BRIDGE_INTERFACE}
  else
      echo "error ip family, the value of IP_FAMILY must be of ipv4,ipv6 or dual." && exit 1
  fi
done

echo -e "\033[35m Succeed to install openvswitch \033[0m"

