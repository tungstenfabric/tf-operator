package k8s

import (
	"context"
	"strings"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Kubernetes is used to create and update meaningful objects
type Kubernetes struct {
	client client.Client
	scheme *runtime.Scheme
}

type object interface {
	GetName() string
	GetUID() types.UID
	GetOwnerReferences() []meta.OwnerReference
	SetOwnerReferences(references []meta.OwnerReference)
	runtime.Object
}

// New is used to create a new Kubernetes
func New(client client.Client, scheme *runtime.Scheme) *Kubernetes {
	return &Kubernetes{
		client: client,
		scheme: scheme,
	}
}

// Owner is used to create Owner object
func (k *Kubernetes) Owner(owner object) *Owner {
	return &Owner{owner: owner, client: k.client, scheme: k.scheme}
}

// ConfigMap is used to create ConfigMap object
func (k *Kubernetes) ConfigMap(name, ownerType string, owner v1.Object) *ConfigMap {
	return &ConfigMap{name: name, ownerType: ownerType, owner: owner, client: k.client, scheme: k.scheme}
}

// Secret is used to create Secret object
func (k *Kubernetes) Secret(name, ownerType string, owner v1.Object) *Secret {
	return &Secret{name: name, ownerType: ownerType, owner: owner, client: k.client, scheme: k.scheme}
}

// Service is used to create Service object
func (k *Kubernetes) Service(name string, servType core.ServiceType, ports map[int32]string, ownerType string, owner v1.Object) *Service {
	return &Service{name: name, servType: servType, ports: ports, ownerType: ownerType, owner: owner, client: k.client, scheme: k.scheme}
}

var isOpenshift bool = false

func IsOpenshift() bool {
	return isOpenshift
}

func SetDeployerTypeE(v bool) {
	isOpenshift = v
}

func SetDeployerType(client client.Client) error {
	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "machineconfiguration.openshift.io",
		Kind:    "MachineConfigList",
		Version: "v1",
	})
	if err := client.List(context.Background(), u); err != nil {
		if strings.Contains(err.Error(), "no matches for kind \"MachineConfig\"") {
			SetDeployerTypeE(false)
			return nil
		}
		return err
	}
	SetDeployerTypeE(true)
	return nil
}
