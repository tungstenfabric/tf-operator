package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type Storage struct {
	// +kubebuilder:validation:Pattern=^([0-9]+)([KMGTPE]i)?$
	Size string `json:"size,omitempty"` // The only reason we don't use resource.Quantity directly is we can't have regexp for different type than string
	Path string `json:"path,omitempty"`
}

func (s Storage) SizeAsQuantity() (resource.Quantity, error) {
	return resource.ParseQuantity(s.Size)
}
