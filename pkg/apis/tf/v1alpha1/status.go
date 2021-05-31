package v1alpha1

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommonStatus is the common part of service status.
// +k8s:openapi-gen=true
type CommonStatus struct {
	Active        *bool             `json:"active,omitempty"`
	Degraded      *bool             `json:"degraded,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}
