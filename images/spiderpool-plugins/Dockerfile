# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

ARG GOLANG_IMAGE=docker.io/library/golang:1.20@sha256:f69d47fedd3b2ebd23bcf473c0b78522ebbc1823f06b7d47f45f04a30bdc901d

#======= build plugins ==========
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as builder
ARG TARGETOS
ARG TARGETARCH
ARG RDMA_VERSION
ARG OVS_VERSION
ARG CNI_VERSION
ARG IB_SRIOV_VERSION
ARG IPOIB_VERSION

WORKDIR /src

RUN touch VERSION.info && \
    printf "CNI_VERSION: %s\n" "$CNI_VERSION" >> VERSION.info && \
    printf "OVS_VERSION: %s\n" "$OVS_VERSION" >> VERSION.info && \
    printf "IB_SRIOV_VERSION: %s\n" "${IB_SRIOV_VERSION}" >> VERSION.info && \
    printf "IPOIB_VERSION: %s\n" "${IPOIB_VERSION}" >> VERSION.info && \
    printf "RDMA_COMMIT_HASH: %s\n" "$RDMA_VERSION" >> VERSION.info

RUN mkdir -p /src/cni/bin && \
    curl -L -O https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-${TARGETARCH}-${CNI_VERSION}.tgz && \
    tar -xvf cni-plugins-linux-${TARGETARCH}-${CNI_VERSION}.tgz -C /src/cni/bin/ && \
    echo "save cni-plguins: ${CNI_VERSION} done"

RUN git clone https://github.com/k8snetworkplumbingwg/rdma-cni.git
RUN git clone -b ${OVS_VERSION} --depth 1 https://github.com/k8snetworkplumbingwg/ovs-cni.git
RUN git clone https://github.com/k8snetworkplumbingwg/ib-sriov-cni.git
RUN git clone https://github.com/Mellanox/ipoib-cni.git

WORKDIR /src/rdma-cni
RUN git checkout ${RDMA_VERSION}  && make TARGET_ARCH=${TARGETARCH}   \
          TARGET_OS=${TARGETOS} build

WORKDIR /src/ovs-cni
RUN mkdir -p build && GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -tags no_openssl -mod vendor -o build/ovs ./cmd/plugin

WORKDIR /src/ib-sriov-cni
RUN git checkout ${IB_SRIOV_VERSION} && mkdir -p build && GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -tags no_openssl -mod vendor -o build/ib-sriov ./cmd/ib-sriov-cni

WORKDIR /src/ipoib-cni
RUN git checkout ${IPOIB_VERSION} &&  make build && [ -f "build/ipoib" ]

#===== release image =========

FROM alpine:3
LABEL maintainer="maintainer@spidernet-io"

# TARGETOS is an automatic platform ARG enabled by Docker BuildKit.
ARG TARGETOS
# TARGETARCH is an automatic platform ARG enabled by Docker BuildKit.
ARG TARGETARCH

ARG GIT_COMMIT_HASH
ENV GIT_COMMIT_HASH=${GIT_COMMIT_HASH}
ARG GIT_COMMIT_TIME
ENV GIT_COMMIT_TIME=${GIT_COMMIT_TIME}

WORKDIR /
COPY --from=builder /src/rdma-cni/build/rdma /usr/src/cni/rdma-cni/
COPY --from=builder /src/ovs-cni/build/ovs /usr/src/cni/ovs-cni/
COPY --from=builder /src/ib-sriov-cni/build/ib-sriov /usr/src/cni/ib-sriov/
COPY --from=builder /src/ipoib-cni/build/ipoib /usr/src/cni/ipoib/
COPY --from=builder /src/cni/bin/ /usr/src/cni/bin/
COPY --from=builder /src/VERSION.info /

ADD ./entrypoint.sh /