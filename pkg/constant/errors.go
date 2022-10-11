// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package constant

import (
	"errors"
)

var (
	ErrWrongInput       = errors.New("wrong input")
	ErrNoAvailablePool  = errors.New("no IPPool available")
	ErrRetriesExhausted = errors.New("insufficient retries")
	ErrIPUsedOut        = errors.New("all IP addresses used out")
)
