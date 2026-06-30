#！/bin/bash
## SPDX-License-Identifier: Apache-2.0
## Copyright Authors of Spider

DOWNLOAD_DIR="$1"
[ -z "$DOWNLOAD_DIR" ] && echo "error, miss DOWNLOAD_DIR" && exit 1
mkdir -p $DOWNLOAD_DIR
echo "Download Package to $DOWNLOAD_DIR "

[ -z "$ARCH" ] && echo "error, miss ARCH " && exit 1
echo "Using ARCH: $ARCH"

[ -z "$IMAGE_LIST" ] && echo "error, miss IMAGE_LIST " && exit 1
echo "all images: $IMAGE_LIST"

#=================================

IMAGE_PULL_RETRY=${IMAGE_PULL_RETRY:-3}
IMAGE_PULL_RETRY_INTERVAL=${IMAGE_PULL_RETRY_INTERVAL:-10}

for image in $IMAGE_LIST ; do
    if docker image inspect "$image" &>/dev/null ; then
      echo "Image: $image already exist locally "
    else
      echo "Image: pulling $image"
      for retry in $(seq 1 "$IMAGE_PULL_RETRY"); do
        if docker pull "$image"; then
          break
        fi
        if [ "$retry" -eq "$IMAGE_PULL_RETRY" ]; then
          echo "error, failed to pull $image after $IMAGE_PULL_RETRY attempts"
          exit 1
        fi
        echo "Image: pull $image failed, retry $retry/$IMAGE_PULL_RETRY after ${IMAGE_PULL_RETRY_INTERVAL}s"
        sleep "$IMAGE_PULL_RETRY_INTERVAL"
      done
    fi
done
