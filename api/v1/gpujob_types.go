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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GPUJobSpec defines the desired state of GPUJob
type GPUJobSpec struct {
	Replicas int32 `json:"replicas"`

	GPUsPerPod int32 `json:"gpusPerPod"`

	Image string `json:"image"`

	Command []string `json:"command,omitempty"`
}

// GPUJobStatus defines the observed state of GPUJob.
type GPUJobStatus struct {
	Scheduled int32 `json:"scheduled,omitempty"`

	Running int32 `json:"running,omitempty"`

	Succeeded int32 `json:"succeeded,omitempty"`

	Failed int32 `json:"failed,omitempty"`

	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="GPUs/Pod",type=integer,JSONPath=`.spec.gpusPerPod`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GPUJob is the Schema for the gpujobs API
type GPUJob struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of GPUJob
	// +required
	Spec GPUJobSpec `json:"spec"`

	// status defines the observed state of GPUJob
	// +optional
	Status GPUJobStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// GPUJobList contains a list of GPUJob
type GPUJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []GPUJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUJob{}, &GPUJobList{})
}
