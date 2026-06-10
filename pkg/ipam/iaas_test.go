// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IaaS ownership", Label("ipam_iaas_test"), func() {
	It("keeps IaaS release ownership in IPAM instead of the ENI device plugin", func() {
		fset := token.NewFileSet()
		ipamFile, err := parser.ParseFile(fset, "iaas.go", nil, parser.ParseComments)
		Expect(err).NotTo(HaveOccurred())

		hasRelease := false
		for _, decl := range ipamFile.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "callIaaSRelease" {
				hasRelease = true
				break
			}
		}
		Expect(hasRelease).To(BeTrue())

		serverSource, err := os.ReadFile(filepath.Join("..", "enislotdeviceplugin", "server.go"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(serverSource)).NotTo(ContainSubstring("pkg/iaas"))
	})
})
