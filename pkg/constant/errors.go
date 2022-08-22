// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"errors"
)

var (
	ErrInternal          = errors.New("internal server error")
	ErrWrongInput        = errors.New("wrong input")
	ErrNotAllocatablePod = errors.New("not allocatable Pod")
	ErrNoAvailablePool   = errors.New("no available IPPool")
	ErrIPUsedOut         = errors.New("all IP used out")
)
