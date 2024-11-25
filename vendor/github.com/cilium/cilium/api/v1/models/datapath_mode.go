// Code generated by go-swagger; DO NOT EDIT.

// Copyright Authors of Cilium
// SPDX-License-Identifier: Apache-2.0

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"encoding/json"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

// DatapathMode Datapath mode
//
// swagger:model DatapathMode
type DatapathMode string

func NewDatapathMode(value DatapathMode) *DatapathMode {
	return &value
}

// Pointer returns a pointer to a freshly-allocated DatapathMode.
func (m DatapathMode) Pointer() *DatapathMode {
	return &m
}

const (

	// DatapathModeVeth captures enum value "veth"
	DatapathModeVeth DatapathMode = "veth"

	// DatapathModeNetkit captures enum value "netkit"
	DatapathModeNetkit DatapathMode = "netkit"

	// DatapathModeNetkitDashL2 captures enum value "netkit-l2"
	DatapathModeNetkitDashL2 DatapathMode = "netkit-l2"
)

// for schema
var datapathModeEnum []interface{}

func init() {
	var res []DatapathMode
	if err := json.Unmarshal([]byte(`["veth","netkit","netkit-l2"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		datapathModeEnum = append(datapathModeEnum, v)
	}
}

func (m DatapathMode) validateDatapathModeEnum(path, location string, value DatapathMode) error {
	if err := validate.EnumCase(path, location, value, datapathModeEnum, true); err != nil {
		return err
	}
	return nil
}

// Validate validates this datapath mode
func (m DatapathMode) Validate(formats strfmt.Registry) error {
	var res []error

	// value enum
	if err := m.validateDatapathModeEnum("", "body", m); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

// ContextValidate validates this datapath mode based on context it is used
func (m DatapathMode) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}
