// Code generated by go-swagger; DO NOT EDIT.

// Copyright Authors of Cilium
// SPDX-License-Identifier: Apache-2.0

package statedb

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/cilium/cilium/api/v1/models"
)

// GetStatedbQueryTableReader is a Reader for the GetStatedbQueryTable structure.
type GetStatedbQueryTableReader struct {
	formats strfmt.Registry
	writer  io.Writer
}

// ReadResponse reads a server response into the received o.
func (o *GetStatedbQueryTableReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetStatedbQueryTableOK(o.writer)
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewGetStatedbQueryTableBadRequest()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewGetStatedbQueryTableNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewGetStatedbQueryTableOK creates a GetStatedbQueryTableOK with default headers values
func NewGetStatedbQueryTableOK(writer io.Writer) *GetStatedbQueryTableOK {
	return &GetStatedbQueryTableOK{

		Payload: writer,
	}
}

/*
GetStatedbQueryTableOK describes a response with status code 200, with default header values.

Success
*/
type GetStatedbQueryTableOK struct {
	Payload io.Writer
}

// IsSuccess returns true when this get statedb query table o k response has a 2xx status code
func (o *GetStatedbQueryTableOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this get statedb query table o k response has a 3xx status code
func (o *GetStatedbQueryTableOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get statedb query table o k response has a 4xx status code
func (o *GetStatedbQueryTableOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this get statedb query table o k response has a 5xx status code
func (o *GetStatedbQueryTableOK) IsServerError() bool {
	return false
}

// IsCode returns true when this get statedb query table o k response a status code equal to that given
func (o *GetStatedbQueryTableOK) IsCode(code int) bool {
	return code == 200
}

func (o *GetStatedbQueryTableOK) Error() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableOK  %+v", 200, o.Payload)
}

func (o *GetStatedbQueryTableOK) String() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableOK  %+v", 200, o.Payload)
}

func (o *GetStatedbQueryTableOK) GetPayload() io.Writer {
	return o.Payload
}

func (o *GetStatedbQueryTableOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetStatedbQueryTableBadRequest creates a GetStatedbQueryTableBadRequest with default headers values
func NewGetStatedbQueryTableBadRequest() *GetStatedbQueryTableBadRequest {
	return &GetStatedbQueryTableBadRequest{}
}

/*
GetStatedbQueryTableBadRequest describes a response with status code 400, with default header values.

Invalid parameters
*/
type GetStatedbQueryTableBadRequest struct {
	Payload models.Error
}

// IsSuccess returns true when this get statedb query table bad request response has a 2xx status code
func (o *GetStatedbQueryTableBadRequest) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this get statedb query table bad request response has a 3xx status code
func (o *GetStatedbQueryTableBadRequest) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get statedb query table bad request response has a 4xx status code
func (o *GetStatedbQueryTableBadRequest) IsClientError() bool {
	return true
}

// IsServerError returns true when this get statedb query table bad request response has a 5xx status code
func (o *GetStatedbQueryTableBadRequest) IsServerError() bool {
	return false
}

// IsCode returns true when this get statedb query table bad request response a status code equal to that given
func (o *GetStatedbQueryTableBadRequest) IsCode(code int) bool {
	return code == 400
}

func (o *GetStatedbQueryTableBadRequest) Error() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableBadRequest  %+v", 400, o.Payload)
}

func (o *GetStatedbQueryTableBadRequest) String() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableBadRequest  %+v", 400, o.Payload)
}

func (o *GetStatedbQueryTableBadRequest) GetPayload() models.Error {
	return o.Payload
}

func (o *GetStatedbQueryTableBadRequest) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetStatedbQueryTableNotFound creates a GetStatedbQueryTableNotFound with default headers values
func NewGetStatedbQueryTableNotFound() *GetStatedbQueryTableNotFound {
	return &GetStatedbQueryTableNotFound{}
}

/*
GetStatedbQueryTableNotFound describes a response with status code 404, with default header values.

Table or Index not found
*/
type GetStatedbQueryTableNotFound struct {
}

// IsSuccess returns true when this get statedb query table not found response has a 2xx status code
func (o *GetStatedbQueryTableNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this get statedb query table not found response has a 3xx status code
func (o *GetStatedbQueryTableNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this get statedb query table not found response has a 4xx status code
func (o *GetStatedbQueryTableNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this get statedb query table not found response has a 5xx status code
func (o *GetStatedbQueryTableNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this get statedb query table not found response a status code equal to that given
func (o *GetStatedbQueryTableNotFound) IsCode(code int) bool {
	return code == 404
}

func (o *GetStatedbQueryTableNotFound) Error() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableNotFound ", 404)
}

func (o *GetStatedbQueryTableNotFound) String() string {
	return fmt.Sprintf("[GET /statedb/query/{table}][%d] getStatedbQueryTableNotFound ", 404)
}

func (o *GetStatedbQueryTableNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
