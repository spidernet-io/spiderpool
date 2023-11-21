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

for image in $IMAGE_LIST ; do
    PREFIX_IMAGE=$(echo $image | awk -F ':' '{print $1}')
    SUFFIX_IMAGE=$(echo $image | awk -F ':' '{print $2}')
    if docker images | grep -E "^${PREFIX_IMAGE}[[:space:]]+${SUFFIX_IMAGE} " &>/dev/null ; then
      echo "Image: $image already exist locally "
    else
      echo "Image: pulling $image"
      docker pull $image
    fi
done
