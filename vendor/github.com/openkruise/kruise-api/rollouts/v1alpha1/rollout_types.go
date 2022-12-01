/*
Copyright 2022 The Kruise Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RolloutSpec defines the desired state of Rollout
type RolloutSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// ObjectRef indicates workload
	ObjectRef ObjectRef `json:"objectRef"`
	// rollout strategy
	Strategy RolloutStrategy `json:"strategy"`
	// RolloutID should be changed before each workload revision publication.
	// It is to distinguish consecutive multiple workload publications and rollout progress.
	RolloutID string `json:"rolloutID,omitempty"`
}

type ObjectRef struct {
	// WorkloadRef contains enough information to let you identify a workload for Rollout
	// Batch release of the bypass
	WorkloadRef *WorkloadRef `json:"workloadRef,omitempty"`

	// revisionRef
	// Fully managed batch publishing capability
	//RevisionRef *ControllerRevisionRef `json:"revisionRef,omitempty"`
}

type ObjectRefType string

const (
	WorkloadRefType = "workloadRef"
	RevisionRefType = "revisionRef"
)

// WorkloadRef holds a references to the Kubernetes object
type WorkloadRef struct {
	// API Version of the referent
	APIVersion string `json:"apiVersion"`
	// Kind of the referent
	Kind string `json:"kind"`
	// Name of the referent
	Name string `json:"name"`
}

/*type ControllerRevisionRef struct {
	TargetRevisionName string `json:"targetRevisionName"`
	SourceRevisionName string `json:"sourceRevisionName"`
}*/

// RolloutStrategy defines strategy to apply during next rollout
type RolloutStrategy struct {
	// Paused indicates that the Rollout is paused.
	// Default value is false
	Paused bool `json:"paused,omitempty"`
	// +optional
	Canary *CanaryStrategy `json:"canary,omitempty"`
	// +optional
	// BlueGreen *BlueGreenStrategy `json:"blueGreen,omitempty"`
}

type RolloutStrategyType string

const (
	RolloutStrategyCanary    RolloutStrategyType = "canary"
	RolloutStrategyBlueGreen RolloutStrategyType = "blueGreen"
)

// CanaryStrategy defines parameters for a Replica Based Canary
type CanaryStrategy struct {
	// Steps define the order of phases to execute release in batches(20%, 40%, 60%, 80%, 100%)
	// +optional
	Steps []CanaryStep `json:"steps,omitempty"`
	// TrafficRoutings hosts all the supported service meshes supported to enable more fine-grained traffic routing
	// todo current only support one TrafficRouting
	TrafficRoutings []*TrafficRouting `json:"trafficRoutings,omitempty"`
	// MetricsAnalysis *MetricsAnalysisBackground `json:"metricsAnalysis,omitempty"`
}

// CanaryStep defines a step of a canary workload.
type CanaryStep struct {
	// SetWeight sets what percentage of the canary pods should receive

	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Weight *int32 `json:"weight,omitempty"`
	// Replicas is the number of expected canary pods in this batch
	// it can be an absolute number (ex: 5) or a percentage of total pods.
	Replicas *intstr.IntOrString `json:"replicas,omitempty"`
	// Pause defines a pause stage for a rollout, manual or auto
	// +optional
	Pause RolloutPause `json:"pause,omitempty"`
	// MetricsAnalysis *RolloutAnalysis `json:"metricsAnalysis,omitempty"`
}

// RolloutPause defines a pause stage for a rollout
type RolloutPause struct {
	// Duration the amount of time to wait before moving to the next step.
	// +optional
	// +kubebuilder:validation:Minimum=0
	Duration *int32 `json:"duration,omitempty"`
}

// TrafficRouting hosts all the different configuration for supported service meshes to enable more fine-grained traffic routing
type TrafficRouting struct {
	// Service holds the name of a service which selects pods with stable version and don't select any pods with canary version.
	Service string `json:"service"`
	// Optional duration in seconds the traffic provider(e.g. nginx ingress controller) consumes the service, ingress configuration changes gracefully.
	// +kubebuilder:validation:Minimum=0
	GracePeriodSeconds int32 `json:"gracePeriodSeconds,omitempty"`
	// Ingress holds Ingress specific configuration to route traffic, e.g. Nginx, Alb.
	Ingress *IngressTrafficRouting `json:"ingress,omitempty"`
	// Gateway holds Gateway specific configuration to route traffic
	// Gateway configuration only supports >= v0.4.0 (v1alpha2).
	Gateway *GatewayTrafficRouting `json:"gateway,omitempty"`
}

// IngressTrafficRouting configuration for ingress controller to control traffic routing
type IngressTrafficRouting struct {
	// ClassType refers to the class type of an `Ingress`, e.g. Nginx. Default is Nginx
	// +optional
	ClassType string `json:"classType,omitempty"`
	// Name refers to the name of an `Ingress` resource in the same namespace as the `Rollout`
	Name string `json:"name"`
}

// GatewayTrafficRouting configuration for gateway api
type GatewayTrafficRouting struct {
	// HTTPRouteName refers to the name of an `HTTPRoute` resource in the same namespace as the `Rollout`
	HTTPRouteName *string `json:"httpRouteName,omitempty"`
	// TCPRouteName *string `json:"tcpRouteName,omitempty"`
	// UDPRouteName *string `json:"udpRouteName,omitempty"`
}

// RolloutStatus defines the observed state of Rollout
type RolloutStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// observedGeneration is the most recent generation observed for this Rollout.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// CanaryRevision the hash of the canary pod template
	// +optional
	//CanaryRevision string `json:"canaryRevision,omitempty"`
	// StableRevision indicates the revision pods that has successfully rolled out
	StableRevision string `json:"stableRevision,omitempty"`
	// Conditions a list of conditions a rollout can have.
	// +optional
	Conditions []RolloutCondition `json:"conditions,omitempty"`
	// Canary describes the state of the canary rollout
	// +optional
	CanaryStatus *CanaryStatus `json:"canaryStatus,omitempty"`
	// +optional
	//BlueGreenStatus *BlueGreenStatus `json:"blueGreenStatus,omitempty"`
	// Phase is the rollout phase.
	Phase RolloutPhase `json:"phase,omitempty"`
	// Message provides details on why the rollout is in its current phase
	Message string `json:"message,omitempty"`
}

// RolloutCondition describes the state of a rollout at a certain point.
type RolloutCondition struct {
	// Type of rollout condition.
	Type RolloutConditionType `json:"type"`
	// Phase of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

// RolloutConditionType defines the conditions of Rollout
type RolloutConditionType string

// These are valid conditions of a rollout.
const (
	// RolloutConditionProgressing means the rollout is progressing. Progress for a rollout is
	// considered when a new replica set is created or adopted, when pods scale
	// up or old pods scale down, or when the services are updated. Progress is not estimated
	// for paused rollouts.
	RolloutConditionProgressing RolloutConditionType = "Progressing"
	// Progressing Reason
	ProgressingReasonInitializing = "Initializing"
	ProgressingReasonInRolling    = "InRolling"
	ProgressingReasonFinalising   = "Finalising"
	ProgressingReasonSucceeded    = "Succeeded"
	ProgressingReasonCancelling   = "Cancelling"
	ProgressingReasonCanceled     = "Canceled"
	ProgressingReasonPaused       = "Paused"

	// Terminating condition
	RolloutConditionTerminating RolloutConditionType = "Terminating"
	// Terminating Reason
	TerminatingReasonInTerminating = "InTerminating"
	TerminatingReasonCompleted     = "Completed"
)

// CanaryStatus status fields that only pertain to the canary rollout
type CanaryStatus struct {
	// observedWorkloadGeneration is the most recent generation observed for this Rollout ref workload generation.
	ObservedWorkloadGeneration int64 `json:"observedWorkloadGeneration,omitempty"`
	// ObservedRolloutID will record the newest spec.RolloutID if status.canaryRevision equals to workload.updateRevision
	ObservedRolloutID string `json:"observedRolloutID,omitempty"`
	// RolloutHash from rollout.spec object
	RolloutHash string `json:"rolloutHash,omitempty"`
	// CanaryService holds the name of a service which selects pods with canary version and don't select any pods with stable version.
	CanaryService string `json:"canaryService"`
	// CanaryRevision is calculated by rollout based on podTemplateHash, and the internal logic flow uses
	// It may be different from rs podTemplateHash in different k8s versions, so it cannot be used as service selector label
	// +optional
	CanaryRevision string `json:"canaryRevision"`
	// pod template hash is used as service selector label
	PodTemplateHash string `json:"podTemplateHash"`
	// CanaryReplicas the numbers of canary revision pods
	CanaryReplicas int32 `json:"canaryReplicas"`
	// CanaryReadyReplicas the numbers of ready canary revision pods
	CanaryReadyReplicas int32 `json:"canaryReadyReplicas"`
	// CurrentStepIndex defines the current step of the rollout is on. If the current step index is null, the
	// controller will execute the rollout.
	// +optional
	CurrentStepIndex int32           `json:"currentStepIndex"`
	CurrentStepState CanaryStepState `json:"currentStepState"`
	Message          string          `json:"message,omitempty"`
	LastUpdateTime   *metav1.Time    `json:"lastUpdateTime,omitempty"`
}

type CanaryStepState string

const (
	CanaryStepStateUpgrade         CanaryStepState = "StepUpgrade"
	CanaryStepStateTrafficRouting  CanaryStepState = "StepTrafficRouting"
	CanaryStepStateMetricsAnalysis CanaryStepState = "StepMetricsAnalysis"
	CanaryStepStatePaused          CanaryStepState = "StepPaused"
	CanaryStepStateReady           CanaryStepState = "StepReady"
	CanaryStepStateCompleted       CanaryStepState = "Completed"
)

// RolloutPhase are a set of phases that this rollout
type RolloutPhase string

const (
	// RolloutPhaseInitial indicates a rollout is Initial
	RolloutPhaseInitial RolloutPhase = "Initial"
	// RolloutPhaseHealthy indicates a rollout is healthy
	RolloutPhaseHealthy RolloutPhase = "Healthy"
	// RolloutPhasePreparing indicates a rollout is preparing for next progress.
	RolloutPhasePreparing RolloutPhase = "Preparing"
	// RolloutPhaseProgressing indicates a rollout is not yet healthy but still making progress towards a healthy state
	RolloutPhaseProgressing RolloutPhase = "Progressing"
	// RolloutPhaseFinalizing indicates a rollout is finalizing
	RolloutPhaseFinalizing RolloutPhase = "Finalizing"
	// RolloutPhaseTerminating indicates a rollout is terminated
	RolloutPhaseTerminating RolloutPhase = "Terminating"
	// RolloutPhaseCompleted indicates a rollout is completed
	RolloutPhaseCompleted RolloutPhase = "Completed"
	// RolloutPhaseCancelled indicates a rollout is cancelled
	RolloutPhaseCancelled RolloutPhase = "Cancelled"
)

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.phase",description="The rollout status phase"
// +kubebuilder:printcolumn:name="CANARY_STEP",type="integer",JSONPath=".status.canaryStatus.currentStepIndex",description="The rollout canary status step"
// +kubebuilder:printcolumn:name="CANARY_STATE",type="string",JSONPath=".status.canaryStatus.currentStepState",description="The rollout canary status step state"
// +kubebuilder:printcolumn:name="MESSAGE",type="string",JSONPath=".status.message",description="The rollout canary status message"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp"

// Rollout is the Schema for the rollouts API
type Rollout struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RolloutSpec   `json:"spec,omitempty"`
	Status RolloutStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RolloutList contains a list of Rollout
type RolloutList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rollout `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rollout{}, &RolloutList{})
}
