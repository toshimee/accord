/*
Copyright 2026.

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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MirrorUpgradeRequestSpec defines the desired state of MirrorUpgradeRequest.
type MirrorUpgradeRequestSpec struct {
	// TargetCluster is the name of the target cluster.
	// +kubebuilder:validation:Required
	TargetCluster string `json:"targetCluster"`

	// TargetNamespace is the namespace where Argo CD is installed.
	// +kubebuilder:validation:Required
	TargetNamespace string `json:"targetNamespace"`

	// CurrentVersion is the current version of Argo CD.
	// +kubebuilder:validation:Required
	CurrentVersion string `json:"currentVersion"`

	// DesiredVersion is the version to upgrade to.
	// +kubebuilder:validation:Required
	DesiredVersion string `json:"desiredVersion"`

	// ApprovalMode controls whether approval is manual or automatic.
	// +kubebuilder:validation:Enum=Manual;Auto
	// +kubebuilder:validation:Required
	ApprovalMode string `json:"approvalMode"`

	// AutoCollectLogs indicates whether to trigger log collection after the job.
	// +optional
	AutoCollectLogs bool `json:"autoCollectLogs,omitempty"`

	// LogArchivePath is the storage path for archived logs.
	// +optional
	LogArchivePath string `json:"logArchivePath,omitempty"`
}

// MirrorUpgradeRequestStatus defines the observed state of MirrorUpgradeRequest.
type MirrorUpgradeRequestStatus struct {
	// Phase is the current state of the request.
	// +kubebuilder:validation:Enum=Pending;Approved;Running;Succeeded;Failed;RolledBack
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message is a human-readable message about the current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt is when the upgrade job actually started.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is when the upgrade job finished.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// JobRef is the name of the Kubernetes Job that was dynamically spawned.
	// +optional
	JobRef string `json:"jobRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MirrorUpgradeRequest is the Schema for the mirrorupgraderequests API
type MirrorUpgradeRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MirrorUpgradeRequest
	// +required
	Spec MirrorUpgradeRequestSpec `json:"spec"`

	// status defines the observed state of MirrorUpgradeRequest
	// +optional
	Status MirrorUpgradeRequestStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MirrorUpgradeRequestList contains a list of MirrorUpgradeRequest
type MirrorUpgradeRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MirrorUpgradeRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MirrorUpgradeRequest{}, &MirrorUpgradeRequestList{})
}
