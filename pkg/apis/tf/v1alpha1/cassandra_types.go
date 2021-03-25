package v1alpha1

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	Containers          []*Container              `json:"containers,omitempty"`
	ConfigInstance      string                    `json:"configInstance,omitempty"`
	ClusterName         string                    `json:"clusterName,omitempty"`
	ListenAddress       string                    `json:"listenAddress,omitempty"`
	Port                *int                      `json:"port,omitempty"`
	CqlPort             *int                      `json:"cqlPort,omitempty"`
	SslStoragePort      *int                      `json:"sslStoragePort,omitempty"`
	StoragePort         *int                      `json:"storagePort,omitempty"`
	JmxLocalPort        *int                      `json:"jmxLocalPort,omitempty"`
	MaxHeapSize         string                    `json:"maxHeapSize,omitempty"`
	MinHeapSize         string                    `json:"minHeapSize,omitempty"`
	StartRPC            *bool                     `json:"startRPC,omitempty"`
	Storage             Storage                   `json:"storage,omitempty"`
	MinimumDiskGB       *int                      `json:"minimumDiskGB,omitempty"`
	CassandraParameters CassandraConfigParameters `json:"cassandraParameters,omitempty"`
}

// CassandraStatus defines the status of the cassandra object.
// +k8s:openapi-gen=true
type CassandraStatus struct {
	Active        *bool                `json:"active,omitempty"`
	Nodes         map[string]string    `json:"nodes,omitempty"`
	Ports         CassandraStatusPorts `json:"ports,omitempty"`
	ClusterIP     string               `json:"clusterIP,omitempty"`
	ConfigChanged *bool                `json:"configChanged,omitempty"`
}

// CassandraStatusPorts defines the status of the ports of the cassandra object.
type CassandraStatusPorts struct {
	Port    string `json:"port,omitempty"`
	CqlPort string `json:"cqlPort,omitempty"`
	JmxPort string `json:"jmxPort,omitempty"`
}

// CassandraConfigParameters defines additional parameters for Cassandra confgiuration
// +k8s:openapi-gen=true
type CassandraConfigParameters struct {
	CompactionThroughputMbPerSec int `json:"compactionThroughputMbPerSec,omitempty"`
	ConcurrentReads              int `json:"concurrentReads,omitempty"`
	ConcurrentWrites             int `json:"concurrentWrites,omitempty"`
	// +kubebuilder:validation:Enum=heap_buffers;offheap_buffers;offheap_objects
	MemtableAllocationType           string `json:"memtableAllocationType,omitempty"`
	ConcurrentCompactors             int    `json:"concurrentCompactors,omitempty"`
	MemtableFlushWriters             int    `json:"memtableFlushWriters,omitempty"`
	ConcurrentCounterWrites          int    `json:"concurrentCounterWrites,omitempty"`
	ConcurrentMaterializedViewWrites int    `json:"concurrentMaterializedViewWrites,omitempty"`
}

// CassandraList contains a list of Cassandra.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CassandraList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cassandra `json:"items"`
}

var cassandraLog = logf.Log.WithName("controller_cassandra")

func init() {
	SchemeBuilder.Register(&Cassandra{}, &CassandraList{})
}

// InstanceConfiguration creates the cassandra instance configuration.
func (c *Cassandra) InstanceConfiguration(request reconcile.Request,
	podList []corev1.Pod,
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
	if configMapInstanceDynamicConfig.Data == nil {
		return errors.New("configMap data is nil")
	}

	cassandraConfig := c.ConfigurationParameters()
	cassandraSecret := &corev1.Secret{}
	if err = client.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-secret", Namespace: request.Namespace}, cassandraSecret); err != nil {
		return err
	}

	seedsListString := strings.Join(c.seeds(podList), ",")
	cassandraLog.Info("InstanceConfiguration", "seedsListString", seedsListString)

	configNodesInformation, err := NewConfigClusterConfiguration(c.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	for _, pod := range podList {

		var cassandraConfigBuffer bytes.Buffer
		err = configtemplates.CassandraConfig.Execute(&cassandraConfigBuffer, struct {
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
			Parameters          CassandraConfigParameters
		}{
			ClusterName:         cassandraConfig.ClusterName,
			Seeds:               seedsListString,
			StoragePort:         strconv.Itoa(*cassandraConfig.StoragePort),
			SslStoragePort:      strconv.Itoa(*cassandraConfig.SslStoragePort),
			ListenAddress:       pod.Status.PodIP,
			BroadcastAddress:    pod.Status.PodIP,
			CqlPort:             strconv.Itoa(*cassandraConfig.CqlPort),
			StartRPC:            "true",
			RPCPort:             strconv.Itoa(*cassandraConfig.Port),
			JmxLocalPort:        strconv.Itoa(*cassandraConfig.JmxLocalPort),
			RPCAddress:          pod.Status.PodIP,
			RPCBroadcastAddress: pod.Status.PodIP,
			KeystorePassword:    string(cassandraSecret.Data["keystorePassword"]),
			TruststorePassword:  string(cassandraSecret.Data["truststorePassword"]),
			Parameters:          c.Spec.ServiceConfiguration.CassandraParameters,
		})
		if err != nil {
			panic(err)
		}
		cassandraConfigString := cassandraConfigBuffer.String()

		var cassandraCqlShrcBuffer bytes.Buffer
		err = configtemplates.CassandraCqlShrc.Execute(&cassandraCqlShrcBuffer, struct {
			CAFilePath string
		}{
			CAFilePath: certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		cassandraCqlShrcConfigString := cassandraCqlShrcBuffer.String()

		collectorEndpointList := configtemplates.EndpointList(configNodesInformation.CollectorServerIPList, configNodesInformation.CollectorPort)
		collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")
		var nodeManagerConfigBuffer bytes.Buffer
		err = configtemplates.CassandraNodemanagerConfig.Execute(&nodeManagerConfigBuffer, struct {
			ListenAddress            string
			InstrospectListenAddress string
			Hostname                 string
			CollectorServerList      string
			CqlPort                  string
			JmxLocalPort             string
			CAFilePath               string
			MinimumDiskGB            int
			LogLevel                 string
		}{
			ListenAddress:            pod.Status.PodIP,
			InstrospectListenAddress: c.Spec.CommonConfiguration.IntrospectionListenAddress(pod.Status.PodIP),
			Hostname:                 pod.Annotations["hostname"],
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			CqlPort:                  strconv.Itoa(*cassandraConfig.CqlPort),
			JmxLocalPort:             strconv.Itoa(*cassandraConfig.JmxLocalPort),
			CAFilePath:               certificates.SignerCAFilepath,
			MinimumDiskGB:            *cassandraConfig.MinimumDiskGB,
			// TODO: move to params
			LogLevel: "SYS_DEBUG",
		})
		if err != nil {
			panic(err)
		}
		nodemanagerConfigString := nodeManagerConfigBuffer.String()

		apiServerIPListCommaSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, ",")
		var vncAPIConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncAPIConfigBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters *KeystoneAuthParameters
		}{
			APIServerList:          apiServerIPListCommaSeparated,
			APIServerPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             certificates.SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
		})
		if err != nil {
			panic(err)
		}
		vncAPIConfigBufferString := vncAPIConfigBuffer.String()

		// TODO: till analytics db is not separated use api as DBs lists
		var nodemanagerEnvBuffer bytes.Buffer
		err = configtemplates.NodemanagerEnv.Execute(&nodemanagerEnvBuffer, struct {
			ConfigDBNodes    string
			AnalyticsDBNodes string
		}{
			ConfigDBNodes:    apiServerIPListCommaSeparated,
			AnalyticsDBNodes: apiServerIPListCommaSeparated,
		})
		if err != nil {
			panic(err)
		}
		nodemanagerEnvString := nodemanagerEnvBuffer.String()

		configMapInstanceDynamicConfig.Data["cassandra."+pod.Status.PodIP+".yaml"] = cassandraConfigString
		configMapInstanceDynamicConfig.Data["cqlshrc."+pod.Status.PodIP] = cassandraCqlShrcConfigString
		// wait for api, nodemgr container will wait for config files be ready
		if apiServerIPListCommaSeparated != "" {
			configMapInstanceDynamicConfig.Data["vnc_api_lib.ini."+pod.Status.PodIP] = vncAPIConfigBufferString
			configMapInstanceDynamicConfig.Data["database-nodemgr.conf."+pod.Status.PodIP] = nodemanagerConfigString
			configMapInstanceDynamicConfig.Data["database-nodemgr.env."+pod.Status.PodIP] = nodemanagerEnvString
		}
	}

	configNodes, err := c.GetConfigNodes(request, client)
	if err != nil {
		return err
	}
	UpdateProvisionerConfigMapData("database-provisioner", configtemplates.JoinListWithSeparator(configNodes, ","), configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap creates a configmap for cassandra service.
func (c *Cassandra) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {

	configMap, err := CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"cassandra",
		c)
	if err != nil {
		return nil, err
	}

	configMap.Data["database-nodemanager-runner.sh"] = GetNodemanagerRunner()

	configNodes, err := c.GetConfigNodes(request, client)
	if err != nil {
		return nil, err
	}
	UpdateProvisionerConfigMapData("database-provisioner", configtemplates.JoinListWithSeparator(configNodes, ","), configMap)

	cassandraSecret := &corev1.Secret{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-secret", Namespace: request.Namespace}, cassandraSecret); err != nil {
		return nil, err
	}
	cassandraConfig := c.ConfigurationParameters()

	var cassandraCommandBuffer bytes.Buffer
	err = configtemplates.CassandraCommandTemplate.Execute(&cassandraCommandBuffer, struct {
		KeystorePassword   string
		TruststorePassword string
		CAFilePath         string
		JmxLocalPort       string
	}{
		KeystorePassword:   string(cassandraSecret.Data["keystorePassword"]),
		TruststorePassword: string(cassandraSecret.Data["truststorePassword"]),
		CAFilePath:         certificates.SignerCAFilepath,
		JmxLocalPort:       strconv.Itoa(*cassandraConfig.JmxLocalPort),
	})
	if err != nil {
		panic(err)
	}
	cassandraCommandString := cassandraCommandBuffer.String()
	configMap.Data["cassandra-run.sh"] = cassandraCommandString

	if err = client.Update(context.TODO(), configMap); err != nil {
		return nil, err
	}
	return configMap, nil
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
	podMgmtPolicyParallel := true
	return PrepareSTS(sts, commonConfiguration, "cassandra", request, scheme, c, podMgmtPolicyParallel)
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
func (c *Cassandra) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// CreateSTS creates the STS.
func (c *Cassandra) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Cassandra) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "rolling")
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Cassandra) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient)
}

//PodsCertSubjects gets list of Cassandra pods certificate subjets which can be passed to the certificate API
func (c *Cassandra) PodsCertSubjects(domain string, podList []corev1.Pod, serviceIP string) []certificates.CertificateSubject {
	altIPs := PodAlternativeIPs{ServiceIP: serviceIP}
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// QuerySTS queries the Cassandra STS
func (c *Cassandra) QuerySTS(name string, namespace string, reconcileClient client.Client) (*appsv1.StatefulSet, error) {
	return QuerySTS(name, namespace, reconcileClient)
}

// IsActive returns true if instance is active.
func (c *Cassandra) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
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
		cassandraConfiguration.Storage.Path = "/var/lib/cassandra"
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

func (c *Cassandra) seeds(podList []corev1.Pod) []string {
	pods := make([]corev1.Pod, len(podList))
	copy(pods, podList)
	sort.SliceStable(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })
	var seeds []string
	for _, pod := range pods {
		seeds = append(seeds, pod.Status.PodIP)
	}
	if len(seeds) > 2 {
		numberOfSeeds := (len(seeds) - 1) / 2
		seeds = seeds[:numberOfSeeds+1]
	}
	return seeds
}

// UpdateStatus manages the status of the Cassandra nodes.
func (c *Cassandra) UpdateStatus(cassandraConfig *CassandraConfiguration, podNameIPMap map[string]string, sts *appsv1.StatefulSet) bool {
	log := cassandraLog.WithName("UpdateStatus")
	changed := false

	if !reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		log.Info("Nodes", "new", podNameIPMap, "old", c.Status.Nodes)
		c.Status.Nodes = podNameIPMap
		changed = true
	}

	p := strconv.Itoa(*cassandraConfig.Port)
	if c.Status.Ports.Port != p {
		log.Info("Port", "new", p, "old", c.Status.Ports.Port)
		c.Status.Ports.Port = p
		changed = true
	}
	p = strconv.Itoa(*cassandraConfig.CqlPort)
	if c.Status.Ports.CqlPort != p {
		log.Info("CqlPort", "new", p, "old", c.Status.Ports.CqlPort)
		c.Status.Ports.CqlPort = p
		changed = true
	}
	p = strconv.Itoa(*cassandraConfig.JmxLocalPort)
	if c.Status.Ports.JmxPort != p {
		log.Info("JmxPort", "new", p, "old", c.Status.Ports.JmxPort)
		c.Status.Ports.JmxPort = p
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

// GetConfigNodes requests config api nodes
func (c *Cassandra) GetConfigNodes(request reconcile.Request, clnt client.Client) ([]string, error) {
	cfg, err := NewConfigClusterConfiguration(c.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, clnt)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	return cfg.APIServerIPList, nil
}

// ConfigDataDiff compare configmaps and retursn list of services to be reloaded
func (c *Cassandra) ConfigDataDiff(pod *corev1.Pod, v1 *corev1.ConfigMap, v2 *corev1.ConfigMap) []string {
	podIP := pod.Status.PodIP
	srvMap := map[string][]string{
		"cassandra":   {"cassandra." + podIP + ".yaml", "cqlshrc." + podIP},
		"nodemanager": {"database-nodemgr.conf." + podIP, "database-nodemgr.env." + podIP, "vnc_api_lib.ini." + podIP},
	}
	var res []string
	for srv, maps := range srvMap {
		for _, d := range maps {
			if v1.Data[d] != v2.Data[d] {
				res = append(res, srv)
				break
			}
		}
	}

	return res
}

// ReloadServices reload servics for pods after changed configs
func (c *Cassandra) ReloadServices(srvList map[*corev1.Pod][]string, clnt client.Client) error {
	restartList := make(map[*corev1.Pod][]string)
	reloadList := make(map[*corev1.Pod][]string)
	for pod := range srvList {
		for idx := range srvList[pod] {
			// cassandra needs restart
			if srvList[pod][idx] == "cassandra" {
				restartList[pod] = append(restartList[pod], srvList[pod][idx])
			} else {
				reloadList[pod] = append(reloadList[pod], srvList[pod][idx])
			}
		}
	}
	err1 := RestartServices(restartList, clnt, cassandraLog)
	err2 := ReloadServices(reloadList, clnt, cassandraLog)
	if err1 != nil && err2 != nil {
		return &CombinedError{[]error{err1, err2}}
	}
	if err1 != nil {
		return err1
	}
	return err2
}
