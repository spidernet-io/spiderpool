# Copyright 2020 Authors of Cilium
# SPDX-License-Identifier: Apache-2.0

include ../Makefile.defs

PUSH ?= false
ifeq ($(PUSH),true)
OUTPUT := "type=registry,push=true"
else
OUTPUT := "type=docker"
endif

#PLATFORM ?= linux/amd64,linux/arm64
PLATFORM ?= linux/amd64
IMAGE_REGISTRIES ?=

.PHONY: update-golang-image
update-golang-image:
	GO_VERSION=$(GO_VERSION) scripts/update-golang-image.sh

