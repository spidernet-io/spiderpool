#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
# ensure this file is sourced to add required components to PATH

here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
root="$(readlink --canonicalize "$here/..")"
VERSION="v0.11.0"
KIND_BINARY_URL="https://github.com/kubernetes-sigs/kind/releases/download/${VERSION}/kind-$(uname)-amd64"
K8_STABLE_RELEASE_URL="https://storage.googleapis.com/kubernetes-release/release/stable.txt"

if [ ! -d "${root}/bin" ]; then
    mkdir "${root}/bin"
fi

echo "retrieving kind"
curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo "${root}/bin/kind" "${KIND_BINARY_URL}"
chmod +x "${root}/bin/kind"

echo "retrieving kubectl"
curl -Lo "${root}/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/$(curl -s ${K8_STABLE_RELEASE_URL})/bin/linux/amd64/kubectl"
chmod +x "${root}/bin/kubectl"

export PATH="$PATH:$root/bin"

#go version &> /dev/null
#if [ $? -eq 127 ]; then
#   curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo go1.17.8.linux-amd64.tar.gz "https://go.dev/dl/go1.17.8.linux-amd64.tar.gz"
#   rm -rf /usr/local/go && tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
#   export PATH=$PATH:/usr/local/go/bin
#   rm -f go1.17.8.linux-amd64.tar.g
#fi

# install ginkgo
# export PATH=$PATH:~/go/bin
# ginkgo version &> /dev/null
# if [ $? -eq 127 ]; then
#  go install github.com/onsi/ginkgo/ginkgo@latest
#  go get github.com/onsi/gomega/...
# fi

# kind
# which kind
# if [ $? -ne 0 ]; then
#   curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/v0.11.1/kind-linux-amd64	
#   chmod +x /usr/local/bin/kind
# fi 

# # kubectl
# which kubectl 
# if [ $? -ne 0 ]; then
#   curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl
#   chmod +x /usr/local/bin/kubectl
# fi

# jq
# which jq 
# if [ $? -ne 0 ]; then
#   curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo /usr/local/bin/jq https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64
#   chmod +x /usr/local/bin/jq
# fi

# # helm
# which helm 
# if [ $? -ne 0 ]; then
#   curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo helm.tar.gz "https://get.helm.sh/helm-v3.8.1-linux-amd64.tar.gz"
#   tar -xzvf helm.tar.gz && mv linux-amd64/helm /usr/local/bin
#   chmod +x /usr/local/bin/helm
#   rm -f helm.tar.gz
# fi