#！/bin/bash
## SPDX-License-Identifier: Apache-2.0
## Copyright Authors of Spider

OS=$(uname | tr 'A-Z' 'a-z')
READY=true

echo "Check kubectl/docker/helm/kind/p2ctl is ready:"

# check docker kind helm p2ctl is exist:
if ! command -v docker > /dev/null 2>&1 ; then
  echo "docker no ready, Please install it manually"
  READY=false
else
  echo "docker ready"
fi

if ! command -v kubectl > /dev/null 2>&1 ; then
  echo "kubectl no ready, Please visit "https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$OS/amd64/kubectl" install it or use ./test/install-tools.sh"
  READY=false
else
  echo "kubectl ready!"
fi
if ! command -v kind > /dev/null 2>&1 ; then
  echo "kind no ready, Please visit "https://github.com/kubernetes-sigs/kind/releases" install it or use ./test/install-tools.sh"
  READY=false
else
  echo "kind ready!"
fi

if ! command -v helm > /dev/null 2>&1 ; then
  echo "helm no ready, Please visit "https://github.com/helm/helm/releases" install it or use ./test/install-tools.sh"
  READY=false
else
  echo "helm ready!"
fi

if ! command -v p2ctl > /dev/null 2>&1 ; then
  echo "p2ctl no ready, Please visit "https://github.com/wrouesnel/p2cli/releases" install it or use ./test/install-tools.sh"
  READY=false
else
  echo "p2ctl ready!"
fi

if [ "$READY" = "false" ]; then
  echo "Some tools no ready, failed to setup kind cluster"
  exit 1
fi


# prepare cni-plugins
if [ ! -f  "$1/tmp/cni-plugins-linux-amd64-v0.8.5.tgz" ]; then
  echo "$1/tmp/cni-plugins-linux-amd64-v0.8.5.tgz no exist, downloading..."
  wget -P $1/tmp https://github.com/containernetworking/plugins/releases/download/v0.8.5/cni-plugins-linux-amd64-v0.8.5.tgz
else
  echo "$1/tmp/cni-plugins-linux-amd64-v0.8.5.tgz exist, skip download..."
fi

if test -z "$IMAGE_MUTLUS"; then
  echo "ERR: multus image tag  no specify, exit"
  exit 1
else
  echo "multus image tag: $IMAGE_MUTLUS"
fi

if test -z "$IMAGE_WHEREABOUTS"; then
  echo "ERR: whereabouts use image tag no specify, exit"
  exit 1
else
  echo "whereabouts use image tag: $IMAGE_WHEREABOUTS"
fi

if test -z "$TEST_IMAGE"; then
  echo "ERR: test image tag no specify, exit"
  exit 1
else
  echo "test image use: $TEST_IMAGE"
fi

# prepare image
IMAGES="
$IMAGE_MUTLUS \
$IMAGE_WHEREABOUTS \
$TEST_IMAGE"

for image in $IMAGES
do
  SUFF_IMAGE=$(echo $image | awk -F ':' '{print $1}')
  EXIST=$(docker images | grep $SUFF_IMAGE)
  if test -z "$EXIST"; then
    echo "Image: $image not exist locally, will pull..."
    docker pull $image
  else
    echo "Image: $image already exist locally"
  fi
done
