#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

INTERFACE=eth0
CHILD_INTERFACE=macvlan1

IPV4_ADDR=$(  ip -4 addr show $INTERFACE | grep -oP '(?<=inet\s)[0-9]+(\.[0-9]+){3}/[0-9]+' )
IPV6_ADDR=$(  ip -6 addr show $INTERFACE | grep -v "scope link" |  grep -oP '(?<=inet6\s)[0-9a-f:]+/[0-9]+' )

IPV4_GW=$( ip r get 8.8.8.8 | grep $INTERFACE | egrep -o 'via .*' | awk '{print $2}' )
IPV6_GW=$( ip r get 2001::1 | grep $INTERFACE | egrep -o 'via .*' | awk '{print $2}' )

echo "for interface $INTERFACE, ipv4=$IPV4_ADDR , ipv6=$IPV6_ADDR , ipv4 gw=$IPV4_GW , ipv6 gw=$IPV6_GW "

DISABLE_IPV6=$( sysctl net.ipv6.conf.all.disable_ipv6 | grep "= 1" )
[ -n "$DISABLE_IPV6" ] && echo " ipv6 is disabled "

ip link add link $INTERFACE name $CHILD_INTERFACE type macvlan  mode bridge
ip link set $CHILD_INTERFACE up
ip link set $INTERFACE promisc on

ip -4 a f $INTERFACE || true
[ -n "$IPV4_GW" ] && ( ip -4 r delete default || true )
[ -n "$IPV4_ADDR" ] && ip a a $IPV4_ADDR dev $CHILD_INTERFACE
[ -n "$IPV4_GW" ] && ip r add default via $IPV4_GW dev $CHILD_INTERFACE

if [ -z "$DISABLE_IPV6" ] ; then
  ip -6 a f $INTERFACE || true
  [ -n "$IPV6_GW" ] && ( ip -6 r delete default || true )
  [ -n "$IPV6_ADDR" ] && ip a a $IPV6_ADDR dev $CHILD_INTERFACE
  [ -n "$IPV6_GW" ] && ip -6 r add default via $IPV6_GW dev $CHILD_INTERFACE
fi
