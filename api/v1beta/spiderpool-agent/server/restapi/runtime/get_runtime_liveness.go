// Code generated by go-swagger; DO NOT EDIT.

// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package runtime

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	"github.com/go-openapi/runtime/middleware"
)

// GetRuntimeLivenessHandlerFunc turns a function with the right signature into a get runtime liveness handler
type GetRuntimeLivenessHandlerFunc func(GetRuntimeLivenessParams) middleware.Responder

// Handle executing the request and returning a response
func (fn GetRuntimeLivenessHandlerFunc) Handle(params GetRuntimeLivenessParams) middleware.Responder {
	return fn(params)
}

// GetRuntimeLivenessHandler interface for that can handle valid get runtime liveness params
type GetRuntimeLivenessHandler interface {
	Handle(GetRuntimeLivenessParams) middleware.Responder
}

// NewGetRuntimeLiveness creates a new http.Handler for the get runtime liveness operation
func NewGetRuntimeLiveness(ctx *middleware.Context, handler GetRuntimeLivenessHandler) *GetRuntimeLiveness {
	return &GetRuntimeLiveness{Context: ctx, Handler: handler}
}

/* GetRuntimeLiveness swagger:route GET /runtime/liveness runtime getRuntimeLiveness

Liveness probe

Check pod liveness probe

*/
type GetRuntimeLiveness struct {
	Context *middleware.Context
	Handler GetRuntimeLivenessHandler
}

func (o *GetRuntimeLiveness) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, rCtx, _ := o.Context.RouteInfo(r)
	if rCtx != nil {
		*r = *rCtx
	}
	var Params = NewGetRuntimeLivenessParams()
	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params) // actually handle the request
	o.Context.Respond(rw, r, route.Produces, route, res)

}
