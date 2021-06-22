package v1alpha1

import (
	"bytes"
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AAAMode aaa mode
// +k8s:openapi-gen=true
// +kubebuilder:validation:Enum=noauth;rbac
type AAAMode string

const (
	// AAAModeNoAuth no auth
	AAAModeNoAuth AAAMode = "no-auth"
	// AAAModeRBAC auth mode rbac
	AAAModeRBAC AAAMode = "rbac"
	// AAAModeCloudAdmin auth mode cloud-admin
	AAAModeCloudAdmin AAAMode = "cloud-admin"
)

// Config is the Schema for the configs API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=configs,scope=Namespaced
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

// ConfigSpec is the Spec for the Config API.
// +k8s:openapi-gen=true
type ConfigSpec struct {
	CommonConfiguration  PodConfiguration    `json:"commonConfiguration,omitempty"`
	ServiceConfiguration ConfigConfiguration `json:"serviceConfiguration"`
}

// ConfigConfiguration is the Spec for the Config API.
// +k8s:openapi-gen=true
type ConfigConfiguration struct {
	Containers                  []*Container            `json:"containers,omitempty"`
	APIAdminPort                *int                    `json:"apiAdminPort,omitempty"`
	APIPort                     *int                    `json:"apiPort,omitempty"`
	ApiIntrospectPort           *int                    `json:"apiIntrospectPort,omitempty"`
	APIWorkerCount              *int                    `json:"apiWorkerCount,omitempty"`
	SchemaIntrospectPort        *int                    `json:"schemaIntrospectPort,omitempty"`
	DeviceManagerIntrospectPort *int                    `json:"deviceManagerIntrospectPort,omitempty"`
	SvcMonitorIntrospectPort    *int                    `json:"svcMonitorIntrospectPort,omitempty"`
	AAAMode                     AAAMode                 `json:"aaaMode,omitempty"`
	FabricMgmtIP                string                  `json:"fabricMgmtIP,omitempty"`
	LinklocalServiceConfig      *LinklocalServiceConfig `json:"linklocalServiceConfig,omitempty"`
	UseExternalTFTP             *bool                   `json:"useExternalTFTP,omitempty"`
}

// LinklocalServiceConfig is the Spec for link local coniguration
// +k8s:openapi-gen=true
type LinklocalServiceConfig struct {
	IPFabricServiceHost string  `json:"ipFabricServiceHost,omitempty"`
	IPFabricServicePort *int    `json:"ipFabricServicePort,omitempty"`
	Name                *string `json:"name,omitempty"`
	Port                *int    `json:"port,omitempty"`
	IP                  *string `json:"ip,omitempty"`
}

// ConfigStatus status of Config
// +k8s:openapi-gen=true
type ConfigStatus struct {
	CommonStatus `json:",inline"`
	Endpoint     string `json:"endpoint,omitempty"`
}

// ConfigList contains a list of Config.
// +k8s:openapi-gen=true
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

var log = logf.Log.WithName("controller_config")

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}

// InstanceConfiguration configures and updates configmaps
func (c *Config) InstanceConfiguration(podList []corev1.Pod, client client.Client,
) (data map[string]string, err error) {
	data, err = make(map[string]string), nil

	configAuth := c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		CassandraInstance, c.Namespace, client)
	if err != nil {
		return
	}

	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(
		ZookeeperInstance, c.Namespace, client)
	if err != nil {
		return
	}

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		RabbitmqInstance, c.Namespace, client)
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

	configConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = RabbitmqVhost
	}
	var analyticsServerList, apiServerList string
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	apiServerList = strings.Join(podIPList, ",")
	analyticsServerList = strings.Join(analyticsNodesInformation.AnalyticsServerIPList, ",")
	analyticsEndpointList := configtemplates.EndpointList(analyticsNodesInformation.AnalyticsServerIPList, analyticsNodesInformation.AnalyticsServerPort)
	analyticsEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(analyticsEndpointList, " ")
	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")
	cassandraEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	cassandraEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraEndpointList, " ")
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	rabbitmqSSLEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, ",")
	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")

	logLevel := ConvertLogLevel(c.Spec.CommonConfiguration.LogLevel)

	adminPorts := []string{strconv.Itoa(*configConfig.APIAdminPort)}
	introspectPorts := []string{strconv.Itoa(*configConfig.ApiIntrospectPort)}
	for i := 0; i < *configConfig.APIWorkerCount-1; i++ {
		introspectPorts = append(introspectPorts, strconv.Itoa(10000+*configConfig.ApiIntrospectPort+i))
		adminPorts = append(adminPorts, strconv.Itoa(20000+*configConfig.APIAdminPort+i))
	}
	adminPortListSpaceSeparated := configtemplates.JoinListWithSeparator(adminPorts, " ")
	introspectPortListSpaceSeparated := configtemplates.JoinListWithSeparator(introspectPorts, " ")

	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		for i := 0; i < *configConfig.APIWorkerCount; i++ {
			var configApiConfigBuffer bytes.Buffer
			err = configtemplates.ConfigAPIConfig.Execute(&configApiConfigBuffer, struct {
				PodIP                    string
				ListenAddress            string
				ListenPort               string
				InstrospectListenAddress string
				ApiIntrospectPort        string
				CassandraServerList      string
				ZookeeperServerList      string
				RabbitmqServerList       string
				CollectorServerList      string
				RabbitmqUser             string
				RabbitmqPassword         string
				RabbitmqVhost            string
				AuthMode                 AuthenticationMode
				AAAMode                  AAAMode
				LogLevel                 string
				CAFilePath               string
				AdminPort                string
				WorkerId                 string
				IntrospectPortList       string
				AdminPortList            string
			}{
				PodIP:                    podIP,
				ListenAddress:            podIP,
				ListenPort:               strconv.Itoa(*configConfig.APIPort),
				InstrospectListenAddress: instrospectListenAddress,
				ApiIntrospectPort:        introspectPorts[i],
				CassandraServerList:      cassandraEndpointListSpaceSeparated,
				ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
				RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
				CollectorServerList:      collectorEndpointListSpaceSeparated,
				RabbitmqUser:             rabbitmqSecretUser,
				RabbitmqPassword:         rabbitmqSecretPassword,
				RabbitmqVhost:            rabbitmqSecretVhost,
				AuthMode:                 c.Spec.CommonConfiguration.AuthParameters.AuthMode,
				AAAMode:                  configConfig.AAAMode,
				LogLevel:                 logLevel,
				CAFilePath:               SignerCAFilepath,
				AdminPort:                adminPorts[i],
				WorkerId:                 strconv.Itoa(i),
				IntrospectPortList:       introspectPortListSpaceSeparated,
				AdminPortList:            adminPortListSpaceSeparated,
			})
			if err != nil {
				panic(err)
			}
			data["api."+strconv.Itoa(i)+"."+podIP] = configApiConfigBuffer.String()
		}

		if *configConfig.APIWorkerCount > 1 {
			var apiUwsgiIniBuffer bytes.Buffer
			err = configtemplates.ConfigAPIUwsgiIniConfig.Execute(&apiUwsgiIniBuffer, struct {
				APIMaxRequests int
				APIWorkerCount int
				BufferSize     int
				ListenAddress  string
				ListenPort     int
				PodIP          string
			}{
				APIMaxRequests: 1024,
				APIWorkerCount: *configConfig.APIWorkerCount,
				BufferSize:     1024 * 1000 * 16,
				ListenAddress:  podIP,
				ListenPort:     *configConfig.APIPort,
				PodIP:          podIP,
			})
			if err != nil {
				panic(err)
			}
			data["api-uwsgi.ini."+podIP] = apiUwsgiIniBuffer.String()
		}
		var vncApiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncApiConfigBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          apiServerList,
			APIServerPort:          strconv.Itoa(*configConfig.APIPort),
			CAFilePath:             SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
			PodIP:                  podIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vncApiConfigBuffer.String()

		fabricMgmtIP := podIP
		if c.Spec.ServiceConfiguration.FabricMgmtIP != "" {
			fabricMgmtIP = c.Spec.ServiceConfiguration.FabricMgmtIP
		}

		var configDevicemanagerConfigBuffer bytes.Buffer
		err = configtemplates.ConfigDeviceManagerConfig.Execute(&configDevicemanagerConfigBuffer, struct {
			PodIP                       string
			ListenAddress               string
			InstrospectListenAddress    string
			DeviceManagerIntrospectPort string
			ApiServerList               string
			AnalyticsServerList         string
			CassandraServerList         string
			ZookeeperServerList         string
			RabbitmqServerList          string
			CollectorServerList         string
			RabbitmqUser                string
			RabbitmqPassword            string
			RabbitmqVhost               string
			LogLevel                    string
			FabricMgmtIP                string
			CAFilePath                  string
		}{
			PodIP:                       podIP,
			ListenAddress:               podIP,
			InstrospectListenAddress:    instrospectListenAddress,
			DeviceManagerIntrospectPort: strconv.Itoa(*configConfig.DeviceManagerIntrospectPort),
			ApiServerList:               apiServerList,
			AnalyticsServerList:         analyticsServerList,
			CassandraServerList:         cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:         zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:          rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:         collectorEndpointListSpaceSeparated,
			RabbitmqUser:                rabbitmqSecretUser,
			RabbitmqPassword:            rabbitmqSecretPassword,
			RabbitmqVhost:               rabbitmqSecretVhost,
			LogLevel:                    logLevel,
			FabricMgmtIP:                fabricMgmtIP,
			CAFilePath:                  SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["devicemanager."+podIP] = configDevicemanagerConfigBuffer.String()

		var fabricAnsibleConfigBuffer bytes.Buffer
		err = configtemplates.FabricAnsibleConf.Execute(&fabricAnsibleConfigBuffer, struct {
			PodIP               string
			CollectorServerList string
			LogLevel            string
			CAFilePath          string
		}{
			PodIP:               podIP,
			CollectorServerList: collectorEndpointListSpaceSeparated,
			LogLevel:            logLevel,
			CAFilePath:          SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-fabric-ansible.conf."+podIP] = fabricAnsibleConfigBuffer.String()

		var configKeystoneAuthConfBuffer bytes.Buffer
		err = configtemplates.ConfigKeystoneAuthConf.Execute(&configKeystoneAuthConfBuffer, struct {
			KeystoneAuthParameters KeystoneAuthParameters
			CAFilePath             string
			PodIP                  string
			AuthMode               AuthenticationMode
		}{
			KeystoneAuthParameters: configAuth,
			CAFilePath:             SignerCAFilepath,
			PodIP:                  podIP,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-keystone-auth.conf."+podIP] = configKeystoneAuthConfBuffer.String()

		data["dnsmasq."+podIP] = configtemplates.ConfigDNSMasqConfig

		// UseExternalTFTP
		var configDNSMasqBuffer bytes.Buffer
		err = configtemplates.ConfigDNSMasqBaseConfig.Execute(&configDNSMasqBuffer, struct {
			UseExternalTFTP bool
		}{
			UseExternalTFTP: *configConfig.UseExternalTFTP,
		})
		if err != nil {
			panic(err)
		}
		data["dnsmasq_base."+podIP] = configDNSMasqBuffer.String()

		var configSchematransformerConfigBuffer bytes.Buffer
		err = configtemplates.ConfigSchematransformerConfig.Execute(&configSchematransformerConfigBuffer, struct {
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			SchemaIntrospectPort     string
			ApiServerList            string
			AnalyticsServerList      string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			LogLevel                 string
			CAFilePath               string
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			SchemaIntrospectPort:     strconv.Itoa(*configConfig.SchemaIntrospectPort),
			ApiServerList:            apiServerList,
			AnalyticsServerList:      analyticsServerList,
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			LogLevel:                 logLevel,
			CAFilePath:               SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["schematransformer."+podIP] = configSchematransformerConfigBuffer.String()

		var configServicemonitorConfigBuffer bytes.Buffer
		err = configtemplates.ConfigServicemonitorConfig.Execute(&configServicemonitorConfigBuffer, struct {
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			SvcMonitorIntrospectPort string
			ApiServerList            string
			AnalyticsServerList      string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			AAAMode                  AAAMode
			LogLevel                 string
			CAFilePath               string
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			SvcMonitorIntrospectPort: strconv.Itoa(*configConfig.SvcMonitorIntrospectPort),
			ApiServerList:            apiServerList,
			AnalyticsServerList:      analyticsEndpointListSpaceSeparated,
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			AAAMode:                  configConfig.AAAMode,
			LogLevel:                 logLevel,
			CAFilePath:               SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["servicemonitor."+podIP] = configServicemonitorConfigBuffer.String()

		var configNodemanagerconfigConfigBuffer bytes.Buffer
		err = configtemplates.NodemanagerConfig.Execute(&configNodemanagerconfigConfigBuffer, struct {
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
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               SignerCAFilepath,
			LogLevel:                 logLevel,
		})
		if err != nil {
			panic(err)
		}
		data["config-nodemgr.conf."+podIP] = configNodemanagerconfigConfigBuffer.String()
		// empty env as no db tracking
		data["config-nodemgr.env."+podIP] = ""
	}

	// update with provisioner configs
	data["config-provisioner.env"] = ProvisionerEnvData(apiServerList, c.Spec.CommonConfiguration.AuthParameters)

	return
}

// CreateConfigMap makes default empty ConfigMap
func (c *Config) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"config",
		c)
}

// CreateSecret creates a secret.
func (c *Config) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"config",
		c)
}

// PrepareSTS prepares the intented statefulset for the config object
func (c *Config) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "config", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the config statefulset
func (c *Config) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Config) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// SetInstanceActive sets the Config instance to active
func (c *Config) SetInstanceActive(client client.Client, activeStatus *bool, degradedStstus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace}, sts); err != nil {
		return err
	}
	*activeStatus = sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	*degradedStstus = sts.Status.ReadyReplicas < *sts.Spec.Replicas
	if err := client.Status().Update(context.TODO(), c); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Config) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("config", request, reconcileClient, "")
}

// ManageNodeStatus updates nodes in status
func (c *Config) ManageNodeStatus(podNameIPMap map[string]string,
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

// IsActive returns true if instance is active
func (c *Config) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// ConfigurationParameters create config struct
func (c *Config) ConfigurationParameters() ConfigConfiguration {
	configConfiguration := ConfigConfiguration{}

	var apiAdminPort int
	if c.Spec.ServiceConfiguration.APIAdminPort != nil {
		apiAdminPort = *c.Spec.ServiceConfiguration.APIAdminPort
	} else {
		apiAdminPort = ConfigAPIAdminPort
	}
	configConfiguration.APIAdminPort = &apiAdminPort

	var apiPort int
	if c.Spec.ServiceConfiguration.APIPort != nil {
		apiPort = *c.Spec.ServiceConfiguration.APIPort
	} else {
		apiPort = ConfigApiPort
	}
	configConfiguration.APIPort = &apiPort

	var apiIntrospectPort int
	if c.Spec.ServiceConfiguration.ApiIntrospectPort != nil {
		apiIntrospectPort = *c.Spec.ServiceConfiguration.ApiIntrospectPort
	} else {
		apiIntrospectPort = ConfigApiIntrospectPort
	}
	configConfiguration.ApiIntrospectPort = &apiIntrospectPort

	var apiWorkerCount int
	if c.Spec.ServiceConfiguration.APIWorkerCount != nil {
		apiWorkerCount = *c.Spec.ServiceConfiguration.APIWorkerCount
	} else {
		apiWorkerCount = ConfigAPIWorkerCount
	}
	configConfiguration.APIWorkerCount = &apiWorkerCount

	var schemaIntrospectPort int
	if c.Spec.ServiceConfiguration.SchemaIntrospectPort != nil {
		schemaIntrospectPort = *c.Spec.ServiceConfiguration.SchemaIntrospectPort
	} else {
		schemaIntrospectPort = ConfigSchemaIntrospectPort
	}
	configConfiguration.SchemaIntrospectPort = &schemaIntrospectPort

	var deviceManagerIntrospectPort int
	if c.Spec.ServiceConfiguration.DeviceManagerIntrospectPort != nil {
		deviceManagerIntrospectPort = *c.Spec.ServiceConfiguration.DeviceManagerIntrospectPort
	} else {
		deviceManagerIntrospectPort = ConfigDeviceManagerIntrospectPort
	}
	configConfiguration.DeviceManagerIntrospectPort = &deviceManagerIntrospectPort

	var svcMonitorIntrospectPort int
	if c.Spec.ServiceConfiguration.SvcMonitorIntrospectPort != nil {
		svcMonitorIntrospectPort = *c.Spec.ServiceConfiguration.SvcMonitorIntrospectPort
	} else {
		svcMonitorIntrospectPort = ConfigSvcMonitorIntrospectPort
	}
	configConfiguration.SvcMonitorIntrospectPort = &svcMonitorIntrospectPort

	configConfiguration.AAAMode = c.Spec.ServiceConfiguration.AAAMode
	if configConfiguration.AAAMode == "" {
		configConfiguration.AAAMode = AAAModeNoAuth
		ap := c.Spec.CommonConfiguration.AuthParameters
		if ap.AuthMode == AuthenticationModeKeystone {
			configConfiguration.AAAMode = AAAModeRBAC
		}
	}

	if c.Spec.ServiceConfiguration.LinklocalServiceConfig != nil {
		configConfiguration.LinklocalServiceConfig = c.Spec.ServiceConfiguration.LinklocalServiceConfig
		if configConfiguration.LinklocalServiceConfig.Name == nil {
			name := LinklocalServiceName
			configConfiguration.LinklocalServiceConfig.Name = &name
		}
		if configConfiguration.LinklocalServiceConfig.Port == nil {
			port := LinklocalServicePort
			configConfiguration.LinklocalServiceConfig.Port = &port
		}
		if configConfiguration.LinklocalServiceConfig.IP == nil {
			ip := LinklocalServiceIp
			configConfiguration.LinklocalServiceConfig.IP = &ip
		}
		if configConfiguration.LinklocalServiceConfig.IPFabricServicePort == nil {
			port := IpfabricServicePort
			configConfiguration.LinklocalServiceConfig.IPFabricServicePort = &port
		}
	}

	useExternalTFTP := false
	if c.Spec.ServiceConfiguration.UseExternalTFTP != nil {
		useExternalTFTP = *c.Spec.ServiceConfiguration.UseExternalTFTP
	}
	configConfiguration.UseExternalTFTP = &useExternalTFTP

	return configConfiguration
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Config) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
