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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ReleaseStrategyType defines strategies for pods rollout
type ReleaseStrategyType string

// ReleasePlan fines the details of the release plan
type ReleasePlan struct {
	// Batches is the details on each batch of the ReleasePlan.
	//Users can specify their batch plan in this field, such as:
	// batches:
	// - canaryReplicas: 1  # batches 0
	// - canaryReplicas: 2  # batches 1
	// - canaryReplicas: 5  # batches 2
	// Not that these canaryReplicas should be a non-decreasing sequence.
	// +optional
	Batches []ReleaseBatch `json:"batches"`
	// All pods in the batches up to the batchPartition (included) will have
	// the target resource specification while the rest still is the stable revision.
	// This is designed for the operators to manually rollout
	// Default is nil, which means no partition and will release all batches.
	// BatchPartition start from 0.
	// +optional
	BatchPartition *int32 `json:"batchPartition,omitempty"`
}

// ReleaseBatch is used to describe how each batch release should be
type ReleaseBatch struct {
	// CanaryReplicas is the number of upgraded pods that should have in this batch.
	// it can be an absolute number (ex: 5) or a percentage of workload replicas.
	// batches[i].canaryReplicas should less than or equal to batches[j].canaryReplicas if i < j.
	CanaryReplicas intstr.IntOrString `json:"canaryReplicas"`
	// The wait time, in seconds, between instances batches, default = 0
	// +optional
	PauseSeconds int64 `json:"pauseSeconds,omitempty"`
}

// BatchReleaseStatus defines the observed state of a release plan
type BatchReleaseStatus struct {
	// Conditions represents the observed process state of each phase during executing the release plan.
	Conditions []RolloutCondition `json:"conditions,omitempty"`
	// CanaryStatus describes the state of the canary rollout.
	CanaryStatus BatchReleaseCanaryStatus `json:"canaryStatus,omitempty"`
	// StableRevision is the pod-template-hash of stable revision pod template.
	StableRevision string `json:"stableRevision,omitempty"`
	// UpdateRevision is the pod-template-hash of update revision pod template.
	UpdateRevision string `json:"updateRevision,omitempty"`
	// ObservedGeneration is the most recent generation observed for this BatchRelease.
	// It corresponds to this BatchRelease's generation, which is updated on mutation
	// by the API Server, and only if BatchRelease Spec was changed, its generation will increase 1.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// ObservedWorkloadReplicas is observed replicas of target referenced workload.
	// This field is designed to deal with scaling event during rollout, if this field changed,
	// it means that the workload is scaling during rollout.
	ObservedWorkloadReplicas int32 `json:"observedWorkloadReplicas,omitempty"`
	// Count of hash collisions for creating canary Deployment. The controller uses this
	// field as a collision avoidance mechanism when it needs to create the name for the
	// newest canary Deployment.
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// ObservedReleasePlanHash is a hash code of observed itself spec.releasePlan.
	ObservedReleasePlanHash string `json:"observedReleasePlanHash,omitempty"`
	// Phase is the release plan phase, which indicates the current state of release
	// plan state machine in BatchRelease controller.
	Phase RolloutPhase `json:"phase,omitempty"`
}

type BatchReleaseCanaryStatus struct {
	// CurrentBatchState indicates the release state of the current batch.
	CurrentBatchState BatchReleaseBatchStateType `json:"batchState,omitempty"`
	// The current batch the rollout is working on/blocked, it starts from 0
	CurrentBatch int32 `json:"currentBatch"`
	// BatchReadyTime is the ready timestamp of the current batch or the last batch.
	// This field is updated once a batch ready, and the batches[x].pausedSeconds
	// relies on this field to calculate the real-time duration.
	BatchReadyTime *metav1.Time `json:"batchReadyTime,omitempty"`
	// UpdatedReplicas is the number of upgraded Pods.
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// UpdatedReadyReplicas is the number upgraded Pods that have a Ready Condition.
	UpdatedReadyReplicas int32 `json:"updatedReadyReplicas,omitempty"`
}

type BatchReleaseBatchStateType string

const (
	// UpgradingBatchState indicates that current batch is at upgrading pod state
	UpgradingBatchState BatchReleaseBatchStateType = "Upgrading"
	// VerifyingBatchState indicates that current batch is at verifying whether it's ready state
	VerifyingBatchState BatchReleaseBatchStateType = "Verifying"
	// ReadyBatchState indicates that current batch is at batch ready state
	ReadyBatchState BatchReleaseBatchStateType = "Ready"
)

const (
	// VerifyingBatchReleaseCondition indicates the controller is verifying whether workload
	// is ready to do rollout.
	VerifyingBatchReleaseCondition RolloutConditionType = "Verifying"
	// PreparingBatchReleaseCondition indicates the controller is preparing something before executing
	// release plan, such as create canary deployment and record stable & canary revisions.
	PreparingBatchReleaseCondition RolloutConditionType = "Preparing"
	// ProgressingBatchReleaseCondition indicates the controller is executing release plan.
	ProgressingBatchReleaseCondition RolloutConditionType = "Progressing"
	// FinalizingBatchReleaseCondition indicates the canary state is completed,
	// and the controller is doing something, such as cleaning up canary deployment.
	FinalizingBatchReleaseCondition RolloutConditionType = "Finalizing"
	// TerminatingBatchReleaseCondition indicates the rollout is terminating when the
	// BatchRelease cr is being deleted or cancelled.
	TerminatingBatchReleaseCondition RolloutConditionType = "Terminating"
	// TerminatedBatchReleaseCondition indicates the BatchRelease cr can be deleted.
	TerminatedBatchReleaseCondition RolloutConditionType = "Terminated"
	// CancelledBatchReleaseCondition indicates the release plan is cancelled during rollout.
	CancelledBatchReleaseCondition RolloutConditionType = "Cancelled"
	// CompletedBatchReleaseCondition indicates the release plan is completed successfully.
	CompletedBatchReleaseCondition RolloutConditionType = "Completed"

	SucceededBatchReleaseConditionReason = "Succeeded"
	FailedBatchReleaseConditionReason    = "Failed"
)
