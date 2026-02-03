# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# docker buildx build -t testbase:latest --platform=linux/arm64,linux/amd64 --output type=docker .

ARG GOLANG_IMAGE=docker.io/library/golang:1.25.6@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2
ARG UBUNTU_IMAGE=docker.io/library/ubuntu:20.04@sha256:bea6d19168bbfd6af8d77c2cc3c572114eb5d113e6f422573c93cb605a0e2ffb



#========= build gops ===============
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} AS gops-cni-builder
COPY /build-gops.sh /build-gops.sh

#use alibaba debian source
#COPY /sources.list /etc/apt/sources.list
#RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 648ACFD622F3D138

#with debian 11 os
RUN apt-get update && apt-get install -y binutils-aarch64-linux-gnu binutils-x86-64-linux-gnu

# build-gops.sh will build both archs at the same time
WORKDIR /
RUN chmod +x /build-gops.sh && /build-gops.sh


#========== root image ==============
FROM ${UBUNTU_IMAGE} AS rootfs
COPY /install-others.sh /install-others.sh
COPY /configure-iptables-wrapper.sh /configure-iptables-wrapper.sh
COPY /iptables-wrapper /usr/sbin/iptables-wrapper
COPY /test-image.sh /test-image.sh

# Change the number to force the generation of a new git-tree SHA. Useful when
# we want to re-run 'apt-get upgrade' for stale images.
ENV FORCE_BUILD=1

#fix warning: debconf: delaying package configuration, since apt-utils is not installed
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get upgrade -y

WORKDIR /
RUN chmod +x /install-others.sh && /install-others.sh && rm -f /install-others.sh
RUN chmod +x /configure-iptables-wrapper.sh && /configure-iptables-wrapper.sh && rm -f /configure-iptables-wrapper.sh

ARG TARGETPLATFORM
COPY --from=gops-cni-builder /out/${TARGETPLATFORM}/bin/gops /bin/gops

#test result
RUN chmod +x /test-image.sh && /test-image.sh && rm -f /test-image.sh


#============
FROM scratch
LABEL maintainer="maintainer@spidernet-io"
COPY --from=rootfs / /
