package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthenticationMode auth mode
// +k8s:openapi-gen=true
// +kubebuilder:validation:Enum=noauth;keystone
type AuthenticationMode string

const (
	// AuthenticationModeNoAuth No auth mode
	AuthenticationModeNoAuth AuthenticationMode = "noauth"
	// AuthenticationModeKeystone Keytsone auth mode
	AuthenticationModeKeystone AuthenticationMode = "keystone"
)

// AuthParameters is Keystone auth options
// +k8s:openapi-gen=true
type AuthParameters struct {
	AuthMode               AuthenticationMode      `json:"authMode,omitempty"`
	KeystoneAuthParameters *KeystoneAuthParameters `json:"keystoneAuthParameters,omitempty"`
	KeystoneSecretName     *string                 `json:"keystoneSecretName,omitempty"`
}

// KeystoneAuthParameters keystone parameters
// +k8s:openapi-gen=true
type KeystoneAuthParameters struct {
	AuthProtocol      string  `json:"authProtocol,omitempty"`
	Address           string  `json:"address,omitempty"`
	Port              *int    `json:"port,omitempty"`
	AdminTenant       string  `json:"adminTenant,omitempty"`
	AdminUsername     string  `json:"adminUsername,omitempty"`
	AdminPassword     *string `json:"adminPassword,omitempty"`
	Region            string  `json:"region,omitempty"`
	UserDomainName    string  `json:"userDomainName,omitempty"`
	ProjectDomainName string  `json:"projectDomainName,omitempty"`
}

// Prepare makes default empty AuthParameters
func (ap *AuthParameters) Prepare(namespace string, client client.Client) error {
	if ap == nil {
		panic(fmt.Errorf("AuthParameters is nil"))
	}
	if ap.KeystoneAuthParameters == nil {
		ap.KeystoneAuthParameters = &KeystoneAuthParameters{}
	}
	c := ap.KeystoneAuthParameters
	if c.AdminUsername == "" {
		c.AdminUsername = KeystoneAuthAdminUser
	}
	if c.AdminTenant == "" {
		c.AdminTenant = KeystoneAuthHost
	}
	if c.AdminPassword == nil {
		if ap.KeystoneSecretName != nil && *ap.KeystoneSecretName != "" {
			adminPasswordSecret := &corev1.Secret{}
			nsName := types.NamespacedName{Name: *ap.KeystoneSecretName, Namespace: namespace}
			if err := client.Get(context.TODO(), nsName, adminPasswordSecret); err != nil {
				return err
			}
			pwdStr := string(adminPasswordSecret.Data["password"])
			c.AdminPassword = &pwdStr
		} else {
			pwdStr := KeystoneAuthAdminPassword
			c.AdminPassword = &pwdStr
		}
	}
	if c.Address == "" {
		c.Address = KeystoneAuthHost
	}
	if c.Port == nil {
		var p int = KeystoneAuthPort
		c.Port = &p
	}
	if c.AuthProtocol == "" {
		c.AuthProtocol = KeystoneAuthProto
	}
	if c.UserDomainName == "" {
		c.UserDomainName = KeystoneAuthUserDomainName
	}
	if c.ProjectDomainName == "" {
		c.ProjectDomainName = KeystoneAuthProjectDomainName
	}
	return nil
}
