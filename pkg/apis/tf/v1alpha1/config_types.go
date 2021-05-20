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
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
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
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready_Replicas",type=integer,JSONPath=`.status.readyReplicas`
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
	APIPort                     *int                    `json:"apiPort,omitempty"`
	ApiIntrospectPort           *int                    `json:"apiIntrospectPort,omitempty"`
	SchemaIntrospectPort        *int                    `json:"schemaIntrospectPort,omitempty"`
	DeviceManagerIntrospectPort *int                    `json:"deviceManagerIntrospectPort,omitempty"`
	SvcMonitorIntrospectPort    *int                    `json:"svcMonitorIntrospectPort,omitempty"`
	AnalyticsInstance           string                  `json:"analyticsInstance,omitempty"`
	CassandraInstance           string                  `json:"cassandraInstance,omitempty"`
	ZookeeperInstance           string                  `json:"zookeeperInstance,omitempty"`
	RabbitmqInstance            string                  `json:"rabbitmqInstance,omitempty"`
	RabbitmqUser                string                  `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword            string                  `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost               string                  `json:"rabbitmqVhost,omitempty"`
	LogLevel                    string                  `json:"logLevel,omitempty"`
	AAAMode                     AAAMode                 `json:"aaaMode,omitempty"`
	Storage                     Storage                 `json:"storage,omitempty"`
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
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
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
func (c *Config) InstanceConfiguration(configMapName string,
	request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {

	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	configAuth := c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		c.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(
		c.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		c.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(
		c.Spec.ServiceConfiguration.AnalyticsInstance, request.Namespace, client)
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

	configConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = configConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = configConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = configConfig.RabbitmqVhost
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

	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)
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
		}{
			PodIP:                    podIP,
			ListenAddress:            podIP,
			ListenPort:               strconv.Itoa(*configConfig.APIPort),
			InstrospectListenAddress: instrospectListenAddress,
			ApiIntrospectPort:        strconv.Itoa(*configConfig.ApiIntrospectPort),
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			AuthMode:                 c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			AAAMode:                  configConfig.AAAMode,
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["api."+podIP] = configApiConfigBuffer.String()

		var vncApiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncApiConfigBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters *KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          apiServerList,
			APIServerPort:          strconv.Itoa(*configConfig.APIPort),
			CAFilePath:             certificates.SignerCAFilepath,
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
			LogLevel:                    configConfig.LogLevel,
			FabricMgmtIP:                fabricMgmtIP,
			CAFilePath:                  certificates.SignerCAFilepath,
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
			LogLevel:            configConfig.LogLevel,
			CAFilePath:          certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-fabric-ansible.conf."+podIP] = fabricAnsibleConfigBuffer.String()

		var configKeystoneAuthConfBuffer bytes.Buffer
		err = configtemplates.ConfigKeystoneAuthConf.Execute(&configKeystoneAuthConfBuffer, struct {
			KeystoneAuthParameters *KeystoneAuthParameters
			CAFilePath             string
			PodIP                  string
			AuthMode               AuthenticationMode
		}{
			KeystoneAuthParameters: configAuth,
			CAFilePath:             certificates.SignerCAFilepath,
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
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
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
			LogLevel:                 configConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["servicemonitor."+podIP] = configServicemonitorConfigBuffer.String()

		var configNodemanagerconfigConfigBuffer bytes.Buffer
		err = configtemplates.ConfigNodemanagerConfigConfig.Execute(&configNodemanagerconfigConfigBuffer, struct {
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
			LogLevel:                 configConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["config-nodemgr.conf."+podIP] = configNodemanagerconfigConfigBuffer.String()
		// empty env as no db tracking
		data["config-nodemgr.env."+podIP] = ""
	}

	configMapInstanceDynamicConfig.Data = data

	// update with nodemanager runner
	nmr := GetNodemanagerRunner()
	configMapInstanceDynamicConfig.Data["config-nodemanager-runner.sh"] = nmr

	// update with provisioner configs
	UpdateProvisionerConfigMapData("config-provisioner", apiServerList,
		c.Spec.CommonConfiguration.AuthParameters, configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
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

//CreateSTS creates the STS
func (c *Config) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

//UpdateSTS updates the STS
func (c *Config) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "deleteFirst")
}

// SetInstanceActive sets the Config instance to active
func (c *Config) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
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
func (c *Config) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("config", request, reconcileClient, "")
}

//PodsCertSubjects gets list of Config pods certificate subjets which can be passed to the certificate API
func (c *Config) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetPodsToReady set pods ready
func (c *Config) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
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
	var apiPort int
	var rabbitmqUser string
	var rabbitmqPassword string
	var rabbitmqVhost string
	var logLevel string

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}
	configConfiguration.LogLevel = logLevel
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

	if c.Spec.ServiceConfiguration.RabbitmqUser != "" {
		rabbitmqUser = c.Spec.ServiceConfiguration.RabbitmqUser
	} else {
		rabbitmqUser = RabbitmqUser
	}
	configConfiguration.RabbitmqUser = rabbitmqUser

	if c.Spec.ServiceConfiguration.RabbitmqPassword != "" {
		rabbitmqPassword = c.Spec.ServiceConfiguration.RabbitmqPassword
	} else {
		rabbitmqPassword = RabbitmqPassword
	}
	configConfiguration.RabbitmqPassword = rabbitmqPassword

	if c.Spec.ServiceConfiguration.RabbitmqVhost != "" {
		rabbitmqVhost = c.Spec.ServiceConfiguration.RabbitmqVhost
	} else {
		rabbitmqVhost = RabbitmqVhost
	}
	configConfiguration.RabbitmqVhost = rabbitmqVhost

	configConfiguration.AAAMode = c.Spec.ServiceConfiguration.AAAMode
	if configConfiguration.AAAMode == "" {
		configConfiguration.AAAMode = AAAModeNoAuth
		ap := c.Spec.CommonConfiguration.AuthParameters
		if ap != nil && ap.AuthMode == AuthenticationModeKeystone {
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

// SetEndpointInStatus updates Endpoint in status
func (c *Config) SetEndpointInStatus(client client.Client, clusterIP string) error {
	c.Status.Endpoint = clusterIP
	err := client.Status().Update(context.TODO(), c)
	return err
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Config) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
