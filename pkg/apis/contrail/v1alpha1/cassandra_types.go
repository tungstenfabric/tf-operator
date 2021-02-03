package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/Juniper/contrail-operator/pkg/certificates"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cassandra is the Schema for the cassandras API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Cassandra struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CassandraSpec   `json:"spec,omitempty"`
	Status CassandraStatus `json:"status,omitempty"`
}

// CassandraSpec is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type CassandraSpec struct {
	CommonConfiguration  PodConfiguration       `json:"commonConfiguration,omitempty"`
	ServiceConfiguration CassandraConfiguration `json:"serviceConfiguration"`
}

// CassandraConfiguration is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type CassandraConfiguration struct {
	Containers     []*Container `json:"containers,omitempty"`
	ClusterName    string       `json:"clusterName,omitempty"`
	ListenAddress  string       `json:"listenAddress,omitempty"`
	Port           *int         `json:"port,omitempty"`
	CqlPort        *int         `json:"cqlPort,omitempty"`
	SslStoragePort *int         `json:"sslStoragePort,omitempty"`
	StoragePort    *int         `json:"storagePort,omitempty"`
	JmxLocalPort   *int         `json:"jmxLocalPort,omitempty"`
	MaxHeapSize    string       `json:"maxHeapSize,omitempty"`
	MinHeapSize    string       `json:"minHeapSize,omitempty"`
	StartRPC       *bool        `json:"startRPC,omitempty"`
	Storage        Storage      `json:"storage,omitempty"`
	MinimumDiskGB  *int         `json:"minimumDiskGB,omitempty"`
}

// CassandraStatus defines the status of the cassandra object.
// +k8s:openapi-gen=true
type CassandraStatus struct {
	Status    `json:",inline"`
	Nodes     map[string]string    `json:"nodes,omitempty"`
	Ports     CassandraStatusPorts `json:"ports,omitempty"`
	ClusterIP string               `json:"clusterIP,omitempty"`
}

// CassandraStatusPorts defines the status of the ports of the cassandra object.
type CassandraStatusPorts struct {
	Port    string `json:"port,omitempty"`
	CqlPort string `json:"cqlPort,omitempty"`
	JmxPort string `json:"jmxPort,omitempty"`
}

// CassandraList contains a list of Cassandra.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CassandraList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cassandra `json:"items"`
}

var CassandraDefaultContainers = []*Container{
	{
		Name:  "cassandra",
		Image: "cassandra:3.11.4",
	},
	{
		Name:  "init",
		Image: "python:3.8.2-alpine",
	},
}

var DefaultCassandra = CassandraConfiguration{
	Containers: CassandraDefaultContainers,
}

func init() {
	SchemeBuilder.Register(&Cassandra{}, &CassandraList{})
}

// InstanceConfiguration creates the cassandra instance configuration.
func (c *Cassandra) InstanceConfiguration(request reconcile.Request,
	podList *corev1.PodList,
	client client.Client) error {
	instanceType := "cassandra"
	instanceConfigMapName := request.Name + "-" + instanceType + "-configmap"
	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: instanceConfigMapName, Namespace: request.Namespace},
		configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}
	cassandraConfig := c.ConfigurationParameters()
	cassandraSecret := &corev1.Secret{}
	if err = client.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-secret", Namespace: request.Namespace}, cassandraSecret); err != nil {
		return err
	}

	seedsListString := strings.Join(c.seeds(podList), ",")

	configNodesInformation, err := NewConfigClusterConfiguration(c.Labels["contrail_cluster"], request.Namespace, client)
	if err != nil {
		return err
	}

	for idx := range podList.Items {
		var cassandraConfigBuffer bytes.Buffer
		configtemplates.CassandraConfig.Execute(&cassandraConfigBuffer, struct {
			ClusterName         string
			Seeds               string
			StoragePort         string
			SslStoragePort      string
			ListenAddress       string
			BroadcastAddress    string
			CqlPort             string
			StartRPC            string
			RPCPort             string
			JmxLocalPort        string
			RPCAddress          string
			RPCBroadcastAddress string
			KeystorePassword    string
			TruststorePassword  string
		}{
			ClusterName:         cassandraConfig.ClusterName,
			Seeds:               seedsListString,
			StoragePort:         strconv.Itoa(*cassandraConfig.StoragePort),
			SslStoragePort:      strconv.Itoa(*cassandraConfig.SslStoragePort),
			ListenAddress:       podList.Items[idx].Status.PodIP,
			BroadcastAddress:    podList.Items[idx].Status.PodIP,
			CqlPort:             strconv.Itoa(*cassandraConfig.CqlPort),
			StartRPC:            "true",
			RPCPort:             strconv.Itoa(*cassandraConfig.Port),
			JmxLocalPort:        strconv.Itoa(*cassandraConfig.JmxLocalPort),
			RPCAddress:          podList.Items[idx].Status.PodIP,
			RPCBroadcastAddress: podList.Items[idx].Status.PodIP,
			KeystorePassword:    string(cassandraSecret.Data["keystorePassword"]),
			TruststorePassword:  string(cassandraSecret.Data["truststorePassword"]),
		})
		cassandraConfigString := cassandraConfigBuffer.String()

		var cassandraCqlShrcBuffer bytes.Buffer
		configtemplates.CassandraCqlShrc.Execute(&cassandraCqlShrcBuffer, struct {
			CAFilePath string
		}{
			CAFilePath: certificates.SignerCAFilepath,
		})
		cassandraCqlShrcConfigString := cassandraCqlShrcBuffer.String()

		collectorEndpointList := configtemplates.EndpointList(configNodesInformation.CollectorServerIPList, configNodesInformation.CollectorPort)
		collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")
		var nodeManagerConfigBuffer bytes.Buffer
		configtemplates.CassandraNodemanagerConfig.Execute(&nodeManagerConfigBuffer, struct {
			ListenAddress       string
			Hostname            string
			CollectorServerList string
			CqlPort             string
			JmxLocalPort        string
			CAFilePath          string
			MinimumDiskGB       int
			LogLevel            string
		}{
			ListenAddress:       podList.Items[idx].Status.PodIP,
			Hostname:            podList.Items[idx].Annotations["hostname"],
			CollectorServerList: collectorEndpointListSpaceSeparated,
			CqlPort:             strconv.Itoa(*cassandraConfig.CqlPort),
			JmxLocalPort:        strconv.Itoa(*cassandraConfig.JmxLocalPort),
			CAFilePath:          certificates.SignerCAFilepath,
			MinimumDiskGB:       *cassandraConfig.MinimumDiskGB,
			// TODO: move to params
			LogLevel: "SYS_DEBUG",
		})
		nodemanagerConfigString := nodeManagerConfigBuffer.String()
		if configMapInstanceDynamicConfig.Data == nil {
			configMapInstanceDynamicConfig.Data = map[string]string{}
		}
		configMapInstanceDynamicConfig.Data["cassandra."+podList.Items[idx].Status.PodIP+".yaml"] = cassandraConfigString
		configMapInstanceDynamicConfig.Data["cqlshrc."+podList.Items[idx].Status.PodIP] = cassandraCqlShrcConfigString
		configMapInstanceDynamicConfig.Data["nodemanager."+podList.Items[idx].Status.PodIP] = nodemanagerConfigString

		err = client.Update(context.TODO(), configMapInstanceDynamicConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateConfigMap creates a configmap for cassandra service.
func (c *Cassandra) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"cassandra",
		c)
}

// CreateSecret creates a secret.
func (c *Cassandra) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"cassandra",
		c)
}

// PrepareSTS prepares the intended deployment for the Cassandra object.
func (c *Cassandra) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "cassandra", request, scheme, c, false)
}

// AddVolumesToIntendedSTS adds volumes to the Cassandra deployment.
func (c *Cassandra) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Cassandra) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// SetPodsToReady sets Cassandra PODs to ready.
func (c *Cassandra) SetPodsToReady(podIPList *corev1.PodList, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// CreateSTS creates the STS.
func (c *Cassandra) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) error {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Cassandra) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client, strategy string) error {
	return UpdateSTS(sts, instanceType, request, reconcileClient, strategy)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Cassandra) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) (*corev1.PodList, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, &c.Spec.CommonConfiguration, request, reconcileClient, true, true, false, false, false, false)
}

//PodsCertSubjects gets list of Cassandra pods certificate subjets which can be passed to the certificate API
func (c *Cassandra) PodsCertSubjects(podList *corev1.PodList, serviceIP string) []certificates.CertificateSubject {
	altIPs := PodAlternativeIPs{ServiceIP: serviceIP}
	return PodsCertSubjects(podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetInstanceActive sets the Cassandra instance to active.
func (c *Cassandra) SetInstanceActive(sts *appsv1.StatefulSet, request reconcile.Request, client client.Client) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace}, sts); err != nil {
		return err
	}
	acceptableReadyReplicaCnt := *sts.Spec.Replicas/2 + 1
	c.Status.Active = sts.Status.ReadyReplicas >= acceptableReadyReplicaCnt
	return client.Status().Update(context.TODO(), c)
}

// ManageNodeStatus manages the status of the Cassandra nodes.
func (c *Cassandra) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) error {
	c.Status.Nodes = podNameIPMap
	cassandraConfig := c.ConfigurationParameters()
	c.Status.Ports.Port = strconv.Itoa(*cassandraConfig.Port)
	c.Status.Ports.CqlPort = strconv.Itoa(*cassandraConfig.CqlPort)
	c.Status.Ports.JmxPort = strconv.Itoa(*cassandraConfig.JmxLocalPort)
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}

// QuerySTS queries the Cassandra STS
func (c *Cassandra) QuerySTS(name string, namespace string, reconcileClient client.Client) (*appsv1.StatefulSet, error) {
	return QuerySTS(name, namespace, reconcileClient)
}

// IsScheduled returns true if instance is scheduled on all pods.
func (c *Cassandra) IsScheduled(name string, namespace string, client client.Client) bool {
	if sts, _ := c.QuerySTS(name+"-"+"cassandra"+"-statefulset", namespace, client); sts != nil {
		log.WithName("Cassandra").Info("IsScheduled", "sts.Spec.Replicas", sts.Spec.Replicas, "sts.Status", sts.Status)
		return sts.Status.CurrentReplicas == *sts.Spec.Replicas
	}
	return false
}

// IsActive returns true if instance is active.
func (c *Cassandra) IsActive(name string, namespace string, client client.Client) bool {
	instance := &Cassandra{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return false
	}
	return instance.Status.Active
}

// IsUpgrading returns true if instance is upgrading.
func (c *Cassandra) IsUpgrading(name string, namespace string, client client.Client) bool {
	instance := &Cassandra{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return false
	}
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name + "-" + "cassandra" + "-statefulset", Namespace: namespace}, sts)
	if err != nil {
		return false
	}
	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return true
	}
	return false
}

// ConfigurationParameters sets the default for the configuration parameters.
func (c *Cassandra) ConfigurationParameters() *CassandraConfiguration {
	cassandraConfiguration := &CassandraConfiguration{}
	var port int
	var cqlPort int
	var jmxPort int
	var storagePort int
	var sslStoragePort int
	var minimumDiskGB int

	if c.Spec.ServiceConfiguration.Storage.Path == "" {
		cassandraConfiguration.Storage.Path = "/mnt/cassandra"
	} else {
		cassandraConfiguration.Storage.Path = c.Spec.ServiceConfiguration.Storage.Path
	}
	if c.Spec.ServiceConfiguration.Storage.Size == "" {
		cassandraConfiguration.Storage.Size = "5Gi"
	} else {
		cassandraConfiguration.Storage.Size = c.Spec.ServiceConfiguration.Storage.Size
	}
	if c.Spec.ServiceConfiguration.Port != nil {
		port = *c.Spec.ServiceConfiguration.Port
	} else {
		port = CassandraPort
	}
	cassandraConfiguration.Port = &port
	if c.Spec.ServiceConfiguration.CqlPort != nil {
		cqlPort = *c.Spec.ServiceConfiguration.CqlPort
	} else {
		cqlPort = CassandraCqlPort
	}
	cassandraConfiguration.CqlPort = &cqlPort
	if c.Spec.ServiceConfiguration.JmxLocalPort != nil {
		jmxPort = *c.Spec.ServiceConfiguration.JmxLocalPort
	} else {
		jmxPort = CassandraJmxLocalPort
	}
	cassandraConfiguration.JmxLocalPort = &jmxPort
	if c.Spec.ServiceConfiguration.StoragePort != nil {
		storagePort = *c.Spec.ServiceConfiguration.StoragePort
	} else {
		storagePort = CassandraStoragePort
	}
	cassandraConfiguration.StoragePort = &storagePort
	if c.Spec.ServiceConfiguration.SslStoragePort != nil {
		sslStoragePort = *c.Spec.ServiceConfiguration.SslStoragePort
	} else {
		sslStoragePort = CassandraSslStoragePort
	}
	cassandraConfiguration.SslStoragePort = &sslStoragePort
	if cassandraConfiguration.ClusterName == "" {
		cassandraConfiguration.ClusterName = "ContrailConfigDB"
	}
	if cassandraConfiguration.ListenAddress == "" {
		cassandraConfiguration.ListenAddress = "auto"
	}
	if c.Spec.ServiceConfiguration.MinimumDiskGB != nil {
		minimumDiskGB = *c.Spec.ServiceConfiguration.MinimumDiskGB
	} else {
		minimumDiskGB = CassandraMinimumDiskGB
	}
	cassandraConfiguration.MinimumDiskGB = &minimumDiskGB

	return cassandraConfiguration
}

func (c *Cassandra) seeds(podList *corev1.PodList) []string {
	pods := make([]corev1.Pod, len(podList.Items))
	copy(pods, podList.Items)
	sort.SliceStable(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

	var seeds []string
	for _, pod := range pods {
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				seeds = append(seeds, pod.Status.PodIP)
				break
			}
		}
	}

	if len(seeds) != 0 {
		numberOfSeeds := (len(seeds) - 1) / 2
		seeds = seeds[:numberOfSeeds+1]
	} else if len(pods) > 0 {
		seeds = []string{pods[0].Status.PodIP}
	}

	return seeds
}

// GetConfigNodes requests config api nodes
func (c *Cassandra) GetConfigNodes(request reconcile.Request, clnt client.Client) ([]string, error) {
	cfg, err := NewConfigClusterConfiguration(c.Labels["contrail_cluster"], request.Namespace, clnt)
	if err != nil {
		return nil, err
	}
	return cfg.APIServerIPList, err
}

// EnvProvisionerConfigMapData creates provision configmap
func (c *Cassandra) EnvProvisionerConfigMapData(request reconcile.Request, clnt client.Client) (map[string]string, error) {
	data := make(map[string]string)
	data["SSL_ENABLE"] = "True"
	data["SERVER_CA_CERTFILE"] = certificates.SignerCAFilepath
	data["SERVER_CERTFILE"] = "/etc/certificates/server-$(POD_IP).crt"
	data["SERVER_KEYFILE"] = "/etc/certificates/server-key-$(POD_IP).pem"

	configNodes, err := c.GetConfigNodes(request, clnt)
	if err != nil {
		return nil, err
	}
	data["CONFIG_NODES"] = configtemplates.JoinListWithSeparator(configNodes, ",")
	return data, nil
}
