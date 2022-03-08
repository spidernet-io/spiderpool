#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0


set -o xtrace
set -o errexit
set -o pipefail
set -o nounset

#test all tools works for built image
iptables -h >/dev/null
ip6tables -h >/dev/null


exit 0

