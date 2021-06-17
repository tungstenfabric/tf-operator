package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AnalyticsSnmp is the Schema for the Analytics SNMP API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=analyticssnmp,scope=Namespaced
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type AnalyticsSnmp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyticsSnmpSpec   `json:"spec,omitempty"`
	Status AnalyticsSnmpStatus `json:"status,omitempty"`
}

// AnalyticsSnmpList contains a list of AnalyticsSnmp.
// +k8s:openapi-gen=true
type AnalyticsSnmpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AnalyticsSnmp `json:"items"`
}

// AnalyticsSnmpSpec is the Spec for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpSpec struct {
	CommonConfiguration  PodConfiguration           `json:"commonConfiguration,omitempty"`
	ServiceConfiguration AnalyticsSnmpConfiguration `json:"serviceConfiguration"`
}

// AnalyticsSnmpConfiguration is the Spec for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpConfiguration struct {
	LogFilePath                       string       `json:"logFilePath,omitempty"`
	LogLocal                          string       `json:"logLocal,omitempty"`
	SnmpCollectorScanFrequency        *int         `json:"snmpCollectorScanFrequency,omitempty"`
	SnmpCollectorFastScanFrequency    *int         `json:"snmpCollectorFastScanFrequency,omitempty"`
	SnmpCollectorIntrospectListenPort *int         `json:"snmpCollectorIntrospectListenPort,omitempty"`
	SnmpCollectorLogFileName          string       `json:"snmpCollectorLogFileName,omitempty"`
	TopologyScanFrequency             *int         `json:"topologySnmpFrequency,omitempty"`
	TopologyIntrospectListenPort      *int         `json:"topologyIntrospectListenPort,omitempty"`
	TopologyLogFileName               string       `json:"topologyLogFileName,omitempty"`
	Containers                        []*Container `json:"containers,omitempty"`
}

// AnalyticsSnmpStatus is the Status for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpStatus struct {
	CommonStatus `json:",inline"`
}

func init() {
	SchemeBuilder.Register(&AnalyticsSnmp{}, &AnalyticsSnmpList{})
}

// CreateConfigMap creates analytics snmp config map
func (c *AnalyticsSnmp) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {

	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"analyticssnmp",
		c)
}

// InstanceConfiguration create config data
func (c *AnalyticsSnmp) InstanceConfiguration(podList []corev1.Pod, client client.Client,
) (data map[string]string, err error) {
	data, err = make(map[string]string), nil

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(CassandraInstance,
		c.Namespace, client)
	if err != nil {
		return
	}
	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(ZookeeperInstance,
		c.Namespace, client)
	if err != nil {
		return
	}
	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(RabbitmqInstance, c.Namespace, client)
	if err != nil {
		return
	}
	configNodesInformation, err := NewConfigClusterConfiguration(ConfigInstance, c.Namespace, client)
	if err != nil {
		return
	}
	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(AnalyticsInstance, c.Namespace, client)
	if err != nil {
		return
	}

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: c.Namespace}, rabbitmqSecret)
		if err != nil {
			return
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	// Create main common values
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	sort.Strings(rabbitMqSSLEndpointList)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")

	configDbEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	sort.Strings(configDbEndpointList)
	configDbEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configDbEndpointList, " ")

	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	sort.Strings(collectorEndpointList)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")

	configApiEndpointList := configtemplates.EndpointList(configNodesInformation.APIServerIPList, configNodesInformation.APIServerPort)
	sort.Strings(configApiEndpointList)
	configApiIPEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, " ")

	configApiList := make([]string, len(configNodesInformation.APIServerIPList))
	copy(configApiList, configNodesInformation.APIServerIPList)
	sort.Strings(configApiList)
	configApiIPCommaSeparated := configtemplates.JoinListWithSeparator(configApiList, ",")

	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	sort.Strings(zookeeperEndpointList)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")

	logLevel := ConvertLogLevel(c.Spec.CommonConfiguration.LogLevel)

	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var collectorBuffer bytes.Buffer
		err = configtemplates.AnalyticsSnmpCollectorConfig.Execute(&collectorBuffer, struct {
			PodIP                             string
			Hostname                          string
			ListenAddress                     string
			InstrospectListenAddress          string
			SnmpCollectorScanFrequency        string
			SnmpCollectorFastScanFrequency    string
			SnmpCollectorIntrospectListenPort string
			LogFile                           string
			LogLevel                          string
			LogLocal                          string
			CollectorServers                  string
			ZookeeperServers                  string
			ConfigServers                     string
			ConfigDbServerList                string
			CassandraSslCaCertfile            string
			RabbitmqServerList                string
			RabbitmqVhost                     string
			RabbitmqUser                      string
			RabbitmqPassword                  string
			CAFilePath                        string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServers:         collectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListCommaSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               SignerCAFilepath,
			// TODO: move to params
			LogLevel:                 logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["tf-snmp-collector."+podIP] = collectorBuffer.String()

		var topologyBuffer bytes.Buffer
		err = configtemplates.AnalyticsSnmpTopologyConfig.Execute(&topologyBuffer, struct {
			PodIP                            string
			Hostname                         string
			ListenAddress                    string
			InstrospectListenAddress         string
			SnmpTopologyScanFrequency        string
			SnmpTopologyIntrospectListenPort string
			LogFile                          string
			LogLevel                         string
			LogLocal                         string
			CollectorServers                 string
			ZookeeperServers                 string
			AnalyticsServers                 string
			ConfigServers                    string
			ConfigDbServerList               string
			CassandraSslCaCertfile           string
			RabbitmqServerList               string
			RabbitmqVhost                    string
			RabbitmqUser                     string
			RabbitmqPassword                 string
			CAFilePath                       string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServers:         collectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListCommaSeparated,
			AnalyticsServers:         configApiIPEndpointListSpaceSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               SignerCAFilepath,
			// TODO: move to params
			LogLevel:                 logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["tf-topology."+podIP] = topologyBuffer.String()

		// TODO: commonize for all services
		var nodemanagerBuffer bytes.Buffer
		err = configtemplates.NodemanagerConfig.Execute(&nodemanagerBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorServerList      string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			MinimumDiskGB            int
			LogLevel                 string
			LogFile                  string
			LogLocal                 string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               SignerCAFilepath,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			// TODO: move to params
			LogLevel:                 logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analytics-snmp-nodemgr.conf."+podIP] = nodemanagerBuffer.String()
		// empty env as no db tracking
		data["analytics-snmp-nodemgr.env."+podIP] = ""

		// TODO: commonize for all services
		var vnciniBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vnciniBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          configApiIPCommaSeparated,
			APIServerPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
			PodIP:                  podIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vnciniBuffer.String()
	}
	data["analyticssnmp-provisioner.env"] = ProvisionerEnvData(configApiIPCommaSeparated,
		"", "", c.Spec.CommonConfiguration.AuthParameters)

	return
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *AnalyticsSnmp) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

// SetInstanceActive sets instance to active.
func (c *AnalyticsSnmp) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, degradedStatus, sts, request, c)
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *AnalyticsSnmp) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
