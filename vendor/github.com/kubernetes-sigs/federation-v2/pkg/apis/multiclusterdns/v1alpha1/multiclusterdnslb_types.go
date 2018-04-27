/*
Copyright 2018 The Federation v2 Authors.

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
	"log"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MultiClusterDNSLb
// +k8s:openapi-gen=true
// +resource:path=multiclusterdnslbs,strategy=MultiClusterDNSLbStrategy
type MultiClusterDNSLb struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MultiClusterDNSLbSpec   `json:"spec,omitempty"`
	Status MultiClusterDNSLbStatus `json:"status,omitempty"`
}

// MultiClusterDNSLbSpec defines the desired state of MultiClusterDNSLb
type MultiClusterDNSLbSpec struct {
	// FederationName is the name of the federation to which the corresponding federated service belongs
	FederationName string `json:"federationName,omitempty"`
	// DNSSuffix is the suffix (domain) to append to DNS names
	DNSSuffix string `json:"dnsSuffix,omitempty"`
}

// MultiClusterDNSLbStatus defines the observed state of MultiClusterDNSLb
type MultiClusterDNSLbStatus struct {
	DNS []ClusterDNS `json:"dns,omitempty"`
}

type ClusterDNS struct {
	// Cluster name
	Cluster string
	// LoadBalancer for the corresponding service
	LoadBalancer corev1.LoadBalancerStatus `json:"loadBalancer,omitempty"`
	// Zone to which the cluster belongs
	Zone string `json:"zone,omitempty"`
	// Region to which the cluster belongs
	Region string `json:"region,omitempty"`
}

// Validate checks that an instance of MultiClusterDNSLb is well formed
func (MultiClusterDNSLbStrategy) Validate(ctx request.Context, obj runtime.Object) field.ErrorList {
	o := obj.(*multiclusterdns.MultiClusterDNSLb)
	log.Printf("Validating fields for MultiClusterDNSLb %s\n", o.Name)
	errors := field.ErrorList{}
	// perform validation here and add to errors using field.Invalid
	return errors
}

// DefaultingFunction sets default MultiClusterDNSLb field values
func (MultiClusterDNSLbSchemeFns) DefaultingFunction(o interface{}) {
	obj := o.(*MultiClusterDNSLb)
	// set default field values here
	log.Printf("Defaulting fields for MultiClusterDNSLb %s\n", obj.Name)
}
