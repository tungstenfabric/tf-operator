package v1alpha1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagerSpec defines the desired state of Manager.
// +k8s:openapi-gen=true
type ManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	CommonConfiguration ManagerConfiguration `json:"commonConfiguration,omitempty"`
	Services            Services             `json:"services,omitempty"`
}

// Services defines the desired state of Services.
// +k8s:openapi-gen=true
type Services struct {
	AnalyticsSnmp  *AnalyticsSnmp  `json:"analyticsSnmp,omitempty"`
	AnalyticsAlarm *AnalyticsAlarm `json:"analyticsAlarm,omitempty"`
	Analytics      *Analytics      `json:"analytics,omitempty"`
	Config         *Config         `json:"config,omitempty"`
	Controls       []*Control      `json:"controls,omitempty"`
	Kubemanager    *Kubemanager    `json:"kubemanager,omitempty"`
	QueryEngine    *QueryEngine    `json:"queryengine,omitempty"`
	Webui          *Webui          `json:"webui,omitempty"`
	Vrouters       []*Vrouter      `json:"vrouters,omitempty"`
	Cassandras     []*Cassandra    `json:"cassandras,omitempty"`
	Zookeeper      *Zookeeper      `json:"zookeeper,omitempty"`
	Rabbitmq       *Rabbitmq       `json:"rabbitmq,omitempty"`
	Redis          []*Redis        `json:"redis,omitempty"`
}

// ManagerConfiguration is the common services struct.
// +k8s:openapi-gen=true
type ManagerConfiguration struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
	// AuthParameters auth parameters
	// +optional
	AuthParameters AuthParameters `json:"authParameters,omitempty"`
	// Allow node-init container to tune sysctl options
	// (for all deployers except opneshift it is done by node-init, in openshift - machineconfig)
	// +optional
	TuneSysctl *bool `json:"tuneSysctl,omitempty"`
	// Kubernetes Cluster Configuration
	// +optional
	ClusterConfig *KubernetesClusterConfig `json:"clusterConfig,omitempty"`
	// Kubernetes Cluster Configuration
	// +kubebuilder:validation:Enum=info;debug;warning;error;critical;none
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
	// OS family
	// +optional
	Distribution *string `json:"distribution,omitempty"`
}

// ZIU status for orchestrating cluster ZIU process
// -1 not needed
// 0 not detected
// 1..x ziu stages
type ZIUStatus int32

// ManagerStatus defines the observed state of Manager.
// +k8s:openapi-gen=true
type ManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	AnalyticsSnmp  *ServiceStatus   `json:"analyticsSnmp,omitempty"`
	AnalyticsAlarm *ServiceStatus   `json:"analyticsAlarm,omitempty"`
	Analytics      *ServiceStatus   `json:"analytics,omitempty"`
	Config         *ServiceStatus   `json:"config,omitempty"`
	Controls       []*ServiceStatus `json:"controls,omitempty"`
	Kubemanager    *ServiceStatus   `json:"kubemanager,omitempty"`
	QueryEngine    *ServiceStatus   `json:"queryengine,omitempty"`
	Webui          *ServiceStatus   `json:"webui,omitempty"`
	Vrouters       []*ServiceStatus `json:"vrouters,omitempty"`
	Cassandras     []*ServiceStatus `json:"cassandras,omitempty"`
	Zookeeper      *ServiceStatus   `json:"zookeeper,omitempty"`
	Rabbitmq       *ServiceStatus   `json:"rabbitmq,omitempty"`
	Redis          []*ServiceStatus `json:"redis,omitempty"`
	CrdStatus      []CrdStatus      `json:"crdStatus,omitempty"`
	ZiuState       ZIUStatus        `json:"ziuState,omitempty"`
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ManagerCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// ManagerConditionType is used to represent condition of manager.
type ManagerConditionType string

// These are valid conditions of manager.
const (
	ManagerReady ManagerConditionType = "Ready"
)

// ConditionStatus is used to indicate state of condition.
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition.
// "ConditionFalse" means a resource is not in the condition.
const (
	ConditionTrue  ConditionStatus = "True"
	ConditionFalse ConditionStatus = "False"
)

// ManagerCondition is used to represent cluster condition
type ManagerCondition struct {
	// Type of manager condition.
	Type ManagerConditionType `json:"type"`
	// Status of the condition, one of True or False.
	Status ConditionStatus `json:"status"`
}

// CrdStatus tracks status of CRD.
// +k8s:openapi-gen=true
type CrdStatus struct {
	Name   string `json:"name,omitempty"`
	Active *bool  `json:"active,omitempty"`
}

func (m *Manager) Cassandra() *Cassandra {
	return &Cassandra{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// Manager is the Schema for the managers API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Manager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagerSpec   `json:"spec,omitempty"`
	Status ManagerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagerList contains a list of Manager.
type ManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manager `json:"items"`
}

func (m *Manager) Get(client client.Client, request reconcile.Request) error {
	err := client.Get(context.TODO(), request.NamespacedName, m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) Create(client client.Client) error {
	err := client.Create(context.TODO(), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) Update(client client.Client) error {
	err := client.Update(context.TODO(), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) Delete(client client.Client) error {
	err := client.Delete(context.TODO(), m)
	if err != nil {
		return err
	}
	return nil
}

func (m *Manager) GetObjectFromObjectList(objectList *[]*interface{}, request reconcile.Request) interface{} {
	return nil
}

func (m Manager) IsClusterReady() bool {
	for _, cassandraService := range m.Spec.Services.Cassandras {
		for _, cassandraStatus := range m.Status.Cassandras {
			if cassandraService.Name == *cassandraStatus.Name && !cassandraStatus.ready() {
				return false
			}
		}
	}
	for _, controlService := range m.Spec.Services.Controls {
		for _, controlStatus := range m.Status.Controls {
			if controlService.Name == *controlStatus.Name && !controlStatus.ready() {
				return false
			}
		}
	}

	for _, vrouterService := range m.Spec.Services.Vrouters {
		for _, vrouterStatus := range m.Status.Vrouters {
			if vrouterService.Name == *vrouterStatus.Name && !vrouterStatus.ready() {
				return false
			}
		}
	}

	for _, redisService := range m.Spec.Services.Redis {
		for _, redisStatus := range m.Status.Redis {
			if redisService.Name == *redisStatus.Name && !redisStatus.ready() {
				return false
			}
		}
	}

	if m.Spec.Services.Zookeeper != nil && !m.Status.Zookeeper.ready() {
		return false
	}

	if m.Spec.Services.Kubemanager != nil && !m.Status.Kubemanager.ready() {
		return false
	}

	if m.Spec.Services.Webui != nil && !m.Status.Webui.ready() {
		return false
	}
	if m.Spec.Services.Config != nil && !m.Status.Config.ready() {
		return false
	}
	if m.Spec.Services.Rabbitmq != nil && !m.Status.Rabbitmq.ready() {
		return false
	}
	return true
}

// IsVrouterActiveOnControllers checks if vrouters are active on master nodes
func (m *Manager) IsVrouterActiveOnControllers(clnt client.Client) bool {
	if len(m.Spec.Services.Vrouters) == 0 {
		return true
	}
	spec := m.Spec.Services.Vrouters[0]
	vrouter := &Vrouter{}
	if err := clnt.Get(context.TODO(), types.NamespacedName{Name: spec.Name, Namespace: m.Namespace}, vrouter); err != nil {
		return false
	}
	if f, err := vrouter.IsActiveOnControllers(clnt); err == nil {
		return f
	}
	return false
}

func init() {
	SchemeBuilder.Register(&Manager{}, &ManagerList{})
}
