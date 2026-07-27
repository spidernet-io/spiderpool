#!/usr/bin/env bash

# Copyright 2024 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# There are many versions of k8S that have been released. We focus on some specific versions, 
# which are also the versions that we all adapt to.
# But as new K8S versions are released, we need to be compatible with more K8S versions. 
# This script will detect whether there is a new version released, and then update the K8S matrix file

set -x
set -o nounset
set -o pipefail

# MINIMUM_K8S_VERSION represents that k8s matrix tests should run on all distributions greater than this version.
MINIMUM_K8S_VERSION="$1"
[ -z "$MINIMUM_K8S_VERSION" ] && echo "error, miss MINIMUM_K8S_VERSION " && exit 1
K8S_MATRIX_FILE_PATH="$2"
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

# Build the new JSON array
NEW_JSON_ARRAY=$(echo "$KIND_NODE_VERSION" | tr ',' '\n' | sed 's/^ *//;s/ *$//' | jq -R . | jq -sc .)

# Read origin version from JSON file and normalize to compact form for comparison
ORIGIN_JSON_ARRAY=$(cat "$K8S_MATRIX_FILE_PATH" | jq -c .)
echo "origin version: $ORIGIN_JSON_ARRAY"
echo "updated version: $NEW_JSON_ARRAY"

ORIGIN_COUNT=$(echo "$ORIGIN_JSON_ARRAY" | jq 'length')
NEW_COUNT=$(echo "$NEW_JSON_ARRAY" | jq 'length')

if [[ "$ORIGIN_JSON_ARRAY" == "$NEW_JSON_ARRAY" ]]; then
  echo "ORIGIN and NEW versions are equal"
  exit 0
elif [[ "$NEW_COUNT" -ge "$ORIGIN_COUNT" ]]; then
  echo "kind/node releases a new version, updates it in k8s matrix."
  echo "$NEW_JSON_ARRAY" > "$K8S_MATRIX_FILE_PATH"
  exit 0
else
  echo "update failed, please check."
  exit 1
fi