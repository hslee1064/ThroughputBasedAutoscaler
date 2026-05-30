/*
Copyright 2023.

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

// AutoscalerSpec defines the desired state of Autoscaler
type AutoscalerSpec struct {
	QueueLength int `json:"queue_length,omitempty"`
}

// AutoscalerStatus defines the observed state of Autoscaler
type AutoscalerStatus struct {
	LastDeploymentsStatus map[string]int `json:"last_deployments_status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Autoscaler is the Schema for the autoscalers API
type Autoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalerSpec   `json:"spec,omitempty"`
	Status AutoscalerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AutoscalerList contains a list of Autoscaler
type AutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Autoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Autoscaler{}, &AutoscalerList{})
}
