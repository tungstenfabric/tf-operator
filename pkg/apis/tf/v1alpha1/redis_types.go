package v1alpha1

import (
	"bytes"
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Redis is the Schema for the redis API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// RedisSpec is the Spec for the redis API.
// +k8s:openapi-gen=true
type RedisSpec struct {
	CommonConfiguration  PodConfiguration   `json:"commonConfiguration,omitempty"`
	ServiceConfiguration RedisConfiguration `json:"serviceConfiguration"`
}

// RedisConfiguration is the Spec for the redis API.
// +k8s:openapi-gen=true
type RedisConfiguration struct {
	Containers    []*Container `json:"containers,omitempty"`
	ClusterName   string       `json:"clusterName,omitempty"`
	ListenAddress string       `json:"listenAddress,omitempty"`
	RedisPort     *int         `json:"redisPort,omitempty"`
	Storage       Storage      `json:"storage,omitempty"`
}

// RedisStatus defines the status of the redis object.
// +k8s:openapi-gen=true
type RedisStatus struct {
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	ClusterIP     string            `json:"clusterIP,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}

// RedisList contains a list of Redis.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redis `json:"items"`
}

var redisLog = logf.Log.WithName("controller_redis")

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}

// InstanceConfiguration creates the redis instance configuration.
func (c *Redis) InstanceConfiguration(request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {
	instanceType := "redis"
	instanceConfigMapName := request.Name + "-" + instanceType + "-configmap"
	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: instanceConfigMapName, Namespace: request.Namespace},
		configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	var data = make(map[string]string)

	for _, pod := range podList {
		podIP := pod.Status.PodIP

		var stunnelBuffer bytes.Buffer
		err = configtemplates.StunnelConfig.Execute(&stunnelBuffer, struct {
			RedisListenAddress string
			RedisServerPort    string
		}{
			RedisListenAddress: podIP,
			RedisServerPort:    "6379",
		})
		if err != nil {
			panic(err)
		}
		data["stunnel."+podIP] = stunnelBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap creates a configmap for redis service.
func (c *Redis) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {

	configMap, err := CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"redis",
		c)
	if err != nil {
		return nil, err
	}

	if err = client.Update(context.TODO(), configMap); err != nil {
		return nil, err
	}
	return configMap, nil
}

// CreateSecret creates a secret.
func (c *Redis) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"redis",
		c)
}

// PrepareSTS prepares the intended deployment for the Redis object.
func (c *Redis) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	podMgmtPolicyParallel := true
	return PrepareSTS(sts, commonConfiguration, "redis", request, scheme, c, podMgmtPolicyParallel)
}

// AddVolumesToIntendedSTS adds volumes to the Redis deployment.
func (c *Redis) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Redis) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// CreateSTS creates the STS.
func (c *Redis) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Redis) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, client client.Client) (bool, error) {
	return UpdateServiceSTS(c, instanceType, sts, false, client)
}

// SetInstanceActive sets the Redis instance to active
func (c *Redis) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace}, sts); err != nil {
		return err
	}
	*activeStatus = sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	if err := client.Status().Update(context.TODO(), c); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Redis) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

//PodsCertSubjects gets list of Redis pods certificate subjets which can be passed to the certificate API
func (c *Redis) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, altIPs)
}

// QuerySTS queries the Redis STS
func (c *Redis) QuerySTS(name string, namespace string, reconcileClient client.Client) (*appsv1.StatefulSet, error) {
	return QuerySTS(name, namespace, reconcileClient)
}

// ManageNodeStatus updates nodes in status
func (c *Redis) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) (updated bool, err error) {
	updated = false
	err = nil

	if reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		return
	}

	c.Status.Nodes = podNameIPMap
	if err = client.Status().Update(context.TODO(), c); err != nil {
		return
	}

	updated = true
	return
}

// IsActive returns true if instance is active.
func (c *Redis) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// IsUpgrading returns true if instance is upgrading.
func (c *Redis) IsUpgrading(name string, namespace string, client client.Client) bool {
	instance := &Redis{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return false
	}
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name + "-" + "redis" + "-statefulset", Namespace: namespace}, sts)
	if err != nil {
		return false
	}
	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return true
	}
	return false
}

// ConfigurationParameters sets the default for the configuration parameters.
func (c *Redis) ConfigurationParameters() *RedisConfiguration {
	redisConfiguration := &RedisConfiguration{}
	var redisPort int

	if c.Spec.ServiceConfiguration.Storage.Path == "" {
		redisConfiguration.Storage.Path = "/var/lib/redis"
	} else {
		redisConfiguration.Storage.Path = c.Spec.ServiceConfiguration.Storage.Path
	}
	if c.Spec.ServiceConfiguration.Storage.Size == "" {
		redisConfiguration.Storage.Size = "5Gi"
	} else {
		redisConfiguration.Storage.Size = c.Spec.ServiceConfiguration.Storage.Size
	}
	if c.Spec.ServiceConfiguration.RedisPort != nil {
		redisPort = *c.Spec.ServiceConfiguration.RedisPort
	} else {
		redisPort = RedisPort
	}
	redisConfiguration.RedisPort = &redisPort
	if redisConfiguration.ListenAddress == "" {
		redisConfiguration.ListenAddress = "auto"
	}

	return redisConfiguration
}

// UpdateStatus manages the status of the Redis nodes.
func (c *Redis) UpdateStatus(redisConfig *RedisConfiguration, podNameIPMap map[string]string, sts *appsv1.StatefulSet) bool {
	log := redisLog.WithName("UpdateStatus")
	changed := false

	if !reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		log.Info("Nodes", "new", podNameIPMap, "old", c.Status.Nodes)
		c.Status.Nodes = podNameIPMap
		changed = true
	}

	// TODO: uncleat why sts.Spec.Replicas might be nul:
	// butsomtimes appear error:
	// "Observed a panic: "invalid memory address or nil pointer dereference"
	a := sts != nil && sts.Spec.Replicas != nil && sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	if c.Status.Active == nil {
		log.Info("Active", "new", a, "old", c.Status.Active)
		c.Status.Active = new(bool)
		*c.Status.Active = a
		changed = true
	}
	if *c.Status.Active != a {
		log.Info("Active", "new", a, "old", *c.Status.Active)
		*c.Status.Active = a
		changed = true
	}

	return changed || (c.Status.ConfigChanged != nil && *c.Status.ConfigChanged)
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Redis) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
