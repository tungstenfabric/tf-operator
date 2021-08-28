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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kubemanager is the Schema for the kubemanager API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Kubemanager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubemanagerSpec   `json:"spec,omitempty"`
	Status KubemanagerStatus `json:"status,omitempty"`
}

// KubemanagerSpec is the Spec for the kubemanager API.
// +k8s:openapi-gen=true
type KubemanagerSpec struct {
	CommonConfiguration  PodConfiguration         `json:"commonConfiguration,omitempty"`
	ServiceConfiguration KubemanagerConfiguration `json:"serviceConfiguration"`
}

// KubemanagerStatus is the Status for the kubemanager API.
// +k8s:openapi-gen=true
type KubemanagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	CommonStatus `json:",inline"`
}

// KubemanagerConfiguration is the configuration for the kubemanager API.
// +k8s:openapi-gen=true
type KubemanagerConfiguration struct {
	Containers           []*Container `json:"containers,omitempty"`
	CloudOrchestrator    string       `json:"cloudOrchestrator,omitempty"`
	KubernetesAPIServer  string       `json:"kubernetesAPIServer,omitempty"`
	KubernetesAPIPort    *int         `json:"kubernetesAPIPort,omitempty"`
	KubernetesAPISSLPort *int         `json:"kubernetesAPISSLPort,omitempty"`
	PodSubnet            string       `json:"podSubnet,omitempty"`
	ServiceSubnet        string       `json:"serviceSubnet,omitempty"`
	IPFabricSubnets      string       `json:"ipFabricSubnets,omitempty"`
	IPFabricForwarding   *bool        `json:"ipFabricForwarding,omitempty"`
	IPFabricSnat         *bool        `json:"ipFabricSnat,omitempty"`
	HostNetworkService   *bool        `json:"hostNetworkService,omitempty"`
	KubernetesTokenFile  string       `json:"kubernetesTokenFile,omitempty"`
	PublicFIPPool        string       `json:"publicFIPPool,omitempty"`
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
func (c *Kubemanager) InstanceConfiguration(podList []corev1.Pod, client client.Client,
) (data map[string]string, err error) {
	data, err = make(map[string]string), nil

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		CassandraInstance, c.Namespace, client)
	if err != nil {
		return
	}
	cassandraNodesInformation.FillWithDefaultValues()

	zookeeperNodesInformation, err := NewZookeeperClusterConfiguration(
		ZookeeperInstance, c.Namespace, client)
	if err != nil {
		return
	}
	zookeeperNodesInformation.FillWithDefaultValues()

	rabbitmqNodesInformation, err := NewRabbitmqClusterConfiguration(
		RabbitmqInstance, c.Namespace, client)
	if err != nil {
		return
	}
	rabbitmqNodesInformation.FillWithDefaultValues()

	configNodesInformation, err := NewConfigClusterConfiguration(
		ConfigInstance, c.Namespace, client)
	if err != nil {
		return
	}
	configNodesInformation.FillWithDefaultValues()

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(AnalyticsInstance, c.Namespace, client)
	if err != nil {
		return
	}
	analyticsNodesInformation.FillWithDefaultValues()

	var rabbitmqSecretUser string
	var rabbitmqSecretPassword string
	var rabbitmqSecretVhost string
	if rabbitmqNodesInformation.Secret != "" {
		rabbitmqSecret := &corev1.Secret{}
		_err := client.Get(context.TODO(), types.NamespacedName{Name: rabbitmqNodesInformation.Secret, Namespace: c.Namespace}, rabbitmqSecret)
		if _err != nil {
			err = _err
			return
		}
		rabbitmqSecretUser = string(rabbitmqSecret.Data["user"])
		rabbitmqSecretPassword = string(rabbitmqSecret.Data["password"])
		rabbitmqSecretVhost = string(rabbitmqSecret.Data["vhost"])
	}

	cinfo, err := ClusterParameters(client)
	if err != nil {
		return nil, err
	}

	kubemanagerConfig, err := c.ConfigurationParameters(cinfo)
	if err != nil {
		return
	}
	if rabbitmqSecretUser == "" {
		rabbitmqSecretUser = RabbitmqUser
	}
	if rabbitmqSecretPassword == "" {
		rabbitmqSecretPassword = RabbitmqPassword
	}
	if rabbitmqSecretVhost == "" {
		rabbitmqSecretVhost = RabbitmqVhost
	}

	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })

	configApiIPListCommaSeparated := configtemplates.JoinListWithSeparator(configNodesInformation.APIServerIPList, ",")
	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")
	rabbitmqSSLEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(rabbitmqNodesInformation.ServerIPList, ",")
	zookeeperEndpointList := configtemplates.EndpointList(zookeeperNodesInformation.ServerIPList, zookeeperNodesInformation.ClientPort)
	zookeeperEndpointListCommaSeparated := configtemplates.JoinListWithSeparator(zookeeperEndpointList, ",")
	cassandraEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.Port)
	cassandraEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraEndpointList, " ")

	for _, pod := range podList {
		var kubemanagerConfigBuffer bytes.Buffer
		secret := &corev1.Secret{}
		if err = client.Get(context.TODO(), types.NamespacedName{Name: c.Name + "-kubemanager-secret", Namespace: c.Namespace}, secret); err != nil {
			return
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
			KeystoneAuthParameters   KeystoneAuthParameters
		}{
			Token:                    token,
			ListenAddress:            pod.Status.PodIP,
			InstrospectListenAddress: c.Spec.CommonConfiguration.IntrospectionListenAddress(pod.Status.PodIP),
			CloudOrchestrator:        kubemanagerConfig.CloudOrchestrator,
			KubernetesAPIServer:      kubemanagerConfig.KubernetesAPIServer,
			KubernetesAPIPort:        strconv.Itoa(*kubemanagerConfig.KubernetesAPIPort),
			KubernetesAPISSLPort:     strconv.Itoa(*kubemanagerConfig.KubernetesAPISSLPort),
			KubernetesClusterName:    cinfo.ClusterName,
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
			CAFilePath:               SignerCAFilepath,
			LogLevel:                 ConvertLogLevel(c.Spec.CommonConfiguration.LogLevel),
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
			KeystoneAuthParameters KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          configApiIPListCommaSeparated,
			APIServerPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
			PodIP:                  pod.Status.PodIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+pod.Status.PodIP] = vncApiConfigBuffer.String()
	}

	return
}

// CreateConfigMap creates empty configmap
func (c *Kubemanager) CreateConfigMap(configMapName string, client client.Client, scheme *runtime.Scheme, request reconcile.Request) (*corev1.ConfigMap, error) {

	data := make(map[string]string)
	data["run-kubemanager.sh"] = c.CommonStartupScript(
		"exec /usr/bin/contrail-kube-manager -c /etc/contrailconfigmaps/kubemanager.${POD_IP}",
		map[string]string{
			"kubemanager.${POD_IP}":     "",
			"vnc_api_lib.ini.${POD_IP}": "vnc_api_lib.ini",
		})

	return CreateConfigMap(configMapName, client, scheme, request, "kubemanager", data, c)
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

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Kubemanager) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

// SetInstanceActive sets the Kubemanager instance to active.
func (c *Kubemanager) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, degradedStatus, sts, request, c)
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
func (c *Kubemanager) ConfigurationParameters(cinfo *KubernetesClusterConfig) (*KubemanagerConfiguration, error) {
	var err error

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
	kubemanagerConfiguration.CloudOrchestrator = cloudOrchestrator
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
