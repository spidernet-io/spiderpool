// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// IsRoute reports whether dst and gw strings constitute a route of the
// specified IP version.
func IsRoute(version types.IPVersion, dst, gw string) error {
	if err := IsIPVersion(version); err != nil {
		return err
	}

	if (version == constant.IPv4 && !IsIPv4Route(dst, gw)) ||
		(version == constant.IPv6 && !IsIPv6Route(dst, gw)) {
		return fmt.Errorf("%w in IPv%d 'dst: %s, gw: %s'", ErrInvalidRouteFormat, version, dst, gw)
	}

	return nil
}

// IsRouteWithoutIPVersion reports whether dst and gw strings constitute
// an route.
func IsRouteWithoutIPVersion(dst, gw string) error {
	if !IsIPv4Route(dst, gw) && !IsIPv6Route(dst, gw) {
		return fmt.Errorf("%w 'dst: %s, gw: %s'", ErrInvalidRouteFormat, dst, gw)
	}

	return nil
}

// IsIPv4Route reports whether dst and gw strings constitute an IPv4 route.
func IsIPv4Route(dst, gw string) bool {
	if IsIPv4CIDR(dst) && govalidator.IsIPv4(gw) {
		return true
	}

	return false
}

// IsIPv6Route reports whether dst and gw strings constitute an IPv6 route.
func IsIPv6Route(dst, gw string) bool {
	if IsIPv6CIDR(dst) && govalidator.IsIPv6(gw) {
		return true
	}

	return false
}
