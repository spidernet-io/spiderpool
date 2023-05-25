// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type SchedulePlan struct {
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	StartAfterMinute int64 `json:"startAfterMinute"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	RoundNumber int64 `json:"roundNumber"`

	// +kubebuilder:default=360
	// +kubebuilder:validation:Minimum=1
	IntervalMinute int64 `json:"intervalMinute"`

	// +kubebuilder:default=60
	// +kubebuilder:validation:Minimum=1
	TimeoutMinute int64 `json:"timeoutMinute"`

	// +kubebuilder:validation:Optional
	SourceAgentNodeSelector *metav1.LabelSelector `json:"sourceAgentNodeSelector,omitempty"`
}

type TaskStatus struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	ExpectedRound *int64 `json:"expectedRound,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	DoneRound *int64 `json:"doneRound,omitempty"`

	Finish bool `json:"finish"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=succeed;fail;unknown
	LastRoundStatus *string `json:"lastRoundStatus,omitempty"`

	History []StatusHistoryRecord `json:"history"`
}

const (
	StatusHistoryRecordStatusSucceed    = "succeed"
	StatusHistoryRecordStatusFail       = "fail"
	StatusHistoryRecordStatusOngoing    = "ongoing"
	StatusHistoryRecordStatusNotstarted = "notstarted"
)

type StatusHistoryRecord struct {

	// +kubebuilder:validation:Enum=succeed;fail;ongoing;notstarted
	Status string `json:"status"`

	// +kubebuilder:validation:Optional
	FailureReason string `json:"failureReason,omitempty"`

	RoundNumber int `json:"roundNumber"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Format:=date-time
	StartTimeStamp metav1.Time `json:"startTimeStamp"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Format:=date-time
	EndTimeStamp *metav1.Time `json:"endTimeStamp,omitempty"`

	// +kubebuilder:validation:Optional
	Duration *string `json:"duration,omitempty"`

	// +kubebuilder:validation:Type:=string
	// +kubebuilder:validation:Format:=date-time
	DeadLineTimeStamp metav1.Time `json:"deadLineTimeStamp"`

	// +kubebuilder:validation:Optional
	// expected how many agents should involve
	ExpectedActorNumber *int `json:"expectedActorNumber,omitempty"`

	FailedAgentNodeList []string `json:"failedAgentNodeList"`

	SucceedAgentNodeList []string `json:"succeedAgentNodeList"`

	NotReportAgentNodeList []string `json:"notReportAgentNodeList"`
}

type NetSuccessCondition struct {

	// +kubebuilder:default=1
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	SuccessRate *float64 `json:"successRate,omitempty"`

	// +kubebuilder:default=5000
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Optional
	MeanAccessDelayInMs *int64 `json:"meanAccessDelayInMs,omitempty"`
}
