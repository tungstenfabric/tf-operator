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

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AnalyticsAlarm is the Schema for the Analytics Alarm API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=analyticsalarm,scope=Namespaced
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type AnalyticsAlarm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyticsAlarmSpec   `json:"spec,omitempty"`
	Status AnalyticsAlarmStatus `json:"status,omitempty"`
}

// AnalyticsAlarmList contains a list of AnalyticsAlarm.
// +k8s:openapi-gen=true
type AnalyticsAlarmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AnalyticsAlarm `json:"items"`
}

// AnalyticsAlarmSpec is the Spec for the Analytics Alarm API.
// +k8s:openapi-gen=true
type AnalyticsAlarmSpec struct {
	CommonConfiguration  PodConfiguration            `json:"commonConfiguration,omitempty"`
	ServiceConfiguration AnalyticsAlarmConfiguration `json:"serviceConfiguration"`
}

// AnalyticsAlarmConfiguration is the Spec for the Analytics Alarm API.
// +k8s:openapi-gen=true
type AnalyticsAlarmConfiguration struct {
	LogFilePath                    string       `json:"logFilePath,omitempty"`
	LogLocal                       string       `json:"logLocal,omitempty"`
	AlarmgenRedisAggregateDbOffset *int         `json:"alarmgenRedisAggregateDbOffset,omitempty"`
	AlarmgenPartitions             *int         `json:"alarmgenPartitions,omitempty"`
	AlarmgenIntrospectListenPort   *int         `json:"alarmgenIntrospectListenPort,omitempty"`
	AlarmgenLogFileName            string       `json:"alarmgenLogFileName,omitempty"`
	Containers                     []*Container `json:"containers,omitempty"`
}

// AnalyticsAlarmStatus is the Status for the Analytics Alarm API.
// +k8s:openapi-gen=true
type AnalyticsAlarmStatus struct {
	CommonStatus `json:",inline"`
}

func init() {
	SchemeBuilder.Register(&AnalyticsAlarm{}, &AnalyticsAlarmList{})
}

// CreateConfigMap creates analytics alarm config map
func (c *AnalyticsAlarm) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {

	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"analyticsalarm",
		c)
}

// InstanceConfiguration create config data
func (c *AnalyticsAlarm) InstanceConfiguration(podList []corev1.Pod, client client.Client,
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
	redisNodesInformation, err := NewRedisClusterConfiguration(RedisInstance, c.Namespace, client)
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
	zookeeperEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, " ")
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")

	zookeeperListLength := len(strings.Split(zookeeperEndpointListSpaceSeparated, " "))
	replicationFactor := 1
	minInsyncReplicas := 1
	if zookeeperListLength == 2 {
		replicationFactor = 2
	} else if zookeeperListLength > 2 {
		replicationFactor = 3
		minInsyncReplicas = 2
	}

	var kafkaServerSpaceSeparatedList string
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	kafkaServerSpaceSeparatedList = strings.Join(podIPList, ":9092 ") + ":9092"
	analyticsAlarmNodes := strings.Join(podIPList, ",")

	kafkaSecret := &corev1.Secret{}
	if err = client.Get(context.TODO(), types.NamespacedName{Name: c.Name + "-secret", Namespace: c.Namespace}, kafkaSecret); err != nil {
		return
	}

	redisEndpointList := configtemplates.EndpointList(redisNodesInformation.ServerIPList, redisNodesInformation.ServerPort)
	redisEndpointListSpaceSpearated := configtemplates.JoinListWithSeparator(redisEndpointList, " ")

	logLevel := ConvertLogLevel(c.Spec.CommonConfiguration.LogLevel)

	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var alarmBuffer bytes.Buffer
		err = configtemplates.AnalyticsAlarmgenConfig.Execute(&alarmBuffer, struct {
			PodIP                          string
			Hostname                       string
			ListenAddress                  string
			InstrospectListenAddress       string
			AlarmgenRedisAggregateDbOffset string
			AlarmgenPartitions             string
			AlarmgenIntrospectListenPort   string
			LogFile                        string
			LogLevel                       string
			LogLocal                       string
			CollectorServers               string
			ZookeeperServers               string
			ConfigServers                  string
			ConfigDbServerList             string
			KafkaServers                   string
			CassandraSslCaCertfile         string
			RabbitmqServerList             string
			RabbitmqVhost                  string
			RabbitmqUser                   string
			RabbitmqPassword               string
			RedisServerList                string
			RedisPort                      int
			CAFilePath                     string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServers:         collectorEndpointListSpaceSeparated,
			ZookeeperServers:         zookeeperEndpointListSpaceSeparated,
			ConfigServers:            configApiIPEndpointListSpaceSeparated,
			ConfigDbServerList:       configDbEndpointListSpaceSeparated,
			KafkaServers:             kafkaServerSpaceSeparatedList,
			CassandraSslCaCertfile:   SignerCAFilepath,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqVhost:            rabbitmqSecretVhost,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RedisServerList:          redisEndpointListSpaceSpearated,
			RedisPort:                redisNodesInformation.ServerPort,
			CAFilePath:               SignerCAFilepath,
			// TODO: move to params
			LogLevel: logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["tf-alarm-gen."+podIP] = alarmBuffer.String()

		myidString := pod.Name[len(pod.Name)-1:]
		myidInt, _err := strconv.Atoi(myidString)
		if _err != nil {
			err = _err
			return
		}

		var kafkaBuffer bytes.Buffer
		err = configtemplates.KafkaConfig.Execute(&kafkaBuffer, struct {
			PodIP              string
			BrokerId           string
			Hostname           string
			ZookeeperServers   string
			ReplicationFactor  string
			MinInsyncReplicas  string
			KeystorePassword   string
			TruststorePassword string
			CAFilePath         string
			LogLevel           string
		}{
			PodIP:              podIP,
			BrokerId:           strconv.Itoa(myidInt),
			Hostname:           hostname,
			ZookeeperServers:   zookeeperEndpointListCommaSeparated,
			ReplicationFactor:  strconv.Itoa(replicationFactor),
			MinInsyncReplicas:  strconv.Itoa(minInsyncReplicas),
			KeystorePassword:   string(kafkaSecret.Data["keystorePassword"]),
			TruststorePassword: string(kafkaSecret.Data["truststorePassword"]),
			CAFilePath:         SignerCAFilepath,
			// TODO: move to params
			LogLevel: logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["kafka.config."+podIP] = kafkaBuffer.String()

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
			LogLevel: logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analytics-alarm-nodemgr.conf."+podIP] = nodemanagerBuffer.String()
		// empty env as no db tracking
		data["analytics-alarm-nodemgr.env."+podIP] = ""

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
	clusterNodes := ClusterNodes{ConfigNodes: configApiIPCommaSeparated,
		AnalyticsAlarmNodes: analyticsAlarmNodes}
	data["analyticsalarm-provisioner.env"] = ProvisionerEnvData(&clusterNodes,
		"", c.Spec.CommonConfiguration.AuthParameters)

	return
}

// CreateSecret creates a secret.
func (c *AnalyticsAlarm) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"kafka",
		c)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *AnalyticsAlarm) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

// SetInstanceActive sets instance to active.
func (c *AnalyticsAlarm) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, degradedStatus, sts, request, c)
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *AnalyticsAlarm) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
