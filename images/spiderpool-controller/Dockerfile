# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0


ARG BASE_IMAGE=ghcr.io/spidernet-io/spiderpool/spiderpool-base:0e6ffc8aeb1b036af480df1974c24e739c0828d4@sha256:e2037963f6bc795326b88deca9954b9e3ee24940c9ac6589c1c4913e3a58d348
ARG GOLANG_IMAGE=docker.io/library/golang:1.18.2@sha256:02c05351ed076c581854c554fa65cb2eca47b4389fb79a1fc36f21b8df59c24f

# TARGETARCH is an automatic platform ARG enabled by Docker BuildKit.
# like amd64 arm64
ARG TARGETARCH

#======= build bin ==========
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} as builder

ARG TARGETOS
ARG TARGETARCH
ARG RACE
ARG NOSTRIP
ARG NOOPT
ARG QUIET_MAKE

COPY . /src
WORKDIR /src/cmd/spiderpool-controller
RUN  make GOARCH=${TARGETARCH}   \
        RACE=${RACE} NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        DESTDIR_BASH_COMPLETION=/tmp/install/${TARGETOS}/${TARGETARCH}/bash-completion \
        all install install-bash-completion

WORKDIR /src/cmd/spiderpoolctl
RUN  make GOARCH=${TARGETARCH}   \
        RACE=${RACE} NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        DESTDIR_BASH_COMPLETION=/tmp/install/${TARGETOS}/${TARGETARCH}/bash-completion \
        all install install-bash-completion


#====== release image =======

FROM ${BASE_IMAGE}

LABEL maintainer="maintainer@spidernet-io"

# TARGETOS is an automatic platform ARG enabled by Docker BuildKit.
ARG TARGETOS
# TARGETARCH is an automatic platform ARG enabled by Docker BuildKit.
ARG TARGETARCH

ARG GIT_COMMIT_VERSION
ENV GIT_COMMIT_VERSION=${GIT_COMMIT_VERSION}
ARG GIT_COMMIT_TIME
ENV GIT_COMMIT_TIME=${GIT_COMMIT_TIME}
ARG VERSION
ENV VERSION=${VERSION}

RUN groupadd -f spidernet \
    && echo ". /etc/profile.d/bash_completion.sh" >> /etc/bash.bashrc

COPY --from=builder /tmp/install/${TARGETOS}/${TARGETARCH}/bin/*   /usr/bin/
COPY --from=builder /tmp/install/${TARGETOS}/${TARGETARCH}/bash-completion/*  /etc/bash_completion.d/

CMD ["/usr/bin/spiderpool-controller daemon"]
