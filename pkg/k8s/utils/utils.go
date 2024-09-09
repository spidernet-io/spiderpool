// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DeleteWebhookConfiguration(ctx context.Context, c client.Client, name string, obj client.Object) error {
	err := c.Get(ctx, client.ObjectKey{Name: name}, obj)
	if err != nil {
		return err
	}

	err = c.Delete(ctx, obj)
	if err != nil {
		return err
	}

	return nil
}
