//
//Copyright The containerd Authors.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go-plugin. DO NOT EDIT.
// versions:
// 	protoc-gen-go-plugin v0.1.0
// 	protoc               v3.20.1
// source: pkg/api/api.proto

package api

import (
	context "context"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Plugin is the API NRI uses to interact with plugins. It is used to
// - configure a plugin and subscribe it for lifecycle events
// - synchronize the state of a plugin with that of the runtime
// - hook a plugin into the lifecycle events of its interest
//
// During configuration the plugin tells the runtime which lifecycle events
// it wishes to get hooked into. Once configured, the plugin is synchronized
// with the runtime by receiving the list of pods and containers known to
// the runtime. The plugin can request changes to any of the containers in
// response. After initial synchronization the plugin starts receiving the
// events it subscribed for as they occur in the runtime. For container
// creation, update, and stop events, the plugin can request changes, both
// to the container that triggered the event or any other existing container
// in the runtime.
//
// For a subset of the container lifecycle events, NRI defines an additional
// Post-variant of the event. These variants are defined for CreateContainer,
// StartContainer, and UpdateContainer. For creation and update, these events
// can be used by plugins to discover the full extent of changes applied to
// the container, including any changes made by other active plugins.
//
// go:plugin type=plugin version=1
type Plugin interface {
	// Configure the plugin and get its event subscription.
	Configure(context.Context, *ConfigureRequest) (*ConfigureResponse, error)
	// Synchronize the plugin with the state of the runtime.
	Synchronize(context.Context, *SynchronizeRequest) (*SynchronizeResponse, error)
	// Shutdown a plugin (let it know the runtime is going down).
	Shutdown(context.Context, *Empty) (*Empty, error)
	// CreateContainer relays the corresponding request to the plugin. In
	// response, the plugin can adjust the container being created, and
	// update other containers in the runtime. Container adjustment can
	// alter labels, annotations, mounts, devices, environment variables,
	// OCI hooks, and assigned container resources. Updates can alter
	// assigned container resources.
	CreateContainer(context.Context, *CreateContainerRequest) (*CreateContainerResponse, error)
	// UpdateContainer relays the corresponding request to the plugin.
	// The plugin can alter how the container is updated and request updates
	// to additional containers in the runtime.
	UpdateContainer(context.Context, *UpdateContainerRequest) (*UpdateContainerResponse, error)
	// StopContainer relays the corresponding request to the plugin. The plugin
	// can update any of the remaining containers in the runtime in response.
	StopContainer(context.Context, *StopContainerRequest) (*StopContainerResponse, error)
	// StateChange relays any remaining pod or container lifecycle/state change
	// events the plugin has subscribed for. These can be used to trigger any
	// plugin-specific processing which needs to occur in connection with any of
	// these events.
	StateChange(context.Context, *StateChangeEvent) (*Empty, error)
}

// go:plugin type=host
type HostFunctions interface {
	// Log displays a log message
	Log(context.Context, *LogRequest) (*Empty, error)
}
