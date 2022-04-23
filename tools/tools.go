//go:build tools
// +build tools

// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/gogo/protobuf/gogoproto" // Used for protobuf generation of pkg/k8s/types/slim/k8s
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
