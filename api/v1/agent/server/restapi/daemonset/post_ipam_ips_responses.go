// Code generated by go-swagger; DO NOT EDIT.

// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package daemonset

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
)

// PostIpamIpsOKCode is the HTTP code returned for type PostIpamIpsOK
const PostIpamIpsOKCode int = 200

/*
PostIpamIpsOK Success

swagger:response postIpamIpsOK
*/
type PostIpamIpsOK struct {
}

// NewPostIpamIpsOK creates PostIpamIpsOK with default headers values
func NewPostIpamIpsOK() *PostIpamIpsOK {

	return &PostIpamIpsOK{}
}

// WriteResponse to the client
func (o *PostIpamIpsOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(200)
}

// PostIpamIpsFailureCode is the HTTP code returned for type PostIpamIpsFailure
const PostIpamIpsFailureCode int = 500

/*
PostIpamIpsFailure Allocation failure

swagger:response postIpamIpsFailure
*/
type PostIpamIpsFailure struct {

	/*
	  In: Body
	*/
	Payload models.Error `json:"body,omitempty"`
}

// NewPostIpamIpsFailure creates PostIpamIpsFailure with default headers values
func NewPostIpamIpsFailure() *PostIpamIpsFailure {

	return &PostIpamIpsFailure{}
}

// WithPayload adds the payload to the post ipam ips failure response
func (o *PostIpamIpsFailure) WithPayload(payload models.Error) *PostIpamIpsFailure {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the post ipam ips failure response
func (o *PostIpamIpsFailure) SetPayload(payload models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *PostIpamIpsFailure) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	payload := o.Payload
	if err := producer.Produce(rw, payload); err != nil {
		panic(err) // let the recovery middleware deal with this
	}
}