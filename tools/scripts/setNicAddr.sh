#!/bin/bash

# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

:<<EOF
way1:
    INTERFACE="eth1" \
    IPV4_IP="172.16.0.10/24"  IPV4_GATEWAY="172.16.0.1" \
    IPV6_IP="fd00::10/64"     IPV6_GATEWAY="fd00::1" \
    MTU="4200" \
    ENABLE_POLICY_ROUTE="true" \
    ENABLE_RDMA_DEFAULT_ROUTE="true" \
    RDMA_SUBNET="172.16.0.0/16" \
    ./setNicAddr.sh

WAY2 (DHCP)：
    INTERFACE="eth1" \
    DHCP4="true" \
    ./setNicAddr.sh

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
ENABLE_RDMA_DEFAULT_ROUTE=${ENABLE_RDMA_DEFAULT_ROUTE:-""}
RDMA_SUBNET=${RDMA_SUBNET:-""}

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
echo "ENABLE_RDMA_DEFAULT_ROUTE=${ENABLE_RDMA_DEFAULT_ROUTE}"
echo "RDMA_SUBNET=${RDMA_SUBNET}"

#========
# Function to configure network using netplan (Ubuntu/Debian)
configure_netplan() {
    Config_IP=""
    [ -n "$IPV4_IP" ] && \
    Config_IP="       - \"${IPV4_IP}\""
    [ -n "$IPV6_IP" ] && \
    Config_IP="\
${Config_IP}
       - \"${IPV6_IP}\""

    Config_gw=""
    if [ "${ENABLE_POLICY_ROUTE}" != "true" ]; then
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

    # Add RDMA subnet route configuration
    if [ "${ENABLE_RDMA_DEFAULT_ROUTE}" == "true" ] && [ -n "${RDMA_SUBNET}" ] && [ -n "${IPV4_GATEWAY}" ]; then
        Config_RDMA_ROUTE="\
      routes:
        - to: ${RDMA_SUBNET}
          via: ${IPV4_GATEWAY}
          on-link: true"
        
        # If we already have routes configured, append to them
        if [ -n "${Config_gw}" ]; then
            Config_gw="${Config_gw}
        - to: ${RDMA_SUBNET}
          via: ${IPV4_GATEWAY}
          on-link: true"
        elif [ -n "${Config_ROUTE}" ]; then
            Config_ROUTE="${Config_ROUTE}
        - to: ${RDMA_SUBNET}
          via: ${IPV4_GATEWAY}
          on-link: true"
        else
            Config_gw="${Config_RDMA_ROUTE}"
        fi
    fi

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
}

# Function to configure network using nmcli (CentOS/RHEL/Fedora)
configure_nmcli() {
    # Check if nmcli is available
    if ! command -v nmcli &> /dev/null; then
        echo "ERROR: nmcli command not found. Please install NetworkManager."
        exit 1
    fi

    # Remove existing connection if it exists
    nmcli connection delete "${INTERFACE}" 2>/dev/null || true

    # Create base connection
    if [ "${DHCP4}" == "true" ]; then
        nmcli connection add type ethernet con-name "${INTERFACE}" ifname "${INTERFACE}" ipv4.method auto
    else
        nmcli connection add type ethernet con-name "${INTERFACE}" ifname "${INTERFACE}" ipv4.method manual
        [ -n "${IPV4_IP}" ] && nmcli connection modify "${INTERFACE}" ipv4.addresses "${IPV4_IP}"
        [ -n "${IPV4_GATEWAY}" ] && [ "${ENABLE_POLICY_ROUTE}" != "true" ] && nmcli connection modify "${INTERFACE}" ipv4.gateway "${IPV4_GATEWAY}"
    fi

    # Configure IPv6
    if [ "${DHCP6}" == "true" ]; then
        nmcli connection modify "${INTERFACE}" ipv6.method auto
    elif [ -n "${IPV6_IP}" ]; then
        nmcli connection modify "${INTERFACE}" ipv6.method manual
        nmcli connection modify "${INTERFACE}" ipv6.addresses "${IPV6_IP}"
        [ -n "${IPV6_GATEWAY}" ] && [ "${ENABLE_POLICY_ROUTE}" != "true" ] && nmcli connection modify "${INTERFACE}" ipv6.gateway "${IPV6_GATEWAY}"
    else
        nmcli connection modify "${INTERFACE}" ipv6.method ignore
    fi

    # Set MTU if specified
    [ -n "${MTU}" ] && nmcli connection modify "${INTERFACE}" ethernet.mtu "${MTU}"


    # Activate the connection
    nmcli connection up "${INTERFACE}"

    # Handle policy routing for nmcli (create persistent dispatcher script)
    if [ -n "${POLICY_ROUTE_TABLE}" ]; then
        # Create NetworkManager dispatcher script for persistent policy routing
        DISPATCHER_SCRIPT="/etc/NetworkManager/dispatcher.d/99-${INTERFACE}-policy-route"
        
        cat > "${DISPATCHER_SCRIPT}" <<DISPATCHER_EOF
#!/bin/bash
# NetworkManager dispatcher script for ${INTERFACE} policy routing
# Auto-generated by setNicAddr.sh

if [ "\$1" = "${INTERFACE}" ] && [ "\$2" = "up" ]; then
    # Check and add route to custom table if not exists
    if ! ip route show table "${POLICY_ROUTE_TABLE}" | grep -q "default via ${IPV4_GATEWAY} dev ${INTERFACE}"; then
        ip route add default via "${IPV4_GATEWAY}" dev "${INTERFACE}" table "${POLICY_ROUTE_TABLE}" && \
            echo "Added route: default via ${IPV4_GATEWAY} dev ${INTERFACE} table ${POLICY_ROUTE_TABLE}"
    fi
    
    # Check and add rule for source-based routing if not exists
    if ! ip rule show | grep -q "from ${IPV4_IP%%/*} lookup ${POLICY_ROUTE_TABLE}"; then
        ip rule add from "${IPV4_IP%%/*}" table "${POLICY_ROUTE_TABLE}" && \
            echo "Added rule: from ${IPV4_IP%%/*} table ${POLICY_ROUTE_TABLE}"
    fi
    
    echo "Policy routing verified for ${INTERFACE}: table ${POLICY_ROUTE_TABLE}"
elif [ "\$1" = "${INTERFACE}" ] && [ "\$2" = "down" ]; then
    # Clean up rules when interface goes down
    ip rule del from "${IPV4_IP%%/*}" table "${POLICY_ROUTE_TABLE}" 2>/dev/null || true
    echo "Policy routing cleaned up for ${INTERFACE}: table ${POLICY_ROUTE_TABLE}"
fi
DISPATCHER_EOF

        # Make the dispatcher script executable
        chmod +x "${DISPATCHER_SCRIPT}"
        
        # Apply the policy routing immediately
        ip route add default via "${IPV4_GATEWAY}" dev "${INTERFACE}" table "${POLICY_ROUTE_TABLE}" 2>/dev/null || true
        ip rule add from "${IPV4_IP%%/*}" table "${POLICY_ROUTE_TABLE}" 2>/dev/null || true
        
        echo "Policy routing configured persistently: table ${POLICY_ROUTE_TABLE}"
        echo "Dispatcher script created: ${DISPATCHER_SCRIPT}"
    fi

    # Configure RDMA subnet route for nmcli
    if [ "${ENABLE_RDMA_DEFAULT_ROUTE}" == "true" ] && [ -n "${RDMA_SUBNET}" ] && [ -n "${IPV4_GATEWAY}" ]; then
        # Add RDMA subnet route immediately
        ip route add "${RDMA_SUBNET}" via "${IPV4_GATEWAY}" dev "${INTERFACE}" 2>/dev/null || true
        
        # Create or update dispatcher script for RDMA route persistence
        RDMA_DISPATCHER_SCRIPT="/etc/NetworkManager/dispatcher.d/98-${INTERFACE}-rdma-route"
        
        cat > "${RDMA_DISPATCHER_SCRIPT}" <<RDMA_DISPATCHER_EOF
#!/bin/bash
# NetworkManager dispatcher script for ${INTERFACE} RDMA routing
# Auto-generated by setNicAddr.sh

if [ "\$1" = "${INTERFACE}" ] && [ "\$2" = "up" ]; then
    # Check and add RDMA subnet route if not exists
    if ! ip route show | grep -q "${RDMA_SUBNET} via ${IPV4_GATEWAY} dev ${INTERFACE}"; then
        ip route add "${RDMA_SUBNET}" via "${IPV4_GATEWAY}" dev "${INTERFACE}" && \
            echo "Added RDMA route: ${RDMA_SUBNET} via ${IPV4_GATEWAY} dev ${INTERFACE}"
    fi
elif [ "\$1" = "${INTERFACE}" ] && [ "\$2" = "down" ]; then
    # Clean up RDMA route when interface goes down
    ip route del "${RDMA_SUBNET}" via "${IPV4_GATEWAY}" dev "${INTERFACE}" 2>/dev/null || true
    echo "RDMA route cleaned up for ${INTERFACE}: ${RDMA_SUBNET}"
fi
RDMA_DISPATCHER_EOF

        # Make the RDMA dispatcher script executable
        chmod +x "${RDMA_DISPATCHER_SCRIPT}"
        
        echo "RDMA subnet route configured: ${RDMA_SUBNET} via ${IPV4_GATEWAY} dev ${INTERFACE}"
        echo "RDMA dispatcher script created: ${RDMA_DISPATCHER_SCRIPT}"
    fi

    echo "Network interface ${INTERFACE} configured using nmcli"
}

if which netplan &>/dev/null; then
    echo "Using netplan to configure network interface ${INTERFACE}"
    configure_netplan
elif which nmcli &>/dev/null; then
    echo "Using nmcli to configure network interface ${INTERFACE}"
    configure_nmcli
else
    echo "ERROR: netplan or nmcli not found"
    exit 1
fi
