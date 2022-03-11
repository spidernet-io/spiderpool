#!/bin/bash

set -o xtrace
set -o errexit
set -o pipefail
set -o nounset

#test all tools works for built image
iptables -h >/dev/null
ip6tables -h >/dev/null


exit 0

