#!/usr/bin/env bash

# Copyright 2024 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# There are many versions of k8S that have been released. We focus on some specific versions, 
# which are also the versions that we all adapt to.
# But as new K8S versions are released, we need to be compatible with more K8S versions. 
# This script will detect whether there is a new version released, and then update the K8S matrix file

set -x
set -o errexit
set -o nounset
set -o pipefail

# MINIMUM_K8S_VERSION represents that k8s matrix tests should run on all distributions greater than this version.
MINIMUM_K8S_VERSION="$1"
[ -z "$MINIMUM_K8S_VERSION" ] && echo "error, miss MINIMUM_K8S_VERSION " && exit 1
K8S_Matrix_Test_File="$2"
[ -z "$K8S_MATRIX_FILE_PATH" ] && echo "error, miss K8S_MATRIX_FILE_PATH " && exit 1
[ ! -f "$K8S_MATRIX_FILE_PATH" ] && echo "error, could not find file $K8S_MATRIX_FILE_PATH " && exit 1


echo "Updating K8S matrix run file:$K8S_MATRIX_FILE_PATH..."
KIND_NODE_VERSION=""
function getAllLatestVersion() {
    page=1
    per_page=100
    repo="kindest/node"
    while true; do
      response=$(curl -s --retry 10 "https://hub.docker.com/v2/repositories/$repo/tags/?page=$page&per_page=$per_page")
      tags=$(echo "$response" | jq -r '.results[].name')
      for tag in $tags; do
        if [[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
          if [[ ! $KIND_NODE_VERSION =~ ${tag%.*} && ${tag%.*} > ${MINIMUM_K8S_VERSION} ]]; then
            KIND_NODE_VERSION+="$tag, "
          fi
        fi
      done
      next=$(echo "$response" | jq -r '.next')
      if [[ "$next" == "null" ]]; then
          break
      fi
      page=$((page + 1))
    done
    KIND_NODE_VERSION=${KIND_NODE_VERSION}
}

getAllLatestVersion

KIND_NODE_VERSION="${KIND_NODE_VERSION%, }"
KIND_NODE_VERSION="[$KIND_NODE_VERSION]"
ORIGIN_K8S_VERSION=$(grep -oP '(?<=version: )\[[^\]]+\]' $K8S_MATRIX_FILE_PATH)
echo "origin version: $ORIGIN_K8S_VERSION"
echo "updated version: $KIND_NODE_VERSION"
if [[ "$ORIGIN_K8S_VERSION" == "$KIND_NODE_VERSION" ]]; then
  echo "ORIGIN_K8S_VERSION:$ORIGIN_K8S_VERSION and KIND_NODE_VERSION:$KIND_NODE_VERSION are equal"
  exit 0
elif [[ ${#KIND_NODE_VERSION} -gt ${#ORIGIN_K8S_VERSION} ]]; then
  echo "kind/node releases a new version, updates it in k8s matrix."
  sed -i 's/\'"${ORIGIN_K8S_VERSION}"/"${KIND_NODE_VERSION}"'/' $K8S_MATRIX_FILE_PATH
else
  echo "update failed, please check."
  exit 1
fi