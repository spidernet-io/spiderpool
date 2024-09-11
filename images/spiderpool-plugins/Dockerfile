# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

ARG GOLANG_IMAGE=docker.io/library/golang:1.23.1@sha256:4a3c2bcd243d3dbb7b15237eecb0792db3614900037998c2cd6a579c46888c1e

#======= build plugins ==========
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as builder
ARG TARGETOS
ARG TARGETARCH

ARG RDMA_VERSION
ENV RDMA_VERSION=${RDMA_VERSION}
ARG OVS_VERSION
ENV OVS_VERSION=${OVS_VERSION}
ARG CNI_VERSION
ENV CNI_VERSION=${CNI_VERSION}
ARG IB_SRIOV_VERSION
ENV IB_SRIOV_VERSION=${IB_SRIOV_VERSION}
ARG SRIOV_VERSION
ENV SRIOV_VERSION=${SRIOV_VERSION}
ARG IPOIB_VERSION
ENV IPOIB_VERSION=${IPOIB_VERSION}

WORKDIR /src

RUN touch VERSION.sh && \
    printf "export CNI_VERSION=%s\n" "${CNI_VERSION}" >> VERSION.sh && \
    printf "export OVS_VERSION=%s\n" "${OVS_VERSION}" >> VERSION.sh && \
    printf "export IB_SRIOV_VERSION=%s\n" "${IB_SRIOV_VERSION}" >> VERSION.sh && \
    printf "export SRIOV_VERSION=%s\n" "${SRIOV_VERSION}" >> VERSION.sh && \
    printf "export IPOIB_VERSION=%s\n" "${IPOIB_VERSION}" >> VERSION.sh && \
    printf "export RDMA_VERSION=%s\n" "${RDMA_VERSION}" >> VERSION.sh

RUN mkdir -p /src/cni/bin && \
    curl -L -O https://github.com/containernetworking/plugins/releases/download/${CNI_VERSION}/cni-plugins-linux-${TARGETARCH}-${CNI_VERSION}.tgz && \
    tar -xvf cni-plugins-linux-${TARGETARCH}-${CNI_VERSION}.tgz -C /src/cni/bin/ && \
    echo "save cni-plguins: ${CNI_VERSION} done"

RUN git clone https://github.com/k8snetworkplumbingwg/rdma-cni.git
RUN git clone -b ${OVS_VERSION} --depth 1 https://github.com/k8snetworkplumbingwg/ovs-cni.git
RUN git clone https://github.com/k8snetworkplumbingwg/ib-sriov-cni.git
RUN git clone https://github.com/k8snetworkplumbingwg/sriov-cni.git
RUN git clone https://github.com/Mellanox/ipoib-cni.git

WORKDIR /src/rdma-cni
RUN git checkout ${RDMA_VERSION}  && make TARGET_ARCH=${TARGETARCH}   \
          TARGET_OS=${TARGETOS} build

WORKDIR /src/ovs-cni
RUN mkdir -p build && GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -ldflags="-s -w" -tags no_openssl -mod vendor -o build/ovs ./cmd/plugin

WORKDIR /src/ib-sriov-cni
RUN git checkout ${IB_SRIOV_VERSION} && go mod vendor && mkdir -p build && GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -ldflags="-s -w" -tags no_openssl -mod vendor -o build/ib-sriov ./cmd/ib-sriov-cni

WORKDIR /src/sriov-cni
RUN git checkout ${SRIOV_VERSION} && go mod vendor && mkdir -p build && GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -v -ldflags="-s -w" -tags no_openssl -mod vendor -o build/sriov ./cmd/sriov

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

ENV RDMA_BIN_PATH="/usr/plugins/rdma"
ENV OVS_BIN_PATH="/usr/plugins/ovs"
ENV SRIOV_BIN_PATH="/usr/plugins/sriov"
ENV IB_SRIOV_BIN_PATH="/usr/plugins/ib-sriov"
ENV CNI_BIN_DIR="/usr/plugins/cni"
ENV IPOIB_BIN_PATH="/usr/plugins/ipoib"
ENV VERSION_FILE_PATH="/usr/plugins/VERSION.sh"

WORKDIR /
COPY --from=builder /src/rdma-cni/build/rdma ${RDMA_BIN_PATH}
COPY --from=builder /src/ovs-cni/build/ovs ${OVS_BIN_PATH}
COPY --from=builder /src/sriov-cni/build/sriov ${SRIOV_BIN_PATH}
COPY --from=builder /src/ib-sriov-cni/build/ib-sriov ${IB_SRIOV_BIN_PATH}
COPY --from=builder /src/ipoib-cni/build/ipoib ${IPOIB_BIN_PATH}
COPY --from=builder /src/cni/bin/ ${CNI_BIN_DIR}
COPY --from=builder /src/VERSION.sh ${VERSION_FILE_PATH}

ADD ./entrypoint.sh /