#!/bin/bash

:<<EOF
way1：
    INTERFACE="eth1" \
    IPV4_IP="172.16.0.10/24"  IPV4_GATEWAY="172.16.0.1" \
    IPV6_IP="fd00::10/64"     IPV6_GATEWAY="fd00::1" \
    MTU="4200" \
    ENABLE_POLICY_ROUTE="true" \
    ./set-netplan.sh

WAY2：
    INTERFACE="eth1" \
    DHCP4="true" \
    set-netplan.sh

EOF

set -o errexit

INTERFACE=${INTERFACE:-""}
IPV4_IP=${IPV4_IP:-""}
IPV4_GATEWAY=${IPV4_GATEWAY:-""}
IPV6_IP=${IPV6_IP:-""}
IPV6_GATEWAY=${IPV6_GATEWAY:-""}
MTU=${MTU:-""}
DHCP4=${DHCP4:-""}
DHCP6=${DHCP6:-""}
ENABLE_POLICY_ROUTE=${ENABLE_POLICY_ROUTE:-""}
POLICY_ROUTE_TABLE=${POLICY_ROUTE_TABLE:-""}

[ -n "${INTERFACE}" ] || { echo "ERROR: INTERFACE is empty"; exit 1; }

ip a s ${INTERFACE} &>/dev/null || { echo "error, interface ${INTERFACE} not found"; exit 1; }

if [ -n "${POLICY_ROUTE_TABLE}" ] || [ "$ENABLE_POLICY_ROUTE" == "true" ] ; then
  if [ -z "${IPV4_GATEWAY}" ]; then
    echo "error, must set IPV4_GATEWAY when setting POLICY_ROUTE_TABLE"
    exit 1
  fi
  if [ -z "${POLICY_ROUTE_TABLE}" ] ; then 
      USED_TABLE=$( ip rule | grep -oE "lookup [0-9]+" | awk '{print $2}' | tr '\n' ' ' )
      for (( i=150 ; i++ ; i<200 )); do 
        if ! grep " ${i} " <<< " ${USED_TABLE} " &>/dev/null ; then 
            POLICY_ROUTE_TABLE="$i"
            break
        fi
      done
  fi
  ROUTE_NUM=$( ip r s table ${POLICY_ROUTE_TABLE} 2>/dev/null | wc -l )
  if (( ROUTE_NUM != 0 )); then 
    echo "error, route table ${POLICY_ROUTE_TABLE} is not empty"
    exit 1
  fi
fi

if [ "${DHCP4}" == "true" ] ; then
  IPV4_IP=""
  IPV4_GATEWAY=""
else
  DHCP4=""
fi

if [ "${DHCP6}" == "true" ] ; then
  IPV6_IP=""
  IPV6_GATEWAY=""
else
  DHCP6=""
fi

echo "INTERFACE=${INTERFACE}"
echo "IPV4_IP=${IPV4_IP}"
echo "IPV4_GATEWAY=${IPV4_GATEWAY}"
echo "IPV6_IP=${IPV6_IP}"
echo "IPV6_GATEWAY=${IPV6_GATEWAY}"
echo "MTU=${MTU}"
echo "DHCP4=${DHCP4}"
echo "DHCP6=${DHCP6}"
echo "POLICY_ROUTE_TABLE=${POLICY_ROUTE_TABLE}"

#========
Config_IP=""
[ -n "$IPV4_IP" ] && \
Config_IP="       - \"${IPV4_IP}\""
[ -n "$IPV6_IP" ] && \
Config_IP="\
${Config_IP}
       - \"${IPV6_IP}\""

Config_gw=""
if [ -n "$IPV4_GATEWAY" ] || [ -n "$IPV6_GATEWAY" ] ; then
Config_gw="      routes:"
[ -n "$IPV4_GATEWAY" ] && \
Config_gw="\
${Config_gw}
        - to: default
          via: ${IPV4_GATEWAY}
          metric: 10"
[ -n "$IPV6_GATEWAY" ] && \
Config_gw="\
${Config_gw}
        - to: default
          via: ${IPV6_GATEWAY}
          metric: 10"
fi
[ -n "$MTU" ] && \
Config_MTU="\
      mtu: ${MTU}"

[ "$DHCP4" == "true" ] && \
DHCP_CONFIG="\
      dhcp4: true"


[ "$DHCP6" == "true" ] && \
DHCP_CONFIG+="\
      dhcp6: true"

[ -n "$POLICY_ROUTE_TABLE" ] && \
Config_ROUTE="\
      routes:
        - to: 0.0.0.0/0
          via: ${IPV4_GATEWAY}
          table: ${POLICY_ROUTE_TABLE}"

[ -n "$POLICY_ROUTE_TABLE" ] && \
Config_RULE="\
      routing-policy:
        - from: ${IPV4_IP%%/*}
          table: ${POLICY_ROUTE_TABLE}"

cat <<EOF >/etc/netplan/12-${INTERFACE}.yaml
network:
  version: 2
  renderer: networkd
  ethernets:
    ${INTERFACE}:
$( [ -n "${DHCP_CONFIG}" ] && echo "${DHCP_CONFIG}" )
$( [ -n "${DHCP_CONFIG}" ] || echo "      addresses:" )
$( [ -n "${Config_IP}" ] && echo "${Config_IP}" )
$( [ -n "${Config_gw}" ] && echo "${Config_gw}" )
$( [ -n "${Config_MTU}" ] && echo "${Config_MTU}" )
$( [ -n "${POLICY_ROUTE_TABLE}" ] && echo "${Config_ROUTE}" )
$( [ -n "${POLICY_ROUTE_TABLE}" ] && echo "${Config_RULE}" )
EOF

echo "new config: /etc/netplan/12-${INTERFACE}.yaml"

# Permissions for /etc/netplan/*.yaml are too open. Netplan configuration should NOT be accessible by others
chmod 600 /etc/netplan/*
netplan apply
