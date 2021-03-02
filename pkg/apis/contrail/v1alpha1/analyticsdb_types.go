package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object


// AnalyticsDB is the Schema for the analyticsdb API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=analyticsdb,scope=Namespaced
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready_Replicas",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type AnalyticsDB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyticsDBSpec   `json:"spec,omitempty"`
	Status AnalyticsDBStatus `json:"status,omitempty"`
}

// AnalyticsDBSpec is the Spec for the AnalyticsDB API.
// +k8s:openapi-gen=true
type AnalyticsDBSpec struct {
	CommonConfiguration  PodConfiguration    `json:"commonConfiguration,omitempty"`
	ServiceConfiguration AnalyticsDBConfiguration `json:"serviceConfiguration"`
}

// AnalyticsDBConfiguration is the Spec for the AnalyticsDB API.
// +k8s:openapi-gen=true
type AnalyticsDBConfiguration struct {
	AnalyticsdbPort             *int               `json:"analyticsdbPort,omitempty"`
	AnalyticsdbIntrospectPort   *int               `json:"analyticsdbIntrospectPort,omitempty"`
	Containers                  []*Container       `json:"containers,omitempty"`
	AnalyticsInstance           string             `json:"analyticsInstance,omitempty"`
	CassandraInstance           string             `json:"cassandraInstance,omitempty"`
	ZookeeperInstance           string             `json:"zookeeperInstance,omitempty"`
	RabbitmqInstance            string             `json:"rabbitmqInstance,omitempty"`
	RabbitmqUser                string             `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword            string             `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost               string             `json:"rabbitmqVhost,omitempty"`
	LogLevel                    string             `json:"logLevel,omitempty"`
	Storage                     Storage            `json:"storage,omitempty"`
}

// AnalyticsDBStatus status of AnalyticsDB
// +k8s:openapi-gen=true
type AnalyticsDBStatus struct {
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}

// AnalyticsDBList contains a list of AnalyticsDB.
// +k8s:openapi-gen=true
type AnalyticsDBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AnalyticsDB `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AnalyticsDB{}, &AnalyticsDBList{})
}

// InstanceConfiguration configures and updates configmaps
func (c *AnalyticsDB) InstanceConfiguration(configMapName string,
	request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {

	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		c.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		c.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(c.Spec.ServiceConfiguration.AnalyticsInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: request.Namespace}, rabbitmqSecret)
		if err != nil {
			return err
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	analyticsdbConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = analyticsdbConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = analyticsdbConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = analyticsdbConfig.RabbitmqVhost
	}
	var apiServerList string
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	cassandraCQLEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.CQLPort)
	cassandraCQLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraCQLEndpointList, " ")
	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")

	var redisServerSpaceSeparatedList string
	redisServerSpaceSeparatedList = strings.Join(podIPList, ":6379 ") + ":6379"

	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var queryEngineBuffer bytes.Buffer
		err = configtemplates.QueryEngineConfig.Execute(&queryEngineBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CassandraServerList      string
			CollectorServerList      string
			RedisServerList          string
			CAFilePath               string
			AnalyticsDataTTL         string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RedisServerList:          redisServerSpaceSeparatedList,
			CAFilePath:               certificates.SignerCAFilepath,
			AnalyticsDataTTL:         strconv.Itoa(analyticsNodesInformation.AnalyticsDataTTL),
			LogLevel:                 analyticsdbConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["queryengine."+podIP] = queryEngineBuffer.String()

		var nodemanagerBuffer bytes.Buffer
		err = configtemplates.AnalyticsDBNodemanagerConfig.Execute(&nodemanagerBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorServerList      string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 analyticsdbConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analyticsdb-nodemgr.conf."+podIP] = nodemanagerBuffer.String()
		// empty env as no db tracking
		data["analyticsdb-nodemgr.env."+podIP] = ""
	}

	configMapInstanceDynamicConfig.Data = data

	// update with nodemanager runner
	nmr := GetNodemanagerRunner()

	configMapInstanceDynamicConfig.Data["analyticsdb-nodemanager-runner.sh"] = nmr

	// update with provisioner analyticsdb
	UpdateProvisionerConfigMapData("analyticsdb-provisioner", apiServerList, configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap makes default empty ConfigMap
func (c *AnalyticsDB) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"analyticsdb",
		c)
}

// CreateSecret creates a secret.
func (c *AnalyticsDB) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"analyticsdb",
		c)
}

// PrepareSTS prepares the intented statefulset for the analyticsdb object
func (c *AnalyticsDB) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "analyticsdb", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the analyticsdb statefulset
func (c *AnalyticsDB) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *AnalyticsDB) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

//CreateSTS creates the STS
func (c *AnalyticsDB) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

//UpdateSTS updates the STS
func (c *AnalyticsDB) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "deleteFirst")
}

// SetInstanceActive sets the AnalyticsDB instance to active
func (c *AnalyticsDB) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
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
func (c *AnalyticsDB) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("analyticsdb", &c.Spec.CommonConfiguration, request, reconcileClient)
}

//PodsCertSubjects gets list of AnalyticsDB pods certificate subjets which can be passed to the certificate API
func (c *AnalyticsDB) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

func (c *AnalyticsDB) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// ManageNodeStatus updates nodes in status
func (c *AnalyticsDB) ManageNodeStatus(podNameIPMap map[string]string, client client.Client) error {
	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}

// IsActive returns true if instance is active
func (c *AnalyticsDB) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// ConfigurationParameters create analyticsdb struct
func (c *AnalyticsDB) ConfigurationParameters() AnalyticsDBConfiguration {
	analyticsdbConfiguration := AnalyticsDBConfiguration{}
	var analyticsdbPort int
	var logLevel string

	if c.Spec.ServiceConfiguration.AnalyticsdbPort != nil {
		analyticsdbPort = *c.Spec.ServiceConfiguration.AnalyticsdbPort
	} else {
		analyticsdbPort = AnalyticsdbPort
	}
	analyticsdbConfiguration.AnalyticsdbPort = &analyticsdbPort

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}
	analyticsdbConfiguration.LogLevel = logLevel
	return analyticsdbConfiguration

}

func (c *AnalyticsDB) SetEndpointInStatus(client client.Client, clusterIP string) error {
	c.Status.Endpoint = clusterIP
	err := client.Status().Update(context.TODO(), c)
	return err
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *AnalyticsDB) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
