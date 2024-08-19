// Code generated by go-swagger; DO NOT EDIT.

// Copyright Authors of Cilium
// SPDX-License-Identifier: Apache-2.0

package endpoint

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/cilium/cilium/api/v1/models"
)

// PatchEndpointIDConfigReader is a Reader for the PatchEndpointIDConfig structure.
type PatchEndpointIDConfigReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *PatchEndpointIDConfigReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewPatchEndpointIDConfigOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := NewPatchEndpointIDConfigInvalid()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := NewPatchEndpointIDConfigForbidden()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := NewPatchEndpointIDConfigNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 429:
		result := NewPatchEndpointIDConfigTooManyRequests()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 500:
		result := NewPatchEndpointIDConfigFailed()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("[PATCH /endpoint/{id}/config] PatchEndpointIDConfig", response, response.Code())
	}
}

// NewPatchEndpointIDConfigOK creates a PatchEndpointIDConfigOK with default headers values
func NewPatchEndpointIDConfigOK() *PatchEndpointIDConfigOK {
	return &PatchEndpointIDConfigOK{}
}

/*
PatchEndpointIDConfigOK describes a response with status code 200, with default header values.

Success
*/
type PatchEndpointIDConfigOK struct {
}

// IsSuccess returns true when this patch endpoint Id config o k response has a 2xx status code
func (o *PatchEndpointIDConfigOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this patch endpoint Id config o k response has a 3xx status code
func (o *PatchEndpointIDConfigOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config o k response has a 4xx status code
func (o *PatchEndpointIDConfigOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this patch endpoint Id config o k response has a 5xx status code
func (o *PatchEndpointIDConfigOK) IsServerError() bool {
	return false
}

// IsCode returns true when this patch endpoint Id config o k response a status code equal to that given
func (o *PatchEndpointIDConfigOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the patch endpoint Id config o k response
func (o *PatchEndpointIDConfigOK) Code() int {
	return 200
}

func (o *PatchEndpointIDConfigOK) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigOK ", 200)
}

func (o *PatchEndpointIDConfigOK) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigOK ", 200)
}

func (o *PatchEndpointIDConfigOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPatchEndpointIDConfigInvalid creates a PatchEndpointIDConfigInvalid with default headers values
func NewPatchEndpointIDConfigInvalid() *PatchEndpointIDConfigInvalid {
	return &PatchEndpointIDConfigInvalid{}
}

/*
PatchEndpointIDConfigInvalid describes a response with status code 400, with default header values.

Invalid configuration request
*/
type PatchEndpointIDConfigInvalid struct {
}

// IsSuccess returns true when this patch endpoint Id config invalid response has a 2xx status code
func (o *PatchEndpointIDConfigInvalid) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this patch endpoint Id config invalid response has a 3xx status code
func (o *PatchEndpointIDConfigInvalid) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config invalid response has a 4xx status code
func (o *PatchEndpointIDConfigInvalid) IsClientError() bool {
	return true
}

// IsServerError returns true when this patch endpoint Id config invalid response has a 5xx status code
func (o *PatchEndpointIDConfigInvalid) IsServerError() bool {
	return false
}

// IsCode returns true when this patch endpoint Id config invalid response a status code equal to that given
func (o *PatchEndpointIDConfigInvalid) IsCode(code int) bool {
	return code == 400
}

// Code gets the status code for the patch endpoint Id config invalid response
func (o *PatchEndpointIDConfigInvalid) Code() int {
	return 400
}

func (o *PatchEndpointIDConfigInvalid) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigInvalid ", 400)
}

func (o *PatchEndpointIDConfigInvalid) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigInvalid ", 400)
}

func (o *PatchEndpointIDConfigInvalid) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPatchEndpointIDConfigForbidden creates a PatchEndpointIDConfigForbidden with default headers values
func NewPatchEndpointIDConfigForbidden() *PatchEndpointIDConfigForbidden {
	return &PatchEndpointIDConfigForbidden{}
}

/*
PatchEndpointIDConfigForbidden describes a response with status code 403, with default header values.

Forbidden
*/
type PatchEndpointIDConfigForbidden struct {
}

// IsSuccess returns true when this patch endpoint Id config forbidden response has a 2xx status code
func (o *PatchEndpointIDConfigForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this patch endpoint Id config forbidden response has a 3xx status code
func (o *PatchEndpointIDConfigForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config forbidden response has a 4xx status code
func (o *PatchEndpointIDConfigForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this patch endpoint Id config forbidden response has a 5xx status code
func (o *PatchEndpointIDConfigForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this patch endpoint Id config forbidden response a status code equal to that given
func (o *PatchEndpointIDConfigForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the patch endpoint Id config forbidden response
func (o *PatchEndpointIDConfigForbidden) Code() int {
	return 403
}

func (o *PatchEndpointIDConfigForbidden) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigForbidden ", 403)
}

func (o *PatchEndpointIDConfigForbidden) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigForbidden ", 403)
}

func (o *PatchEndpointIDConfigForbidden) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPatchEndpointIDConfigNotFound creates a PatchEndpointIDConfigNotFound with default headers values
func NewPatchEndpointIDConfigNotFound() *PatchEndpointIDConfigNotFound {
	return &PatchEndpointIDConfigNotFound{}
}

/*
PatchEndpointIDConfigNotFound describes a response with status code 404, with default header values.

Endpoint not found
*/
type PatchEndpointIDConfigNotFound struct {
}

// IsSuccess returns true when this patch endpoint Id config not found response has a 2xx status code
func (o *PatchEndpointIDConfigNotFound) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this patch endpoint Id config not found response has a 3xx status code
func (o *PatchEndpointIDConfigNotFound) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config not found response has a 4xx status code
func (o *PatchEndpointIDConfigNotFound) IsClientError() bool {
	return true
}

// IsServerError returns true when this patch endpoint Id config not found response has a 5xx status code
func (o *PatchEndpointIDConfigNotFound) IsServerError() bool {
	return false
}

// IsCode returns true when this patch endpoint Id config not found response a status code equal to that given
func (o *PatchEndpointIDConfigNotFound) IsCode(code int) bool {
	return code == 404
}

// Code gets the status code for the patch endpoint Id config not found response
func (o *PatchEndpointIDConfigNotFound) Code() int {
	return 404
}

func (o *PatchEndpointIDConfigNotFound) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigNotFound ", 404)
}

func (o *PatchEndpointIDConfigNotFound) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigNotFound ", 404)
}

func (o *PatchEndpointIDConfigNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPatchEndpointIDConfigTooManyRequests creates a PatchEndpointIDConfigTooManyRequests with default headers values
func NewPatchEndpointIDConfigTooManyRequests() *PatchEndpointIDConfigTooManyRequests {
	return &PatchEndpointIDConfigTooManyRequests{}
}

/*
PatchEndpointIDConfigTooManyRequests describes a response with status code 429, with default header values.

Rate-limiting too many requests in the given time frame
*/
type PatchEndpointIDConfigTooManyRequests struct {
}

// IsSuccess returns true when this patch endpoint Id config too many requests response has a 2xx status code
func (o *PatchEndpointIDConfigTooManyRequests) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this patch endpoint Id config too many requests response has a 3xx status code
func (o *PatchEndpointIDConfigTooManyRequests) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config too many requests response has a 4xx status code
func (o *PatchEndpointIDConfigTooManyRequests) IsClientError() bool {
	return true
}

// IsServerError returns true when this patch endpoint Id config too many requests response has a 5xx status code
func (o *PatchEndpointIDConfigTooManyRequests) IsServerError() bool {
	return false
}

// IsCode returns true when this patch endpoint Id config too many requests response a status code equal to that given
func (o *PatchEndpointIDConfigTooManyRequests) IsCode(code int) bool {
	return code == 429
}

// Code gets the status code for the patch endpoint Id config too many requests response
func (o *PatchEndpointIDConfigTooManyRequests) Code() int {
	return 429
}

func (o *PatchEndpointIDConfigTooManyRequests) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigTooManyRequests ", 429)
}

func (o *PatchEndpointIDConfigTooManyRequests) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigTooManyRequests ", 429)
}

func (o *PatchEndpointIDConfigTooManyRequests) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewPatchEndpointIDConfigFailed creates a PatchEndpointIDConfigFailed with default headers values
func NewPatchEndpointIDConfigFailed() *PatchEndpointIDConfigFailed {
	return &PatchEndpointIDConfigFailed{}
}

/*
PatchEndpointIDConfigFailed describes a response with status code 500, with default header values.

Update failed. Details in message.
*/
type PatchEndpointIDConfigFailed struct {
	Payload models.Error
}

// IsSuccess returns true when this patch endpoint Id config failed response has a 2xx status code
func (o *PatchEndpointIDConfigFailed) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this patch endpoint Id config failed response has a 3xx status code
func (o *PatchEndpointIDConfigFailed) IsRedirect() bool {
	return false
}

// IsClientError returns true when this patch endpoint Id config failed response has a 4xx status code
func (o *PatchEndpointIDConfigFailed) IsClientError() bool {
	return false
}

// IsServerError returns true when this patch endpoint Id config failed response has a 5xx status code
func (o *PatchEndpointIDConfigFailed) IsServerError() bool {
	return true
}

// IsCode returns true when this patch endpoint Id config failed response a status code equal to that given
func (o *PatchEndpointIDConfigFailed) IsCode(code int) bool {
	return code == 500
}

// Code gets the status code for the patch endpoint Id config failed response
func (o *PatchEndpointIDConfigFailed) Code() int {
	return 500
}

func (o *PatchEndpointIDConfigFailed) Error() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigFailed  %+v", 500, o.Payload)
}

func (o *PatchEndpointIDConfigFailed) String() string {
	return fmt.Sprintf("[PATCH /endpoint/{id}/config][%d] patchEndpointIdConfigFailed  %+v", 500, o.Payload)
}

func (o *PatchEndpointIDConfigFailed) GetPayload() models.Error {
	return o.Payload
}

func (o *PatchEndpointIDConfigFailed) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
