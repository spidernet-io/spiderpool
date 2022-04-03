#!/bin/bash

# Copyright 2017-2020 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

# ./update-golang-image.sh 1.18

set -o xtrace
set -o errexit
set -o pipefail
set -o nounset


if [ "$#" -gt 1 ] ; then
  echo "$0 supports at most 1 argument"
  exit 1
fi

go_version=${GO_VERSION:-"1.17.8"}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

root_dir="$(git rev-parse --show-toplevel)"

cd "${root_dir}"


image="docker.io/library/golang:${go_version}"
image_digest="$("${script_dir}/get-image-digest.sh" "${image}")"

if [ -z "${image_digest}" ]; then
  echo "Image digest not available"
  exit 1
fi


# shellcheck disable=SC2207
used_by=($(git grep -l GOLANG_IMAGE= images/*/Dockerfile))

for i in "${used_by[@]}" ; do
    # golang images with image digest
    sed "s|GOLANG_IMAGE=docker\.io/library/golang:[0-9][0-9]*\.[0-9][0-9]*\(\.[0-9][0-9]*\)\?@.*|GOLANG_IMAGE=${image}@${image_digest}|" "${i}" > "${i}.sedtmp" && mv "${i}.sedtmp" "${i}"
done
