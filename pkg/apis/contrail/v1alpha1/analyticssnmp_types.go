package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"

	configtemplates "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
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
	Containers []*Container `json:"containers,omitempty"`
	// Dependeces
	CassandraInstance string `json:"cassandraInstance,omitempty"`
	ZookeeperInstance string `json:"zookeeperInstance,omitempty"`
	// Service parameters: common
	LogFilePath string `json:"logFilePath,omitempty"`
	LogLevel    string `json:"logLevel,omitempty"`
	LogLocal    string `json:"logLocal,omitempty"`
	// Service parameters: snmp-collector
	SnmpCollectorScanFrequency        *int   `json:"snmpCollectorScanFrequency,omitempty"`
	SnmpCollectorFastScanFrequency    *int   `json:"snmpCollectorFastScanFrequency,omitempty"`
	SnmpCollectorIntrospectListenPort *int   `json:"snmpCollectorIntrospectListenPort,omitempty"`
	SnmpCollectorLogFileName          string `json:"snmpCollectorLogFileName,omitempty"`
	// Service parameters: topology
	TopologyScanFrequency        *int   `json:"topologySnmpFrequency,omitempty"`
	TopologyIntrospectListenPort *int   `json:"topologyIntrospectListenPort,omitempty"`
	TopologyLogFileName          string `json:"topologyLogFileName,omitempty"`
	// Service parameters: nodemanager
	NodemanagerLogFileName string `json:"nodemanagerLogFileName,omitempty"`
}

// AnalyticsSnmpStatus is the Status for the Analytics SNMP API.
// +k8s:openapi-gen=true
type AnalyticsSnmpStatus struct {
	Active *bool `json:"active,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AnalyticsSnmp{}, &AnalyticsSnmpList{})
}

// GetDataForConfigMap
func (c *AnalyticsSnmp) GetDataForConfigMap(podIpList *corev1.PodList, request reconcile.Request, client client.Client, logger logr.Logger) (map[string]string, error) {
	if logger == nil {
		logger = logf.Log.WithName("snmp_configmap")
	}

	// Get Cassandra information
	cassandraNodesInformation, err := NewCassandraClusterConfiguration(c.Spec.ServiceConfiguration.CassandraInstance,
		request.Namespace, client)
	if err != nil {
		logger.Error(err, "Cassandra information not found.")
		return nil, err
	}

	// Get ZooKeeper information
	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(c.Spec.ServiceConfiguration.ZookeeperInstance,
		request.Namespace, client)
	if err != nil {
		logger.Error(err, "Zookeeper information not found.")
		return nil, err
	}

	// Get RabbitMQ information
	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(c.Labels["contrail_cluster"],
		request.Namespace, client)
	if err != nil {
		logger.Error(err, "RabbitMQ information not found.")
		return nil, err
	}

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		err = client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: request.Namespace}, rabbitmqSecret)
		if err != nil {
			logger.Error(err, "RabbitMQ Secret not found.")
			return nil, err
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	// Get Config information
	configNodesInformation, err := NewConfigClusterConfiguration(c.Labels["contrail_cluster"],
		request.Namespace, client)
	if err != nil {
		logger.Error(err, "Config information not found.")
		return nil, err
	}

	// Create main common values
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	sort.Strings(rabbitMqSSLEndpointList)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")

	configDbEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	sort.Strings(configDbEndpointList)
	configDbEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configDbEndpointList, " ")

	configCollectorEndpointList := configtemplates.EndpointList(configNodesInformation.CollectorServerIPList, configNodesInformation.CollectorPort)
	sort.Strings(configCollectorEndpointList)
	configCollectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configCollectorEndpointList, " ")

	configApiEndpointList := configtemplates.EndpointList(configNodesInformation.APIServerIPList, configNodesInformation.APIServerPort)
	sort.Strings(configApiEndpointList)
	configApiIPEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, " ")

	configApiList := make([]string, len(configNodesInformation.APIServerIPList))
	copy(configApiList, configNodesInformation.APIServerIPList)
	sort.Strings(configApiList)
	configApiIPCommaSeparated := configtemplates.JoinListWithSeparator(configApiList, ",")

	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	sort.Strings(zookeeperEndpointList)
	zookeeperEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, " ")

	var data = make(map[string]string)
	for _, pod := range podIpList.Items {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var collectorBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpCollectorConfig.Execute(&collectorBuffer, struct {
			PodIP                             string
			Hostname                          string
			ListenAddress                     string
			InstrospectListenAddress          string
			SnmpCollectorScanFrequency        string
			SnmpCollectorFastScanFrequency    string
			HttpServerList                    string
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
			CollectorServers:         configCollectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListSpaceSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   certificates.SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               certificates.SignerCAFilepath,
			// TODO: move to params
			LogLevel:                 "SYS_DEBUG",
		})
		data["tf-snmp-collector."+podIP] = collectorBuffer.String()

		var topologyBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpTopologyConfig.Execute(&topologyBuffer, struct {
			PodIP                            string
			Hostname                         string
			ListenAddress                    string
			InstrospectListenAddress         string
			SnmpTopologyScanFrequency        string
			HttpServerList                   string
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
			CollectorServers:         configCollectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListSpaceSeparated,
			AnalyticsServers:         configApiIPEndpointListSpaceSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			CassandraSslCaCertfile:   certificates.SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			CAFilePath:               certificates.SignerCAFilepath,
			// TODO: move to params
			LogLevel:                 "SYS_DEBUG",
		})
		data["tf-topology."+podIP] = topologyBuffer.String()

		var nodemanagerBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpNodemanagerConfig.Execute(&nodemanagerBuffer, struct {
			PodIP                    string
			Hostname                 string
			ListenAddress            string
			InstrospectListenAddress string
			LogFile                  string
			LogLevel                 string
			LogLocal                 string
			CassandraPort            string
			CassandraJmxPort         string
			CAFilePath               string
			CollectorServerList      string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			CollectorServerList:      configCollectorEndpointListSpaceSeparated,
			// TODO: move to params
			LogLevel:                 "SYS_DEBUG",
		})
		data["nodemanager."+podIP] = nodemanagerBuffer.String()

		var vnciniBuffer bytes.Buffer
		configtemplates.AnalyticsSnmpVncConfig.Execute(&vnciniBuffer, struct {
			ConfigNodes   string
			ConfigApiPort string
			CAFilePath    string
		}{
			ConfigNodes:   configApiIPCommaSeparated,
			ConfigApiPort: strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:    certificates.SignerCAFilepath,
		})
		data["vnc."+podIP] = vnciniBuffer.String()
	}

	return data, nil
}
