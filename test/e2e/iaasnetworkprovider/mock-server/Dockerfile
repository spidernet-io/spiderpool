# Copyright 2026 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

FROM docker.m.daocloud.io/golang:1.25-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/iaas-provider-mock ./test/e2e/iaasnetworkprovider/mock-server

FROM docker.m.daocloud.io/alpine:3.22
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /out/iaas-provider-mock /usr/local/bin/iaas-provider-mock
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/iaas-provider-mock"]
