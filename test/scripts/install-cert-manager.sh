#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider-net

set -o errexit
set -o nounset
set -o pipefail

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

CERT_MANAGER_CHART_PATH=$( cd ${CURRENT_DIR_PATH}/../yamls/cert-manager-v1.6.1 && pwd )

helm install cert-manager ${CERT_MANAGER_CHART_PATH} --namespace kube-system --set installCRDs=true
