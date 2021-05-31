package v1alpha1

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/randomstring"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Rabbitmq is the Schema for the rabbitmqs API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Rabbitmq struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqSpec   `json:"spec,omitempty"`
	Status RabbitmqStatus `json:"status,omitempty"`
}

// RabbitmqSpec is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type RabbitmqSpec struct {
	CommonConfiguration  PodConfiguration      `json:"commonConfiguration,omitempty"`
	ServiceConfiguration RabbitmqConfiguration `json:"serviceConfiguration"`
}

// RabbitmqConfiguration is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type RabbitmqConfiguration struct {
	Containers   []*Container `json:"containers,omitempty"`
	Port         *int         `json:"port,omitempty"`
	ErlEpmdPort  *int         `json:"erlEpmdPort,omitempty"`
	ErlangCookie string       `json:"erlangCookie,omitempty"`
	Vhost        string       `json:"vhost,omitempty"`
	User         string       `json:"user,omitempty"`
	Password     string       `json:"password,omitempty"`
	Secret       string       `json:"secret,omitempty"`
	// +kubebuilder:validation:Enum=exactly;all;nodes
	MirroredQueueMode        *string                 `json:"mirroredQueueMode,omitempty"`
	ClusterPartitionHandling *string                 `json:"clusterPartitionHandling,omitempty"`
	TCPListenOptions         *TCPListenOptionsConfig `json:"tcpListenOptions,omitempty"`
}

// RabbitmqStatus +k8s:openapi-gen=true
type RabbitmqStatus struct {
	CommonStatus `json:",inline"`
	Secret       string `json:"secret,omitempty"`
}

// TCPListenOptionsConfig is configuration for RabbitMQ TCP listen
// +k8s:openapi-gen=true
type TCPListenOptionsConfig struct {
	Backlog       *int  `json:"backlog,omitempty"`
	Nodelay       *bool `json:"nodelay,omitempty"`
	LingerOn      *bool `json:"lingerOn,omitempty"`
	LingerTimeout *int  `json:"lingerTimeout,omitempty"`
	ExitOnClose   *bool `json:"exitOnClose,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqList contains a list of Rabbitmq.
type RabbitmqList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Rabbitmq `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Rabbitmq{}, &RabbitmqList{})
}

// InstanceConfiguration prepare rabbit configs
func (c *Rabbitmq) InstanceConfiguration(request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {

	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })

	c.ConfigurationParameters()

	var data = make(map[string]string)
	for _, pod := range podList {
		var rabbitmqPodConfig bytes.Buffer
		err := configtemplates.RabbitmqPodConfig.Execute(&rabbitmqPodConfig, struct {
			RabbitmqPort             int
			SignerCAFilepath         string
			ClusterPartitionHandling string
			PodIP                    string
			PodsList                 []corev1.Pod
			TCPListenOptions         *TCPListenOptionsConfig
			LogLevel                 string
		}{
			RabbitmqPort:             *c.Spec.ServiceConfiguration.Port,
			SignerCAFilepath:         certificates.SignerCAFilepath,
			ClusterPartitionHandling: *c.Spec.ServiceConfiguration.ClusterPartitionHandling,
			PodIP:                    pod.Status.PodIP,
			PodsList:                 podList,
			LogLevel:                 c.Spec.CommonConfiguration.LogLevel,
			TCPListenOptions:         c.Spec.ServiceConfiguration.TCPListenOptions,
		})
		if err != nil {
			panic(err)
		}
		data["rabbitmq.conf."+pod.Status.PodIP] = rabbitmqPodConfig.String()
		rabbitmqEnvConfigString := fmt.Sprintf("HOME=/var/lib/rabbitmq\n")
		rabbitmqEnvConfigString = rabbitmqEnvConfigString + fmt.Sprintf("NODENAME=rabbit@%s\n", pod.Status.PodIP)

		data["rabbitmq-env.conf."+pod.Status.PodIP] = rabbitmqEnvConfigString
	}

	var rabbitmqNodes string
	for _, pod := range podList {
		myidString := pod.Name[len(pod.Name)-1:]
		data[myidString] = pod.Status.PodIP
		rabbitmqNodes = rabbitmqNodes + fmt.Sprintf("%s\n", pod.Status.PodIP)
	}

	data["rabbitmq.nodes"] = rabbitmqNodes
	data["plugins.conf"] = "[rabbitmq_management,rabbitmq_management_agent,rabbitmq_peer_discovery_k8s]."

	// common env vars
	rabbitmqCommonEnvString := fmt.Sprintf("export RABBITMQ_ERLANG_COOKIE=%s\n", c.Spec.ServiceConfiguration.ErlangCookie)
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_CONFIG_FILE=/etc/rabbitmq/rabbitmq.conf\n")
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_CONF_ENV_FILE=/etc/rabbitmq/rabbitmq-env.conf\n")
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_ENABLED_PLUGINS_FILE=/etc/rabbitmq/plugins.conf\n")
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_USE_LONGNAME=true\n")
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_PID_FILE=/var/run/rabbitmq.pid\n")

	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export ERL_EPMD_PORT=%d\n", *c.Spec.ServiceConfiguration.ErlEpmdPort)
	distPort := *c.Spec.ServiceConfiguration.Port + 20000
	rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_DIST_PORT=%d\n", distPort)
	// TODO: for now tls is not enabled for dist & management ports
	// rabbitmqCommonEnvString = rabbitmqCommonEnvString + fmt.Sprintf("export RABBITMQ_CTL_ERL_ARGS=\"-proto_dist inet_tls\"\n")

	data["rabbitmq-common.env"] = rabbitmqCommonEnvString

	var secretName string
	secret := &corev1.Secret{}
	if c.Spec.ServiceConfiguration.Secret != "" {
		secretName = c.Spec.ServiceConfiguration.Secret
	} else {
		secretName = request.Name + "-secret"
	}
	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: request.Namespace}, secret)
	if err != nil {
		return err
	}

	saltedP := secret.Data["salted_password"]

	var rabbitmqDefinitionBuffer bytes.Buffer
	err = configtemplates.RabbitmqDefinition.Execute(&rabbitmqDefinitionBuffer, struct {
		RabbitmqUser      string
		RabbitmqPassword  string
		RabbitmqVhost     string
		MirroredQueueMode string
		RabbitmqPort      int
	}{
		RabbitmqUser:      string(secret.Data["user"]),
		RabbitmqPassword:  base64.StdEncoding.EncodeToString(saltedP),
		RabbitmqVhost:     string(secret.Data["vhost"]),
		MirroredQueueMode: *c.Spec.ServiceConfiguration.MirroredQueueMode,
		RabbitmqPort:      *c.Spec.ServiceConfiguration.Port,
	})
	if err != nil {
		panic(err)
	}
	data["definitions.json"] = rabbitmqDefinitionBuffer.String()

	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err = client.Get(context.TODO(),
		types.NamespacedName{Name: request.Name + "-" + "rabbitmq" + "-configmap", Namespace: request.Namespace},
		configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}
	configMapInstanceDynamicConfig.Data = data
	err = client.Update(context.TODO(), configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	configMapInstancConfig := &corev1.ConfigMap{}
	err = client.Get(context.TODO(),
		types.NamespacedName{Name: request.Name + "-" + "rabbitmq" + "-configmap-runner", Namespace: request.Namespace},
		configMapInstancConfig)
	if err != nil {
		return err
	}
	var rabbitmqConfigBuffer bytes.Buffer
	err = configtemplates.RabbitmqConfig.Execute(&rabbitmqConfigBuffer, struct{}{})
	if err != nil {
		panic(err)
	}
	configMapInstancConfig.Data = map[string]string{"run.sh": rabbitmqConfigBuffer.String()}
	err = client.Update(context.TODO(), configMapInstancConfig)
	if err != nil {
		return err
	}

	return nil
}

func setRandomField(data map[string][]byte, field string, value string, size int) map[string][]byte {
	if value == "" {
		if _, ok := data[field]; !ok {
			data[field] = []byte(randomstring.RandString{Size: size}.Generate())
		}
	} else {
		data[field] = []byte(value)
	}
	return data
}

// UpdateSecret
func (c *Rabbitmq) UpdateSecret(secret *corev1.Secret, client client.Client) (updated bool, err error) {
	updated, err = false, nil

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	oldData := make(map[string][]byte)
	for k, v := range secret.Data {
		oldData[k] = v
	}

	setRandomField(secret.Data, "user", c.Spec.ServiceConfiguration.User, 8)
	setRandomField(secret.Data, "password", c.Spec.ServiceConfiguration.Password, 32)
	setRandomField(secret.Data, "vhost", c.Spec.ServiceConfiguration.Vhost, 6)

	if old, ok := oldData["password"]; !ok || string(old) != string(secret.Data["password"]) {
		salt := [4]byte{}
		if _, err = rand.Read(salt[:]); err != nil {
			return
		}
		saltedP := append(salt[:], secret.Data["password"]...)
		hash := sha256.New()
		if _, err = hash.Write(saltedP); err != nil {
			return
		}
		hashPass := hash.Sum(nil)
		saltedP = append(salt[:], hashPass...)
		secret.Data["salted_password"] = saltedP
	}

	if reflect.DeepEqual(oldData, secret.Data) {
		return
	}

	if err = client.Update(context.TODO(), secret); err != nil {
		return
	}
	updated = true
	return
}

func (c *Rabbitmq) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"rabbitmq",
		c)
}

func (c *Rabbitmq) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"rabbitmq",
		c)
}

// IsActive returns true if instance is active.
func (c *Rabbitmq) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// IsUpgrading returns true if instance is upgrading.
func (c *Rabbitmq) IsUpgrading(name string, namespace string, client client.Client) bool {
	instance := &Rabbitmq{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return false
	}
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name + "-" + "rabbitmq" + "-statefulset", Namespace: namespace}, sts)
	if err != nil {
		return false
	}
	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return true
	}
	return false
}

// PrepareSTS prepares the intended deployment for the Rabbitmq object.
func (c *Rabbitmq) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "rabbitmq", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Rabbitmq) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Rabbitmq) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// CreateSTS creates the STS.
func (c *Rabbitmq) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Rabbitmq) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, client client.Client) (bool, error) {
	return UpdateServiceSTS(c, instanceType, sts, false, client)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Rabbitmq) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

//PodsCertSubjects gets list of Rabbitmq pods certificate subjets which can be passed to the certificate API
func (c *Rabbitmq) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetInstanceActive sets the Cassandra instance to active.
func (c *Rabbitmq) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, degradedStatus, sts, request, c)
}

func (c *Rabbitmq) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) (updated bool, err error) {
	updated = false
	err = nil

	c.ConfigurationParameters()
	secret := c.Spec.ServiceConfiguration.Secret
	if secret == c.Status.Secret && reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		return
	}

	c.Status.Secret = secret
	c.Status.Nodes = podNameIPMap
	if err = client.Status().Update(context.TODO(), c); err != nil {
		return
	}

	updated = true
	return
}

func (c *Rabbitmq) ConfigurationParameters() {
	var port = RabbitmqNodePort
	var erlEpmdPort = RabbitmqErlEpmdPort
	var erlangCookie = RabbitmqErlangCookie
	var vhost = RabbitmqVhost
	var user = RabbitmqUser
	var password = RabbitmqPassword
	var secret = c.GetName() + "-secret"
	var partHandling = RabbitmqClusterPartitionHandling
	var mirredQueueMode = RabbitmqMirroredQueueMode

	if c.Spec.ServiceConfiguration.Port == nil {
		c.Spec.ServiceConfiguration.Port = &port
	}
	if c.Spec.ServiceConfiguration.ErlEpmdPort == nil {
		c.Spec.ServiceConfiguration.ErlEpmdPort = &erlEpmdPort
	}
	if c.Spec.ServiceConfiguration.ErlangCookie == "" {
		c.Spec.ServiceConfiguration.ErlangCookie = erlangCookie
	}
	if c.Spec.ServiceConfiguration.Vhost == "" {
		c.Spec.ServiceConfiguration.Vhost = vhost
	}
	if c.Spec.ServiceConfiguration.User == "" {
		c.Spec.ServiceConfiguration.User = user
	}
	if c.Spec.ServiceConfiguration.Password == "" {
		c.Spec.ServiceConfiguration.Password = password
	}
	if c.Spec.ServiceConfiguration.Secret == "" {
		c.Spec.ServiceConfiguration.Secret = secret
	}
	if c.Spec.ServiceConfiguration.ClusterPartitionHandling == nil {
		c.Spec.ServiceConfiguration.ClusterPartitionHandling = &partHandling
	}
	if c.Spec.ServiceConfiguration.MirroredQueueMode == nil {
		c.Spec.ServiceConfiguration.MirroredQueueMode = &mirredQueueMode
	}
}
