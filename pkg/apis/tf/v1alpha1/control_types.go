package v1alpha1

import (
	"bytes"
	"context"
	"reflect"
	"sort"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Control is the Schema for the controls API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Control struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlSpec   `json:"spec,omitempty"`
	Status ControlStatus `json:"status,omitempty"`
}

// ControlSpec is the Spec for the controls API.
// +k8s:openapi-gen=true
type ControlSpec struct {
	CommonConfiguration  PodConfiguration     `json:"commonConfiguration,omitempty"`
	ServiceConfiguration ControlConfiguration `json:"serviceConfiguration"`
}

// ControlConfiguration is the Spec for the controls API.
// +k8s:openapi-gen=true
type ControlConfiguration struct {
	Containers        []*Container `json:"containers,omitempty"`
	CassandraInstance string       `json:"cassandraInstance,omitempty"`
	RabbitmqInstance  string       `json:"rabbitmqInstance,omitempty"`
	ConfigInstance    string       `json:"configInstance,omitempty"`
	AnalyticsInstance string       `json:"analyticsInstance,omitempty"`
	BGPPort           *int         `json:"bgpPort,omitempty"`
	ASNNumber         *int         `json:"asnNumber,omitempty"`
	XMPPPort          *int         `json:"xmppPort,omitempty"`
	DNSPort           *int         `json:"dnsPort,omitempty"`
	DNSIntrospectPort *int         `json:"dnsIntrospectPort,omitempty"`
	RabbitmqUser      string       `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword  string       `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost     string       `json:"rabbitmqVhost,omitempty"`
	// DataSubnet allow to set alternative network in which control, nodemanager
	// and dns services will listen. Local pod address from this subnet will be
	// discovered and used both in configuration for hostip directive and provision
	// script.
	// +kubebuilder:validation:Pattern=`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\/(3[0-2]|2[0-9]|1[0-9]|[0-9]))$`
	DataSubnet string `json:"dataSubnet,omitempty"`
	LogLevel   string `json:"logLevel,omitempty"`
}

// ControlStatus defines the observed state of Control.
// +k8s:openapi-gen=true
type ControlStatus struct {
	Active        *bool                           `json:"active,omitempty"`
	Nodes         map[string]string               `json:"nodes,omitempty"`
	Ports         ControlStatusPorts              `json:"ports,omitempty"`
	ServiceStatus map[string]ControlServiceStatus `json:"serviceStatus,omitempty"`
	ConfigChanged *bool                           `json:"configChanged,omitempty"`
}

// ControlServiceStatus status of control
// +k8s:openapi-gen=true
type ControlServiceStatus struct {
	Connections              []Connection `json:"connections,omitempty"`
	NumberOfXMPPPeers        string       `json:"numberOfXMPPPeers,omitempty"`
	NumberOfRoutingInstances string       `json:"numberOfRoutingInstances,omitempty"`
	StaticRoutes             StaticRoutes `json:"staticRoutes,omitempty"`
	BGPPeer                  BGPPeer      `json:"bgpPeer,omitempty"`
	State                    string       `json:"state,omitempty"`
}

// StaticRoutes statuic routes
// +k8s:openapi-gen=true
type StaticRoutes struct {
	Down   string `json:"down,omitempty"`
	Number string `json:"number,omitempty"`
}

// BGPPeer bgp peer status
// +k8s:openapi-gen=true
type BGPPeer struct {
	Up     string `json:"up,omitempty"`
	Number string `json:"number,omitempty"`
}

// Connection connection status
// +k8s:openapi-gen=true
type Connection struct {
	Type   string   `json:"type,omitempty"`
	Name   string   `json:"name,omitempty"`
	Status string   `json:"status,omitempty"`
	Nodes  []string `json:"nodes,omitempty"`
}

// ControlStatusPorts status of connection ports
// +k8s:openapi-gen=true
type ControlStatusPorts struct {
	BGPPort           string `json:"bgpPort,omitempty"`
	ASNNumber         string `json:"asnNumber,omitempty"`
	XMPPPort          string `json:"xmppPort,omitempty"`
	DNSPort           string `json:"dnsPort,omitempty"`
	DNSIntrospectPort string `json:"dnsIntrospectPort,omitempty"`
}

// ControlList contains a list of Control.
// +k8s:openapi-gen=true
type ControlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Control `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Control{}, &ControlList{})
}

func getPodDataIP(pod *corev1.Pod) (string, error) {
	if cidr, isSet := pod.Annotations["dataSubnet"]; isSet {
		ip, err := GetDataAddresses(pod, "control", cidr)
		if err != nil {
			return "", err
		}
		return ip, nil
	}
	return pod.Status.PodIP, nil
}

// InstanceConfiguration prepares control configmap
func (c *Control) InstanceConfiguration(request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {
	instanceConfigMapName := request.Name + "-" + "control" + "-configmap"
	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: instanceConfigMapName, Namespace: request.Namespace},
		configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(c.Spec.ServiceConfiguration.CassandraInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(c.Spec.ServiceConfiguration.RabbitmqInstance,
		request.Namespace, client)
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

	configNodesInformation, err := NewConfigClusterConfiguration(c.Spec.ServiceConfiguration.ConfigInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(c.Spec.ServiceConfiguration.AnalyticsInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}

	controlConfig := c.ConfigurationParameters()
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = controlConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = controlConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = controlConfig.RabbitmqVhost
	}

	rabbitMqSSLEndpointList := configtemplates.EndpointList(rabbitmqNodesInformation.ServerIPList, rabbitmqNodesInformation.Port)
	rabbitmqSSLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(rabbitMqSSLEndpointList, " ")
	cassandraCQLEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.CQLPort)
	cassandraCQLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraCQLEndpointList, " ")

	configApiIPListSpaceSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, " ")
	configApiIPListCommaSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, ",")
	configApiIPListCommaSeparatedQuoted := configtemplates.JoinListWithSeparatorAndSingleQuotes(configNodesInformation.APIServerIPList, ",")
	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")

	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	var data = make(map[string]string)
	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		podListenAddress, err := getPodDataIP(&pod)
		if err != nil {
			return err
		}
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)
		var controlControlConfigBuffer bytes.Buffer
		err = configtemplates.ControlControlConfig.Execute(&controlControlConfigBuffer, struct {
			PodIP                    string
			Hostname                 string
			ListenAddress            string
			InstrospectListenAddress string
			BGPPort                  string
			ASNNumber                string
			APIServerList            string
			APIServerPort            string
			CassandraServerList      string
			RabbitmqServerList       string
			RabbitmqServerPort       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			CAFilePath               string
			LogLevel                 string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podListenAddress,
			InstrospectListenAddress: instrospectListenAddress,
			BGPPort:                  strconv.Itoa(*controlConfig.BGPPort),
			ASNNumber:                strconv.Itoa(*controlConfig.ASNNumber),
			APIServerList:            configApiIPListSpaceSeparated,
			APIServerPort:            strconv.Itoa(configNodesInformation.APIServerPort),
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqServerPort:       strconv.Itoa(rabbitmqNodesInformation.Port),
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 controlConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["control."+podIP] = controlControlConfigBuffer.String()

		var controlNamedConfigBuffer bytes.Buffer
		err = configtemplates.ControlNamedConfig.Execute(&controlNamedConfigBuffer, struct{}{})
		if err != nil {
			panic(err)
		}
		data["named."+podIP] = controlNamedConfigBuffer.String()

		var controlDNSConfigBuffer bytes.Buffer
		err = configtemplates.ControlDNSConfig.Execute(&controlDNSConfigBuffer, struct {
			PodIP                    string
			Hostname                 string
			ListenAddress            string
			InstrospectListenAddress string
			APIServerList            string
			APIServerPort            string
			CassandraServerList      string
			RabbitmqServerList       string
			RabbitmqServerPort       string
			CollectorServerList      string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			CAFilePath               string
			LogLevel                 string
		}{
			PodIP:                    podIP,
			Hostname:                 hostname,
			ListenAddress:            podListenAddress,
			InstrospectListenAddress: instrospectListenAddress,
			APIServerList:            configApiIPListSpaceSeparated,
			APIServerPort:            strconv.Itoa(configNodesInformation.APIServerPort),
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListSpaceSeparated,
			RabbitmqServerPort:       strconv.Itoa(rabbitmqNodesInformation.Port),
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 controlConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["dns."+podIP] = controlDNSConfigBuffer.String()

		var controlNodemanagerBuffer bytes.Buffer
		err = configtemplates.ControlNodemanagerConfig.Execute(&controlNodemanagerBuffer, struct {
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
			ListenAddress:            podListenAddress,
			InstrospectListenAddress: instrospectListenAddress,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			CassandraPort:            strconv.Itoa(cassandraNodesInformation.CQLPort),
			CassandraJmxPort:         strconv.Itoa(cassandraNodesInformation.JMXPort),
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 controlConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["control-nodemgr.conf."+podIP] = controlNodemanagerBuffer.String()
		// empty env as no db tracking
		data["control-nodemgr.env."+podIP] = ""

		var vncApiConfigBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncApiConfigBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters *KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          configApiIPListCommaSeparated,
			APIServerPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             certificates.SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
			PodIP:                  podIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vncApiConfigBuffer.String()

		var controlDeProvisionBuffer bytes.Buffer
		// TODO: use auth options from config instead of defaults
		err = configtemplates.ControlDeProvisionConfig.Execute(&controlDeProvisionBuffer, struct {
			AdminUsername string
			AdminPassword string
			AdminTenant   string
			APIServerList string
			APIServerPort string
			Hostname      string
		}{
			AdminUsername: KeystoneAuthAdminUser,
			AdminPassword: KeystoneAuthAdminPassword,
			AdminTenant:   KeystoneAuthAdminTenant,
			APIServerList: configApiIPListCommaSeparatedQuoted,
			APIServerPort: strconv.Itoa(configNodesInformation.APIServerPort),
			Hostname:      hostname,
		})
		if err != nil {
			panic(err)
		}
		data["deprovision.py."+podIP] = controlDeProvisionBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data

	// update with nodemanager runner
	configMapInstanceDynamicConfig.Data["control-nodemanager-runner.sh"] = GetNodemanagerRunner()

	// update with provisioner configs
	UpdateProvisionerConfigMapData("control-provisioner", configApiIPListCommaSeparated,
		c.Spec.CommonConfiguration.AuthParameters, configMapInstanceDynamicConfig)

	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap creates configmap
func (c *Control) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"control",
		c)
}

// IsActive returns true if instance is active.
func (c *Control) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// CreateSecret creates a secret.
func (c *Control) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"control",
		c)
}

// PrepareSTS prepares the intended deployment for the Control object.
func (c *Control) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "control", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the Control deployment.
func (c *Control) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Control) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// CreateSTS creates the STS.
func (c *Control) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Control) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "rolling")
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Control) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	datanetwork := c.Spec.ServiceConfiguration.DataSubnet
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, datanetwork)
}

func retrieveDataIPs(pod corev1.Pod) []string {
	var altIPs []string
	altIP, _ := getPodDataIP(&pod)
	altIPs = append(altIPs, altIP)
	return altIPs
}

//PodsCertSubjects gets list of Control pods certificate subjects which can be passed to the certificate API
func (c *Control) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	altIPs := PodAlternativeIPs{Retriever: retrieveDataIPs}
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetInstanceActive sets instance to active.
func (c *Control) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, sts, request, c)
}

func (c *Control) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) (updated bool, err error) {
	updated = false
	err = nil

	config := c.ConfigurationParameters()
	bgpPort := strconv.Itoa(*config.BGPPort)
	asnNumber := strconv.Itoa(*config.ASNNumber)
	xmppPort := strconv.Itoa(*config.XMPPPort)
	dnsPort := strconv.Itoa(*config.DNSPort)
	dnsIntrospectPort := strconv.Itoa(*config.DNSIntrospectPort)
	if bgpPort == c.Status.Ports.BGPPort &&
		asnNumber == c.Status.Ports.ASNNumber &&
		xmppPort == c.Status.Ports.XMPPPort &&
		dnsPort == c.Status.Ports.DNSPort &&
		dnsIntrospectPort == c.Status.Ports.DNSIntrospectPort &&
		reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		return
	}

	c.Status.Ports.BGPPort = bgpPort
	c.Status.Ports.ASNNumber = asnNumber
	c.Status.Ports.XMPPPort = xmppPort
	c.Status.Ports.DNSPort = dnsPort
	c.Status.Ports.DNSIntrospectPort = dnsIntrospectPort
	c.Status.Nodes = podNameIPMap
	if err = client.Status().Update(context.TODO(), c); err != nil {
		return
	}

	updated = true
	return
}

// ConfigurationParameters makes ControlConfiguration
func (c *Control) ConfigurationParameters() ControlConfiguration {
	controlConfiguration := ControlConfiguration{}
	var bgpPort int
	var asnNumber int
	var xmppPort int
	var dnsPort int
	var dnsIntrospectPort int
	var logLevel string

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}

	if c.Spec.ServiceConfiguration.BGPPort != nil {
		bgpPort = *c.Spec.ServiceConfiguration.BGPPort
	} else {
		bgpPort = BgpPort
	}

	if c.Spec.ServiceConfiguration.ASNNumber != nil {
		asnNumber = *c.Spec.ServiceConfiguration.ASNNumber
	} else {
		asnNumber = BgpAsn
	}

	if c.Spec.ServiceConfiguration.XMPPPort != nil {
		xmppPort = *c.Spec.ServiceConfiguration.XMPPPort
	} else {
		xmppPort = XmppServerPort
	}

	if c.Spec.ServiceConfiguration.DNSPort != nil {
		dnsPort = *c.Spec.ServiceConfiguration.DNSPort
	} else {
		dnsPort = DnsServerPort
	}

	if c.Spec.ServiceConfiguration.DNSIntrospectPort != nil {
		dnsIntrospectPort = *c.Spec.ServiceConfiguration.DNSIntrospectPort
	} else {
		dnsIntrospectPort = DnsIntrospectPort
	}

	controlConfiguration.BGPPort = &bgpPort
	controlConfiguration.ASNNumber = &asnNumber
	controlConfiguration.XMPPPort = &xmppPort
	controlConfiguration.DNSPort = &dnsPort
	controlConfiguration.DNSIntrospectPort = &dnsIntrospectPort
	controlConfiguration.LogLevel = logLevel

	return controlConfiguration
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Control) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
