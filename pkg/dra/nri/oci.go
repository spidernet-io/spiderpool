// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// this file is inspired by containerd:
// github.com/containerd/containerd/v2/pkg/oci/utils_unix.go
package nri

import (
	"errors"

	"github.com/containerd/nri/pkg/api"
	"golang.org/x/sys/unix"
)

const (
	wildcardDevice = "a" //nolint:nolintlint,unused,varcheck // currently unused, but should be included when upstreaming to OCI runtime-spec.
	blockDevice    = "b"
	charDevice     = "c" // or "u"
	fifoDevice     = "p"
)

// ErrNotADevice denotes that a file is not a valid linux device.
// When checking this error, use errors.Is(err, oci.ErrNotADevice)
var ErrNotADevice = errors.New("not a device node")

func DeviceFromPath(path string) (*api.LinuxDevice, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return nil, err
	}

	var (
		devNumber = uint64(stat.Rdev) //nolint:nolintlint,unconvert // the type is 32bit on mips.
		major     = unix.Major(devNumber)
		minor     = unix.Minor(devNumber)
	)

	var (
		devType string
		mode    = stat.Mode
	)

	switch mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = blockDevice
	case unix.S_IFCHR:
		devType = charDevice
	case unix.S_IFIFO:
		devType = fifoDevice
	default:
		return nil, ErrNotADevice
	}
	fm := api.OptionalFileMode{Value: mode &^ unix.S_IFMT}
	return &api.LinuxDevice{
		Type:     devType,
		Path:     path,
		Major:    int64(major),
		Minor:    int64(minor),
		FileMode: &fm,
		Uid:      &api.OptionalUInt32{Value: stat.Uid},
		Gid:      &api.OptionalUInt32{Value: stat.Gid},
	}, nil
}
