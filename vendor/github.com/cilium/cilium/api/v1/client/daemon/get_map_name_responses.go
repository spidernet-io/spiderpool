// Code generated by go-swagger; DO NOT EDIT.

// Copyright Authors of Cilium
// SPDX-License-Identifier: Apache-2.0

package daemon

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/cilium/cilium/api/v1/models"
)

// GetMapNameReader is a Reader for the GetMapName structure.
type GetMapNameReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetMapNameReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetMapNameOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 404:
		result := NewGetMapNameNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("[GET /map/{name}] GetMapName", response, response.Code())
	}
}

// NewGetMapNameOK creates a GetMapNameOK with default headers values
func NewGetMapNameOK() *GetMapNameOK {
	return &GetMapNameOK{}
}

/*
GetMapNameOK describes a response with status code 200, with default header values.

Success
*/
type GetMapNameOK struct {
	Payload *models.BPFMap
}

// IsSuccess returns true when this get map name o k response has a 2xx status code
func (o *GetMapNameOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get map name o k response has a 3xx status code
func (o *GetMapNameOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get map name o k response has a 4xx status code
func (o *GetMapNameOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get map name o k response has a 5xx status code
func (o *GetMapNameOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get map name o k response a status code equal to that given
func (o *GetMapNameOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the get map name o k response
func (o *GetMapNameOK) Code() int {
	return 200
}

func (o *GetMapNameOK) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /map/{name}][%d] getMapNameOK %s", 200, payload)
}

func (o *GetMapNameOK) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[GET /map/{name}][%d] getMapNameOK %s", 200, payload)
}

func (o *GetMapNameOK) GetPayload() *models.BPFMap {
	return o.Payload
}

func (o *GetMapNameOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.BPFMap)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetMapNameNotFound creates a GetMapNameNotFound with default headers values
func NewGetMapNameNotFound() *GetMapNameNotFound {
	return &GetMapNameNotFound{}
}

/*
GetMapNameNotFound describes a response with status code 404, with default header values.

Map not found
*/
type GetMapNameNotFound struct {
}

// IsSuccess returns true when this get map name not found response has a 2xx status code
func (o *GetMapNameNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this get map name not found response has a 3xx status code
func (o *GetMapNameNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get map name not found response has a 4xx status code
func (o *GetMapNameNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this get map name not found response has a 5xx status code
func (o *GetMapNameNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this get map name not found response a status code equal to that given
func (o *GetMapNameNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the get map name not found response
func (o *GetMapNameNotFound) Code() int {
	return 404
}

func (o *GetMapNameNotFound) Error() string {
	return fmt.Sprintf("[GET /map/{name}][%d] getMapNameNotFound", 404)
}

func (o *GetMapNameNotFound) String() string {
	return fmt.Sprintf("[GET /map/{name}][%d] getMapNameNotFound", 404)
}

func (o *GetMapNameNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
