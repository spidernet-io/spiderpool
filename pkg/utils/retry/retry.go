// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package retry

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

var DefaultRetry = wait.Backoff{
	Steps:    5,
	Duration: 10 * time.Millisecond,
	Factor:   1.5,
	Jitter:   0.1,
}

var DefaultBackoff = wait.Backoff{
	Steps:    4,
	Duration: 10 * time.Millisecond,
	Factor:   5.0,
	Jitter:   0.1,
}

func OnErrorWithContext(ctx context.Context, backoff wait.Backoff, retriable func(error) bool, f func(context.Context) error) error {
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(context.Context) (bool, error) {
		err := f(ctx)
		switch {
		case err == nil:
			return true, nil
		case retriable(err):
			return false, nil
		default:
			return false, err
		}
	})
	return err
}

func RetryOnConflictWithContext(ctx context.Context, backoff wait.Backoff, f func(context.Context) error) error {
	return OnErrorWithContext(ctx, backoff, apierrors.IsConflict, f)
}
