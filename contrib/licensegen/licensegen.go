// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	IsCheckLicense bool

	targetPath      = "./api"
	licenseFilePath = "./hack/spdx-copyright-header.txt"
)

func main() {
	if IsCheckLicense {
		licenseCheck()
	}

	err := LicenseAdd(targetPath, licenseFilePath, false)
	if nil != err {
		log.Fatal(err)
	}
}

func licenseCheck() {
	err := filepath.Walk("./vendor", func(path string, _ os.FileInfo, _ error) error {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		if stem := strings.TrimSuffix(base, ext); stem == "LICENSE" || stem == "COPYING" {
			switch strings.TrimPrefix(strings.ToLower(ext), ".") {
			case "", "code", "docs", "libyaml", "md", "txt":
				fmt.Println("Name:", path)
				lb, err := os.ReadFile(path)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("License:", string(lb))
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

// LicenseAdd will help you find and add license header for *.go files.
// The first param is the dir that you wanna check, the second param is the license path,
// the third param defines whether you want to add license header for them.
func LicenseAdd(targetPath, licenseFilePath string, addCopy bool) error {
	licenseFile, err := os.Open(licenseFilePath)
	if nil != err {
		return err
	}
	licenseReader := bufio.NewReader(licenseFile)
	var li []byte
	for {
		line, err := licenseReader.ReadString('\n')
		if nil != err || err == io.EOF {
			if line == "" {
				break
			}
		}
		li = append(li, []byte("// "+line)...)
	}

	var noLicenseFileCount int
	err = filepath.Walk(targetPath, func(path string, _ fs.FileInfo, _ error) error {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		if ext == ".go" {
			body, err := os.ReadFile(path)
			if nil != err {
				return err
			}
			if !bytes.Contains(body, li) {
				noLicenseFileCount++
				fmt.Println("Found no license header file:", path)
				if addCopy {
					f, err := os.OpenFile(path, os.O_WRONLY, 0644)
					if nil != err {
						return err
					}
					seek, err := f.Seek(0, 0)
					if nil != err {
						return err
					}
					res := append(li, '\n', '\n')
					body = append(res, body...)
					_, err = f.WriteAt(body, seek)
					if nil != err {
						return err
					}
				}
			}
		}
		return nil
	})
	if nil != err {
		return err
	}

	if noLicenseFileCount == 0 {
		fmt.Printf("All dir %s files have license header.", targetPath)
	}
	return nil
}
