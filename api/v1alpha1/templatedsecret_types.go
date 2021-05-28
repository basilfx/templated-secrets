package v1alpha1

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create.
// Necessary until https://github.com/kubernetes-sigs/controller-tools/pull/539 is merged.
type PartialObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,4,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,5,rep,name=annotations"`
}

// SecretTemplateSpec defines the structure a Secret should have
// when created from a template
type SecretTemplateSpec struct {
	// Standard object's metadata.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +kubebuilder:validation:Optional
	ObjectMeta PartialObjectMeta `json:"metadata,omitempty"`

	// Used to facilitate programmatic handling of secret data.
	// +kubebuilder:validation:Optional
	Type apiv1.SecretType `json:"type,omitempty"`
}

// TemplatedSecretSpec defines the desired state of TemplatedSecret
type TemplatedSecretSpec struct {
	// +kubebuilder:validation:Optional
	Template SecretTemplateSpec `json:"template,omitempty"`
	Data     map[string]string  `json:"data"`
}

// TemplatedSecretStatus defines the observed state of TemplatedSecret
type TemplatedSecretStatus struct {
	Message string `json:"message"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TemplatedSecret is the Schema for the templatedsecrets API
type TemplatedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplatedSecretSpec   `json:"spec,omitempty"`
	Status TemplatedSecretStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TemplatedSecretList contains a list of TemplatedSecret
type TemplatedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplatedSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplatedSecret{}, &TemplatedSecretList{})
}
