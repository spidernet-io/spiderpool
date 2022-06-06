// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import "errors"

var ErrWrongInput = errors.New("input variable is not valid")
var ErrTimeOut = errors.New("context timeout")
var ErrChanelClosed = errors.New("channel is closed")
var ErrWatch = errors.New("failed to Watch")
var ErrEvent = errors.New("received error event")
var ErrResDel = errors.New("resource is deleted")
var ErrGetObj = errors.New("failed to get metaObject")
var ErrAlreadyExisted = errors.New("failed to create , a same Controller %v/%v exist")
