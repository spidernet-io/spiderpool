// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import "errors"

var ErrWrongInput = errors.New("input variable is not valid")
var ErrTimeOut = errors.New("context timeout")
