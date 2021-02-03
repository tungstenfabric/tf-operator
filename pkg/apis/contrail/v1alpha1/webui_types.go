package v1alpha1

import (
	"bytes"
	"context"
	"sort"
	"strconv"

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Webui is the Schema for the webuis API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Ready_Replicas",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type Webui struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebuiSpec   `json:"spec,omitempty"`
	Status WebuiStatus `json:"status,omitempty"`
}

// WebuiSpec is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type WebuiSpec struct {
	CommonConfiguration  PodConfiguration   `json:"commonConfiguration,omitempty"`
	ServiceConfiguration WebuiConfiguration `json:"serviceConfiguration"`
}

// WebuiConfiguration is the Spec for the cassandras API.
// +k8s:openapi-gen=true
type WebuiConfiguration struct {
	Containers         []*Container `json:"containers,omitempty"`
	CassandraInstance  string       `json:"cassandraInstance,omitempty"`
	KeystoneSecretName string       `json:"keystoneSecretName,omitempty"`
}

type WebUIStatusPorts struct {
	WebUIHttpPort  int `json:"webUIHttpPort,omitempty"`
	WebUIHttpsPort int `json:"webUIHttpsPort,omitempty"`
}

type WebUIServiceStatus struct {
	ModuleName  string `json:"moduleName,omitempty"`
	ModuleState string `json:"state"`
}

// +k8s:openapi-gen=true
type WebuiStatus struct {
	Status        `json:",inline"`
	Nodes         map[string]string                `json:"nodes,omitempty"`
	Ports         WebUIStatusPorts                 `json:"ports,omitempty"`
	ServiceStatus map[string]WebUIServiceStatusMap `json:"serviceStatus,omitempty"`
	Endpoint      string                           `json:"endpoint,omitempty"`
}

type WebUIServiceStatusMap map[string]WebUIServiceStatus

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WebuiList contains a list of Webui.
type WebuiList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Webui `json:"items"`
}

type keystoneEndpoint struct {
	address           string
	port              int
	authProtocol      string
	region            string
	userDomainName    string
	projectDomainName string
}

func init() {
	SchemeBuilder.Register(&Webui{}, &WebuiList{})
}

func (c *Webui) InstanceConfiguration(request reconcile.Request,
	podList *corev1.PodList,
	client client.Client) error {
	instanceConfigMapName := request.Name + "-" + "webui" + "-configmap"
	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	err := client.Get(context.TODO(),
		types.NamespacedName{Name: instanceConfigMapName, Namespace: request.Namespace},
		configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}

	controlNodesInformation, err := NewControlClusterConfiguration("", "master",
		request.Namespace, client)
	if err != nil {
		return err
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(c.Spec.ServiceConfiguration.CassandraInstance,
		request.Namespace, client)
	if err != nil {
		return err
	}

	configNodesInformation, err := NewConfigClusterConfiguration(c.Labels["contrail_cluster"],
		request.Namespace, client)
	if err != nil {
		return err
	}

	authConfig, err := c.AuthParameters(client)
	if err != nil {
		return err
	}

	manager := "none"

	configApiIPListCommaSeparatedQuoted := configtemplates.JoinListWithSeparatorAndSingleQuotes(configNodesInformation.APIServerIPList, ",")
	analyticsIPListCommaSeparatedQuoted := configtemplates.JoinListWithSeparatorAndSingleQuotes(configNodesInformation.AnalyticsServerIPList, ",")
	controlXMPPIPListCommaSeparatedQuoted := configtemplates.JoinListWithSeparatorAndSingleQuotes(controlNodesInformation.ControlServerIPList, ",")
	cassandraIPListCommaSeparatedQuoted := configtemplates.JoinListWithSeparatorAndSingleQuotes(cassandraNodesInformation.ServerIPList, ",")
	sort.SliceStable(podList.Items, func(i, j int) bool { return podList.Items[i].Status.PodIP < podList.Items[j].Status.PodIP })
	var data = make(map[string]string)
	for idx := range podList.Items {
		var webuiWebConfigBuffer bytes.Buffer
		err := configtemplates.WebuiWebConfig.Execute(&webuiWebConfigBuffer, struct {
			HostIP                    string
			Hostname                  string
			APIServerList             string
			APIServerPort             string
			AnalyticsServerList       string
			AnalyticsServerPort       string
			ControlNodeList           string
			DnsNodePort               string
			CassandraServerList       string
			CassandraPort             string
			AdminUsername             string
			AdminPassword             string
			KeystoneProjectDomainName string
			KeystoneUserDomainName    string
			KeystoneAuthProtocol      string
			KeystoneAddress           string
			KeystonePort              int
			KeystoneRegion            string
			Manager                   string
			CAFilePath                string
		}{
			HostIP:                    podList.Items[idx].Status.PodIP,
			Hostname:                  podList.Items[idx].Name,
			APIServerList:             configApiIPListCommaSeparatedQuoted,
			APIServerPort:             strconv.Itoa(configNodesInformation.APIServerPort),
			AnalyticsServerList:       analyticsIPListCommaSeparatedQuoted,
			AnalyticsServerPort:       strconv.Itoa(configNodesInformation.AnalyticsServerPort),
			ControlNodeList:           controlXMPPIPListCommaSeparatedQuoted,
			DnsNodePort:               strconv.Itoa(controlNodesInformation.DNSIntrospectPort),
			CassandraServerList:       cassandraIPListCommaSeparatedQuoted,
			CassandraPort:             strconv.Itoa(cassandraNodesInformation.CQLPort),
			AdminUsername:             authConfig.AdminUsername,
			AdminPassword:             authConfig.AdminPassword,
			KeystoneProjectDomainName: authConfig.ProjectDomainName,
			KeystoneUserDomainName:    authConfig.UserDomainName,
			KeystoneAuthProtocol:      authConfig.AuthProtocol,
			KeystoneAddress:           authConfig.Address,
			KeystonePort:              authConfig.Port,
			KeystoneRegion:            authConfig.Region,
			Manager:                   manager,
			CAFilePath:                certificates.SignerCAFilepath,
		})
		if err != nil {
			log.Error(err, "configtemplates.WebuiWebConfig.Execute failed")
		}
		data["config.global.js."+podList.Items[idx].Status.PodIP] = webuiWebConfigBuffer.String()
		//fmt.Println("DATA ", data)
		var webuiAuthConfigBuffer bytes.Buffer
		err = configtemplates.WebuiAuthConfig.Execute(&webuiAuthConfigBuffer, struct {
			AdminUsername             string
			AdminPassword             string
			KeystoneProjectDomainName string
			KeystoneUserDomainName    string
		}{
			AdminUsername:             authConfig.AdminUsername,
			AdminPassword:             authConfig.AdminPassword,
			KeystoneProjectDomainName: authConfig.ProjectDomainName,
			KeystoneUserDomainName:    authConfig.UserDomainName,
		})
		if err != nil {
			log.Error(err, "configtemplates.WebuiWebConfig.Execute failed")
		}
		data["contrail-webui-userauth.js."+podList.Items[idx].Status.PodIP] = webuiAuthConfigBuffer.String()
	}
	configMapInstanceDynamicConfig.Data = data
	err = client.Update(context.TODO(), configMapInstanceDynamicConfig)
	if err != nil {
		return err
	}
	return nil
}

// CreateSecret creates a secret.
func (c *Webui) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"webui",
		c)
}

// CreateConfigMap create webui configmap
func (c *Webui) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"webui",
		c)
}

// AuthParameters makes default empty ConfigAuthParameters
func (c *Webui) AuthParameters(client client.Client) (*ConfigAuthParameters, error) {
	// TODO: to be implented
	secretName := ""
	return AuthParameters(c.Namespace, secretName, client)
}

// PrepareSTS prepares the intended deployment for the Webui object.
func (c *Webui) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "webui", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the Webui deployment.
func (c *Webui) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Webui) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// SetPodsToReady sets Webui PODs to ready.
func (c *Webui) SetPodsToReady(podIPList *corev1.PodList, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// CreateSTS creates the STS.
func (c *Webui) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) error {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Webui) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client, strategy string) error {
	return UpdateSTS(sts, instanceType, request, reconcileClient, strategy)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Webui) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) (*corev1.PodList, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, &c.Spec.CommonConfiguration, request, reconcileClient, true, false, false, false, false, false)
}

//PodsCertSubjects gets list of Config pods certificate subjets which can be passed to the certificate API
func (c *Webui) PodsCertSubjects(podList *corev1.PodList) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetInstanceActive sets the Webui instance to active.
func (c *Webui) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, sts, request, c)
}

func (c *Webui) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) error {
	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}
