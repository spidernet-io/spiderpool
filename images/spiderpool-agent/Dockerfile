# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

ARG BASE_IMAGE=ghcr.io/spidernet-io/spiderpool/spiderpool-base:163aca9e9d927363fa80aca7d9721b379671a790
ARG GOLANG_IMAGE=docker.io/library/golang:1.24.3@sha256:39d9e7d9c5d9c9e4baf0d8fff579f06d5032c0f4425cdec9e86732e8e4e374dc

#======= build bin ==========
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} AS builder

ARG TARGETOS
ARG TARGETARCH
ARG RACE
ARG NOSTRIP
ARG NOOPT
ARG QUIET_MAKE

COPY . /src
WORKDIR /src/cmd/spiderpool-agent
RUN  make GOARCH=${TARGETARCH}   \
        RACE=${RACE} NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        DESTDIR_BASH_COMPLETION=/tmp/install/${TARGETOS}/${TARGETARCH}/bash-completion \
        all install install-bash-completion

WORKDIR /src/cmd/spiderpool
RUN  make GOARCH=${TARGETARCH}   \
        RACE=${RACE} NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        DESTDIR_BASH_COMPLETION=/tmp/install/${TARGETOS}/${TARGETARCH}/bash-completion \
        all install install-bash-completion

WORKDIR /src/cmd/coordinator
RUN make GOARCH=${TARGETARCH}   \
        NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        all install

WORKDIR /src/cmd/ifacer
RUN make GOARCH=${TARGETARCH}   \
        NOSTRIP=${NOSTRIP} NOOPT=${NOOPT} QUIET_MAKE=${QUIET_MAKE} \
        DESTDIR_BIN=/tmp/install/${TARGETOS}/${TARGETARCH}/bin \
        all install

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

CMD ["/usr/bin/spiderpool-agent daemon"]
