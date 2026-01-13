// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	err := filepath.Walk("./vendor", func(path string, _ os.FileInfo, _ error) error {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		if stem := strings.TrimSuffix(base, ext); stem == "LICENSE" || stem == "COPYING" {
			switch strings.TrimPrefix(strings.ToLower(ext), ".") {
			case "", "code", "docs", "libyaml", "md", "txt":
				_, _ = fmt.Println("Name:", path)
				lb, err := os.ReadFile(path)
				if err != nil {
					log.Fatal(err)
				}
				_, _ = fmt.Println("License:", string(lb))
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
