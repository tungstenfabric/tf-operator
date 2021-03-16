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

// Analytics is the Schema for the analytics API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=analytics,scope=Namespaced
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready_Replicas",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type Analytics struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyticsSpec   `json:"spec,omitempty"`
	Status AnalyticsStatus `json:"status,omitempty"`
}

// AnalyticsSpec is the Spec for the Analytics API.
// +k8s:openapi-gen=true
type AnalyticsSpec struct {
	CommonConfiguration  PodConfiguration    `json:"commonConfiguration,omitempty"`
	ServiceConfiguration AnalyticsConfiguration `json:"serviceConfiguration"`
}

// AnalyticsConfiguration is the Spec for the Analytics API.
// +k8s:openapi-gen=true
type AnalyticsConfiguration struct {
	Containers                  []*Container       `json:"containers,omitempty"`
	AnalyticsPort               *int               `json:"analyticsPort,omitempty"`
	CollectorPort               *int               `json:"collectorPort,omitempty"`
	AnalyticsApiIntrospectPort  *int               `json:"analyticsIntrospectPort,omitempty"`
	CollectorIntrospectPort     *int               `json:"collectorIntrospectPort,omitempty"`
	ConfigInstance              string             `json:"configInstance,omitempty"`
	CassandraInstance           string             `json:"cassandraInstance,omitempty"`
	ZookeeperInstance           string             `json:"zookeeperInstance,omitempty"`
	RabbitmqInstance            string             `json:"rabbitmqInstance,omitempty"`
	RabbitmqUser                string             `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword            string             `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost               string             `json:"rabbitmqVhost,omitempty"`
	LogLevel                    string             `json:"logLevel,omitempty"`
	AAAMode                     AAAMode            `json:"aaaMode,omitempty"`
	Storage                     Storage            `json:"storage,omitempty"`
	// Time (in hours) that the analytics object and log data stays in the Cassandra database. Defaults to 48 hours.
	AnalyticsDataTTL            *int               `json:"analyticsDataTTL,omitempty"`
	// Time (in hours) the analytics config data entering the collector stays in the Cassandra database. Defaults to 2160 hours.
	AnalyticsConfigAuditTTL     *int               `json:"analyticsConfigAuditTTL,omitempty"`
	// Time to live (TTL) for statistics data in hours. Defaults to 4 hours.
	AnalyticsStatisticsTTL      *int               `json:"analyticsStatisticsTTL,omitempty"`
	// Time to live (TTL) for flow data in hours. Defaults to 2 hours.
	AnalyticsFlowTTL            *int               `json:"analyticsFlowTTL,omitempty"`
}

// AnalyticsStatus status of Analytics
// +k8s:openapi-gen=true
type AnalyticsStatus struct {
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	Endpoint      string            `json:"endpoint,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}

// AnalyticsList contains a list of Analytics.
// +k8s:openapi-gen=true
type AnalyticsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Analytics `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Analytics{}, &AnalyticsList{})
}

// InstanceConfiguration configures and updates configmaps
func (c *Analytics) InstanceConfiguration(configMapName string,
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

	configNodesInformation, err := NewConfigClusterConfiguration(
		c.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, client)
	if err != nil {
		return err
	}

	analyticsAuth := c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters

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

	analyticsConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = analyticsConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = analyticsConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = analyticsConfig.RabbitmqVhost
	}
	var collectorServerList, apiServerList, analyticsServerSpaceSeparatedList string
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	collectorServerList = strings.Join(podIPList, ":"+strconv.Itoa(*analyticsConfig.CollectorPort)+" ")
	collectorServerList = collectorServerList + ":" + strconv.Itoa(*analyticsConfig.CollectorPort)
	analyticsServerSpaceSeparatedList = strings.Join(podIPList, ":"+strconv.Itoa(*analyticsConfig.AnalyticsPort)+" ")
	analyticsServerSpaceSeparatedList = analyticsServerSpaceSeparatedList + ":" + strconv.Itoa(*analyticsConfig.AnalyticsPort)
	apiServerEndpointList := configtemplates.EndpointList(configNodesInformation.APIServerIPList, configNodesInformation.APIServerPort)
	apiServerEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(apiServerEndpointList, " ")
	apiServerIPListCommaSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, ",")
	cassandraEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	cassandraEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraEndpointList, " ")
	cassandraCQLEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.CQLPort)
	cassandraCQLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraCQLEndpointList, " ")
	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")
	rabbitmqSSLEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, ",")
	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")
	zookeeperEndpointListSpaceSpearated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, " ")

	redisServerSpaceSeparatedList := strings.Join(podIPList, ":6379 ") + ":6379"

	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var analyticsapiBuffer bytes.Buffer
		err = configtemplates.AnalyticsapiConfig.Execute(&analyticsapiBuffer, struct {
			PodIP                      string
			ListenAddress              string
			InstrospectListenAddress   string
			AnalyticsApiIntrospectPort string
			ApiServerList              string
			AnalyticsServerList        string
			CassandraServerList        string
			ZookeeperServerList        string
			RabbitmqServerList         string
			CollectorServerList        string
			RedisServerList            string
			RabbitmqUser               string
			RabbitmqPassword           string
			RabbitmqVhost              string
			AuthMode                   string
			AAAMode                    AAAMode
			CAFilePath                 string
			LogLevel                   string
		}{
			PodIP:                      podIP,
			ListenAddress:              podIP,
			InstrospectListenAddress:   instrospectListenAddress,
			AnalyticsApiIntrospectPort: strconv.Itoa(*analyticsConfig.AnalyticsApiIntrospectPort),
			ApiServerList:              apiServerEndpointListSpaceSeparated,
			AnalyticsServerList:        analyticsServerSpaceSeparatedList,
			CassandraServerList:        cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:        zookeeperEndpointListSpaceSpearated,
			RabbitmqServerList:         rabbitmqSSLEndpointListCommaSeparated,
			CollectorServerList:        collectorServerList,
			RedisServerList:            redisServerSpaceSeparatedList,
			RabbitmqUser:               rabbitmqSecretUser,
			RabbitmqPassword:           rabbitmqSecretPassword,
			RabbitmqVhost:              rabbitmqSecretVhost,
			AAAMode:                    analyticsConfig.AAAMode,
			CAFilePath:                 certificates.SignerCAFilepath,
			LogLevel:                   analyticsConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analyticsapi."+podIP] = analyticsapiBuffer.String()

		var collectorBuffer bytes.Buffer
		err = configtemplates.CollectorConfig.Execute(&collectorBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CollectorIntrospectPort  string
			ApiServerList            string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			LogLevel                 string
			CAFilePath               string
			AnalyticsDataTTL         string
			AnalyticsConfigAuditTTL  string
			AnalyticsStatisticsTTL   string
			AnalyticsFlowTTL         string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorIntrospectPort:  strconv.Itoa(*analyticsConfig.CollectorIntrospectPort),
			ApiServerList:            apiServerEndpointListSpaceSeparated,
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			LogLevel:                 analyticsConfig.LogLevel,
			CAFilePath:               certificates.SignerCAFilepath,
			AnalyticsDataTTL:         strconv.Itoa(*analyticsConfig.AnalyticsDataTTL),
			AnalyticsConfigAuditTTL:  strconv.Itoa(*analyticsConfig.AnalyticsConfigAuditTTL),
			AnalyticsStatisticsTTL:   strconv.Itoa(*analyticsConfig.AnalyticsStatisticsTTL),
			AnalyticsFlowTTL:         strconv.Itoa(*analyticsConfig.AnalyticsFlowTTL),
		})
		if err != nil {
			panic(err)
		}
		data["collector."+podIP] = collectorBuffer.String()

		var nodemanagerBuffer bytes.Buffer
		err = configtemplates.AnalyticsNodemanagerConfig.Execute(&nodemanagerBuffer, struct {
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
			CollectorServerList:      collectorServerList,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 analyticsConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["analytics-nodemgr.conf."+podIP] = nodemanagerBuffer.String()
		// empty env as no db tracking
		data["analytics-nodemgr.env."+podIP] = ""

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

		var analyticsKeystoneAuthConfBuffer bytes.Buffer
		err = configtemplates.AnalyticsKeystoneAuthConf.Execute(&analyticsKeystoneAuthConfBuffer, struct {
			AdminUsername             string
			AdminPassword             *string
			KeystoneAddress           string
			KeystonePort              *int
			KeystoneAuthProtocol      string
			KeystoneUserDomainName    string
			KeystoneProjectDomainName string
			KeystoneRegion            string
			CAFilePath                string
		}{
			AdminUsername:             analyticsAuth.AdminUsername,
			AdminPassword:             analyticsAuth.AdminPassword,
			KeystoneAddress:           analyticsAuth.Address,
			KeystonePort:              analyticsAuth.Port,
			KeystoneAuthProtocol:      analyticsAuth.AuthProtocol,
			KeystoneUserDomainName:    analyticsAuth.UserDomainName,
			KeystoneProjectDomainName: analyticsAuth.ProjectDomainName,
			KeystoneRegion:            analyticsAuth.Region,
			CAFilePath:                certificates.SignerCAFilepath,
		})
		if err != nil {
			panic(err)
		}
		data["contrail-keystone-auth.conf."+podIP] = analyticsKeystoneAuthConfBuffer.String()

		// TODO: commonize for all services
		var vncApiBuffer bytes.Buffer
		err = configtemplates.AnalyticsVncConfig.Execute(&vncApiBuffer, struct {
			ConfigNodes            string
			ConfigApiPort          string
			CAFilePath             string
			AuthMode			   AuthenticationMode
		}{
			ConfigNodes:            apiServerIPListCommaSeparated,
			ConfigApiPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             certificates.SignerCAFilepath,
			AuthMode:  			    c.Spec.CommonConfiguration.AuthParameters.AuthMode,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vncApiBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data

	// update with nodemanager runner
	nmr := GetNodemanagerRunner()

	configMapInstanceDynamicConfig.Data["analytics-nodemanager-runner.sh"] = nmr

	// update with provisioner configs
	UpdateProvisionerConfigMapData("analytics-provisioner", apiServerList, configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap makes default empty ConfigMap
func (c *Analytics) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"analytics",
		c)
}

// CreateSecret creates a secret.
func (c *Analytics) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"analytics",
		c)
}

// PrepareSTS prepares the intented statefulset for the analytics object
func (c *Analytics) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "analytics", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the analytics statefulset
func (c *Analytics) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Analytics) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

//CreateSTS creates the STS
func (c *Analytics) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

//UpdateSTS updates the STS
func (c *Analytics) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "deleteFirst")
}

// SetInstanceActive sets the Analytics instance to active
func (c *Analytics) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
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
func (c *Analytics) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("analytics", &c.Spec.CommonConfiguration, request, reconcileClient)
}

//PodsCertSubjects gets list of Analytics pods certificate subjets which can be passed to the certificate API
func (c *Analytics) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetPodsToReady set pods ready
func (c *Analytics) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// ManageNodeStatus updates nodes in status
func (c *Analytics) ManageNodeStatus(podNameIPMap map[string]string, client client.Client) error {
	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}

// IsActive returns true if instance is active
func (c *Analytics) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// ConfigurationParameters create analytics struct
func (c *Analytics) ConfigurationParameters() AnalyticsConfiguration {
	analyticsConfiguration := AnalyticsConfiguration{}
	var analyticsPort int
	var collectorPort int
	var rabbitmqUser string
	var rabbitmqPassword string
	var rabbitmqVhost string
	var logLevel string

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}
	analyticsConfiguration.LogLevel = logLevel

	if c.Spec.ServiceConfiguration.AnalyticsPort != nil {
		analyticsPort = *c.Spec.ServiceConfiguration.AnalyticsPort
	} else {
		analyticsPort = AnalyticsApiPort
	}
	analyticsConfiguration.AnalyticsPort = &analyticsPort

	if c.Spec.ServiceConfiguration.CollectorPort != nil {
		collectorPort = *c.Spec.ServiceConfiguration.CollectorPort
	} else {
		collectorPort = CollectorPort
	}
	analyticsConfiguration.CollectorPort = &collectorPort

	var analyticsApiIntrospectPort int
	if c.Spec.ServiceConfiguration.AnalyticsApiIntrospectPort != nil {
		analyticsApiIntrospectPort = *c.Spec.ServiceConfiguration.AnalyticsApiIntrospectPort
	} else {
		analyticsApiIntrospectPort = AnalyticsApiIntrospectPort
	}
	analyticsConfiguration.AnalyticsApiIntrospectPort = &analyticsApiIntrospectPort

	var collectorIntrospectPort int
	if c.Spec.ServiceConfiguration.CollectorIntrospectPort != nil {
		collectorIntrospectPort = *c.Spec.ServiceConfiguration.CollectorIntrospectPort
	} else {
		collectorIntrospectPort = CollectorIntrospectPort
	}
	analyticsConfiguration.CollectorIntrospectPort = &collectorIntrospectPort

	if c.Spec.ServiceConfiguration.RabbitmqUser != "" {
		rabbitmqUser = c.Spec.ServiceConfiguration.RabbitmqUser
	} else {
		rabbitmqUser = RabbitmqUser
	}
	analyticsConfiguration.RabbitmqUser = rabbitmqUser

	if c.Spec.ServiceConfiguration.RabbitmqPassword != "" {
		rabbitmqPassword = c.Spec.ServiceConfiguration.RabbitmqPassword
	} else {
		rabbitmqPassword = RabbitmqPassword
	}
	analyticsConfiguration.RabbitmqPassword = rabbitmqPassword

	if c.Spec.ServiceConfiguration.RabbitmqVhost != "" {
		rabbitmqVhost = c.Spec.ServiceConfiguration.RabbitmqVhost
	} else {
		rabbitmqVhost = RabbitmqVhost
	}
	analyticsConfiguration.RabbitmqVhost = rabbitmqVhost

	analyticsConfiguration.AAAMode = c.Spec.ServiceConfiguration.AAAMode
	if analyticsConfiguration.AAAMode == "" {
		analyticsConfiguration.AAAMode = AAAModeNoAuth
		ap := c.Spec.CommonConfiguration.AuthParameters
		if ap != nil && ap.AuthMode == AuthenticationModeKeystone {
			analyticsConfiguration.AAAMode = AAAModeRBAC
		}
	}

	var analyticsDataTTL int
	if c.Spec.ServiceConfiguration.AnalyticsDataTTL != nil {
		analyticsDataTTL = *c.Spec.ServiceConfiguration.AnalyticsDataTTL
	} else {
		analyticsDataTTL = AnalyticsDataTTL
	}
	analyticsConfiguration.AnalyticsDataTTL = &analyticsDataTTL

	var analyticsConfigAuditTTL int
	if c.Spec.ServiceConfiguration.AnalyticsConfigAuditTTL != nil {
		analyticsConfigAuditTTL = *c.Spec.ServiceConfiguration.AnalyticsConfigAuditTTL
	} else {
		analyticsConfigAuditTTL = AnalyticsConfigAuditTTL
	}
	analyticsConfiguration.AnalyticsConfigAuditTTL = &analyticsConfigAuditTTL

	var analyticsStatisticsTTL int
	if c.Spec.ServiceConfiguration.AnalyticsStatisticsTTL != nil {
		analyticsStatisticsTTL = *c.Spec.ServiceConfiguration.AnalyticsStatisticsTTL
	} else {
		analyticsStatisticsTTL = AnalyticsStatisticsTTL
	}
	analyticsConfiguration.AnalyticsStatisticsTTL = &analyticsStatisticsTTL

	var analyticsFlowTTL int
	if c.Spec.ServiceConfiguration.AnalyticsFlowTTL != nil {
		analyticsFlowTTL = *c.Spec.ServiceConfiguration.AnalyticsFlowTTL
	} else {
		analyticsFlowTTL = AnalyticsFlowTTL
	}
	analyticsConfiguration.AnalyticsFlowTTL = &analyticsFlowTTL

	return analyticsConfiguration

}

func (c *Analytics) SetEndpointInStatus(client client.Client, clusterIP string) error {
	c.Status.Endpoint = clusterIP
	err := client.Status().Update(context.TODO(), c)
	return err
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Analytics) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
