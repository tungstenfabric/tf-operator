package v1alpha1

import (
	"bytes"
	"context"
	"fmt"
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

// Kubemanager is the Schema for the kubemanagers API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Kubemanager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubemanagerSpec   `json:"spec,omitempty"`
	Status KubemanagerStatus `json:"status,omitempty"`
}

// KubemanagerSpec is the Spec for the kubemanagers API.
// +k8s:openapi-gen=true
type KubemanagerSpec struct {
	CommonConfiguration  PodConfiguration                `json:"commonConfiguration,omitempty"`
	ServiceConfiguration KubemanagerServiceConfiguration `json:"serviceConfiguration"`
}

// KubemanagerStatus is the Status for the kubemanagers API.
// +k8s:openapi-gen=true
type KubemanagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Active        *bool             `json:"active,omitempty"`
	Nodes         map[string]string `json:"nodes,omitempty"`
	ConfigChanged *bool             `json:"configChanged,omitempty"`
}

// KubemanagerServiceConfiguration is the Spec for the kubemanagers API.
// +k8s:openapi-gen=true
type KubemanagerServiceConfiguration struct {
	KubemanagerConfiguration `json:",inline"`
	CassandraInstance        string `json:"cassandraInstance,omitempty"`
	ZookeeperInstance        string `json:"zookeeperInstance,omitempty"`
	ConfigInstance           string `json:"configInstance,omitempty"`
	AnalyticsInstance        string `json:"analyticsInstance,omitempty"`
}

// KubemanagerConfiguration is the configuration for the kubemanagers API.
// +k8s:openapi-gen=true
type KubemanagerConfiguration struct {
	Containers            []*Container `json:"containers,omitempty"`
	ServiceAccount        string       `json:"serviceAccount,omitempty"`
	ClusterRole           string       `json:"clusterRole,omitempty"`
	ClusterRoleBinding    string       `json:"clusterRoleBinding,omitempty"`
	CloudOrchestrator     string       `json:"cloudOrchestrator,omitempty"`
	SecretName            string       `json:"secretName,omitempty"`
	KubernetesAPIServer   string       `json:"kubernetesAPIServer,omitempty"`
	KubernetesAPIPort     *int         `json:"kubernetesAPIPort,omitempty"`
	KubernetesAPISSLPort  *int         `json:"kubernetesAPISSLPort,omitempty"`
	PodSubnet             string       `json:"podSubnet,omitempty"`
	ServiceSubnet         string       `json:"serviceSubnet,omitempty"`
	KubernetesClusterName string       `json:"kubernetesClusterName,omitempty"`
	IPFabricSubnets       string       `json:"ipFabricSubnets,omitempty"`
	IPFabricForwarding    *bool        `json:"ipFabricForwarding,omitempty"`
	IPFabricSnat          *bool        `json:"ipFabricSnat,omitempty"`
	HostNetworkService    *bool        `json:"hostNetworkService,omitempty"`
	KubernetesTokenFile   string       `json:"kubernetesTokenFile,omitempty"`
	RabbitmqUser          string       `json:"rabbitmqUser,omitempty"`
	RabbitmqPassword      string       `json:"rabbitmqPassword,omitempty"`
	RabbitmqVhost         string       `json:"rabbitmqVhost,omitempty"`
	LogLevel              string       `json:"logLevel,omitempty"`
	PublicFIPPool         string       `json:"publicFIPPool,omitempty"`
}

// KubemanagerList contains a list of Kubemanager.
// +k8s:openapi-gen=true
type KubemanagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubemanager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kubemanager{}, &KubemanagerList{})
}

// InstanceConfiguration creates kubemanager's instance sonfiguration
func (c *Kubemanager) InstanceConfiguration(request reconcile.Request,
	podList []corev1.Pod,
	client client.Client) error {
	instanceConfigMapName := request.Name + "-" + "kubemanager" + "-configmap"
	configMapInstanceDynamicConfig := &corev1.ConfigMap{}
	if err := client.Get(
		context.TODO(),
		types.NamespacedName{Name: instanceConfigMapName, Namespace: request.Namespace},
		configMapInstanceDynamicConfig); err != nil {
		return err
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		c.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	cassandraNodesInformation.FillWithDefaultValues()

	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(
		c.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	zookeeperNodesInformation.FillWithDefaultValues()

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		RabbitmqInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	rabbitmqNodesInformation.FillWithDefaultValues()

	configNodesInformation, err := NewConfigClusterConfiguration(
		c.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	configNodesInformation.FillWithDefaultValues()

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(
		c.Spec.ServiceConfiguration.AnalyticsInstance, request.Namespace, client)
	if err != nil {
		return err
	}
	analyticsNodesInformation.FillWithDefaultValues()

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: request.Namespace}, rabbitmqSecret)
		if err != nil {
			return err
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	kubemanagerConfig, err := c.ConfigurationParameters(client)
	if err != nil {
		return err
	}
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = kubemanagerConfig.RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = kubemanagerConfig.RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = kubemanagerConfig.RabbitmqVhost
	}

	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	var data = map[string]string{}
	for _, pod := range podList {

		configApiIPListCommaSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, ",")
		collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
		collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")
		rabbitmqSSLEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(rabbitmqNodesInformation.ServerIPList, ",")
		zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
		zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")
		cassandraEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
		cassandraEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraEndpointList, " ")
		var kubemanagerConfigBuffer bytes.Buffer
		secret := &corev1.Secret{}
		var secretName string
		if c.Spec.ServiceConfiguration.SecretName != "" {
			secretName = c.Spec.ServiceConfiguration.SecretName
		} else {
			secretName = request.Name + "-kubemanager-secret"
		}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: request.Namespace}, secret); err != nil {
			return err
		}
		token := string(secret.Data["token"])
		err = configtemplates.KubemanagerConfig.Execute(&kubemanagerConfigBuffer, struct {
			Token                    string
			ListenAddress            string
			InstrospectListenAddress string
			CloudOrchestrator        string
			KubernetesAPIServer      string
			KubernetesAPIPort        string
			KubernetesAPISSLPort     string
			KubernetesClusterName    string
			PodSubnet                string
			IPFabricSubnet           string
			ServiceSubnet            string
			IPFabricForwarding       string
			IPFabricSnat             string
			APIServerList            string
			APIServerPort            string
			CassandraServerList      string
			ZookeeperServerList      string
			RabbitmqServerList       string
			RabbitmqServerPort       string
			CollectorServerList      string
			HostNetworkService       string
			RabbitmqUser             string
			RabbitmqPassword         string
			RabbitmqVhost            string
			CAFilePath               string
			LogLevel                 string
			PublicFIPPool            string
			AuthMode                 AuthenticationMode
			KeystoneAuthParameters   *KeystoneAuthParameters
		}{
			Token:                    token,
			ListenAddress:            pod.Status.PodIP,
			InstrospectListenAddress: c.Spec.CommonConfiguration.IntrospectionListenAddress(pod.Status.PodIP),
			CloudOrchestrator:        kubemanagerConfig.CloudOrchestrator,
			KubernetesAPIServer:      kubemanagerConfig.KubernetesAPIServer,
			KubernetesAPIPort:        strconv.Itoa(*kubemanagerConfig.KubernetesAPIPort),
			KubernetesAPISSLPort:     strconv.Itoa(*kubemanagerConfig.KubernetesAPISSLPort),
			KubernetesClusterName:    kubemanagerConfig.KubernetesClusterName,
			PodSubnet:                kubemanagerConfig.PodSubnet,
			IPFabricSubnet:           kubemanagerConfig.IPFabricSubnets,
			ServiceSubnet:            kubemanagerConfig.ServiceSubnet,
			IPFabricForwarding:       strconv.FormatBool(*kubemanagerConfig.IPFabricForwarding),
			IPFabricSnat:             strconv.FormatBool(*kubemanagerConfig.IPFabricSnat),
			APIServerList:            configApiIPListCommaSeparated,
			APIServerPort:            strconv.Itoa(configNodesInformation.APIServerPort),
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			RabbitmqServerPort:       strconv.Itoa(rabbitmqNodesInformation.Port),
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			HostNetworkService:       strconv.FormatBool(*kubemanagerConfig.HostNetworkService),
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 kubemanagerConfig.LogLevel,
			PublicFIPPool:            kubemanagerConfig.PublicFIPPool,
			AuthMode:                 c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters:   c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
		})
		if err != nil {
			panic(err)
		}
		data["kubemanager."+pod.Status.PodIP] = kubemanagerConfigBuffer.String()

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
			PodIP:                  pod.Status.PodIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+pod.Status.PodIP] = vncApiConfigBuffer.String()
	}

	configMapInstanceDynamicConfig.Data = data
	return client.Update(context.TODO(), configMapInstanceDynamicConfig)
}

// CreateConfigMap creates empty configmap
func (c *Kubemanager) CreateConfigMap(configMapName string, client client.Client, scheme *runtime.Scheme, request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName, client, scheme, request, "kubemanager", c)
}

// IsActive returns true if instance is active.
func (c *Kubemanager) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// CreateSecret creates a secret.
func (c *Kubemanager) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"kubemanager",
		c)
}

// PrepareSTS prepares the intended deployment for the Kubemanager object.
func (c *Kubemanager) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "kubemanager", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the Kubemanager deployment.
func (c *Kubemanager) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Kubemanager) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// CreateSTS creates the STS.
func (c *Kubemanager) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Kubemanager) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, client client.Client) (bool, error) {
	return UpdateServiceSTS(c, instanceType, sts, false, client)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Kubemanager) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

//PodsCertSubjects gets list of Kubemanager pods certificate subjets which can be passed to the certificate API
func (c *Kubemanager) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, altIPs)
}

// SetInstanceActive sets the Kubemanager instance to active.
func (c *Kubemanager) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, sts, request, c)
}

// ManageNodeStatus updates node status
func (c *Kubemanager) ManageNodeStatus(podNameIPMap map[string]string,
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

// ConfigurationParameters creates KubemanagerConfiguration
func (c *Kubemanager) ConfigurationParameters(client client.Client) (*KubemanagerConfiguration, error) {

	cinfo, err := ClusterParameters(client)
	if err != nil {
		return nil, err
	}

	var logLevel string = LogLevel
	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	}

	var cloudOrchestrator string = CloudOrchestrator
	if c.Spec.ServiceConfiguration.CloudOrchestrator != "" {
		cloudOrchestrator = c.Spec.ServiceConfiguration.CloudOrchestrator
	}

	var kubernetesAPIServer string
	if kubernetesAPIServer, err = cinfo.KubernetesAPIServer(); err != nil {
		return nil, err
	}

	var kubernetesAPISSLPort int
	if kubernetesAPISSLPort, err = cinfo.KubernetesAPISSLPort(); err != nil {
		return nil, err
	}

	var ipFabricSubnets string = KubernetesIpFabricSubnets
	if c.Spec.ServiceConfiguration.IPFabricSubnets != "" {
		ipFabricSubnets = c.Spec.ServiceConfiguration.IPFabricSubnets
	}

	var ipFabricForwarding bool = KubernetesIPFabricForwarding
	var ipFabricSnat bool = KubernetesIPFabricSnat

	if c.Spec.ServiceConfiguration.IPFabricForwarding != nil {
		ipFabricForwarding = *c.Spec.ServiceConfiguration.IPFabricForwarding
	}

	var hostNetworkService bool = KubernetesHostNetworkService
	if c.Spec.ServiceConfiguration.HostNetworkService != nil {
		hostNetworkService = *c.Spec.ServiceConfiguration.HostNetworkService
	}

	if c.Spec.ServiceConfiguration.IPFabricSnat != nil {
		ipFabricSnat = *c.Spec.ServiceConfiguration.IPFabricSnat
	}

	var publicFIPPool string = fmt.Sprintf(KubernetesPublicFIPPoolTemplate, cinfo.ClusterName)
	if c.Spec.ServiceConfiguration.PublicFIPPool != "" {
		publicFIPPool = c.Spec.ServiceConfiguration.PublicFIPPool
	}

	kubemanagerConfiguration := &KubemanagerConfiguration{}
	kubemanagerConfiguration.LogLevel = logLevel
	kubemanagerConfiguration.CloudOrchestrator = cloudOrchestrator
	kubemanagerConfiguration.KubernetesClusterName = cinfo.ClusterName
	kubemanagerConfiguration.KubernetesAPIServer = kubernetesAPIServer
	kubemanagerConfiguration.KubernetesAPISSLPort = &kubernetesAPISSLPort
	var kubernetesAPIPort int = KubernetesApiPort
	kubemanagerConfiguration.KubernetesAPIPort = &kubernetesAPIPort
	kubemanagerConfiguration.PodSubnet = cinfo.Networking.PodSubnet
	kubemanagerConfiguration.ServiceSubnet = cinfo.Networking.ServiceSubnet
	kubemanagerConfiguration.IPFabricSubnets = ipFabricSubnets
	kubemanagerConfiguration.IPFabricForwarding = &ipFabricForwarding
	kubemanagerConfiguration.HostNetworkService = &hostNetworkService
	kubemanagerConfiguration.IPFabricSnat = &ipFabricSnat
	kubemanagerConfiguration.PublicFIPPool = publicFIPPool

	return kubemanagerConfiguration, nil
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Kubemanager) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
