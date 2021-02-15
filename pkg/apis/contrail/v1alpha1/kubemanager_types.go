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

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"

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
	RabbitmqInstance         string `json:"rabbitmqInstance,omitempty"`
	ConfigInstance           string `json:"configInstance,omitempty"`
}

// KubemanagerConfiguration is the configuration for the kubemanagers API.
// +k8s:openapi-gen=true
type KubemanagerConfiguration struct {
	UseKubeadmConfig      *bool        `json:"useKubeadmConfig,omitempty"`
	Containers            []*Container `json:"containers,omitempty"`
	ServiceAccount        string       `json:"serviceAccount,omitempty"`
	ClusterRole           string       `json:"clusterRole,omitempty"`
	ClusterRoleBinding    string       `json:"clusterRoleBinding,omitempty"`
	CloudOrchestrator     string       `json:"cloudOrchestrator,omitempty"`
	SecretName            string       `json:"secretName,omitempty"`
	KubernetesAPIServer   string       `json:"kubernetesAPIServer,omitempty"`
	KubernetesAPIPort     *int         `json:"kubernetesAPIPort,omitempty"`
	KubernetesAPISSLPort  *int         `json:"kubernetesAPISSLPort,omitempty"`
	PodSubnets            string       `json:"podSubnets,omitempty"`
	ServiceSubnets        string       `json:"serviceSubnets,omitempty"`
	KubernetesClusterName string       `json:"kubernetesClusterName,omitempty"`
	IPFabricSubnets       string       `json:"ipFabricSubnets,omitempty"`
	IPFabricForwarding    *bool        `json:"ipFabricForwarding,omitempty"`
	IPFabricSnat          *bool        `json:"ipFabricSnat,omitempty"`
	KubernetesTokenFile   string       `json:"kubernetesTokenFile,omitempty"`
	HostNetworkService    *bool        `json:"hostNetworkService,omitempty"`
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
	client client.Client,
	ci *k8s.ClusterInfo) error {
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
		c.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, client)
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

	kubemanagerConfig, err := c.ConfigurationParameters(ci)
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
		configCollectorEndpointList := configtemplates.EndpointList(configNodesInformation.CollectorServerIPList, configNodesInformation.CollectorPort)
		configCollectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(configCollectorEndpointList, " ")
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
			secretName = "contrail-kubemanager-secret"
		}
		if err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: request.Namespace}, secret); err != nil {
			return err
		}
		token := string(secret.Data["token"])
		configtemplates.KubemanagerConfig.Execute(&kubemanagerConfigBuffer, struct {
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
		}{
			Token:                    token,
			ListenAddress:            pod.Status.PodIP,
			InstrospectListenAddress: c.Spec.CommonConfiguration.IntrospectionListenAddress(pod.Status.PodIP),
			CloudOrchestrator:        kubemanagerConfig.CloudOrchestrator,
			KubernetesAPIServer:      kubemanagerConfig.KubernetesAPIServer,
			KubernetesAPIPort:        strconv.Itoa(*kubemanagerConfig.KubernetesAPIPort),
			KubernetesAPISSLPort:     strconv.Itoa(*kubemanagerConfig.KubernetesAPISSLPort),
			KubernetesClusterName:    kubemanagerConfig.KubernetesClusterName,
			PodSubnet:                kubemanagerConfig.PodSubnets,
			IPFabricSubnet:           kubemanagerConfig.IPFabricSubnets,
			ServiceSubnet:            kubemanagerConfig.ServiceSubnets,
			IPFabricForwarding:       strconv.FormatBool(*kubemanagerConfig.IPFabricForwarding),
			IPFabricSnat:             strconv.FormatBool(*kubemanagerConfig.IPFabricSnat),
			APIServerList:            configApiIPListCommaSeparated,
			APIServerPort:            strconv.Itoa(configNodesInformation.APIServerPort),
			CassandraServerList:      cassandraEndpointListSpaceSeparated,
			ZookeeperServerList:      zookeeperEndpointListCommaSeparated,
			RabbitmqServerList:       rabbitmqSSLEndpointListCommaSeparated,
			RabbitmqServerPort:       strconv.Itoa(rabbitmqNodesInformation.Port),
			CollectorServerList:      configCollectorEndpointListSpaceSeparated,
			HostNetworkService:       strconv.FormatBool(*kubemanagerConfig.HostNetworkService),
			RabbitmqUser:             rabbitmqSecretUser,
			RabbitmqPassword:         rabbitmqSecretPassword,
			RabbitmqVhost:            rabbitmqSecretVhost,
			CAFilePath:               certificates.SignerCAFilepath,
			LogLevel:                 kubemanagerConfig.LogLevel,
			PublicFIPPool:            kubemanagerConfig.PublicFIPPool,
		})
		data["kubemanager."+pod.Status.PodIP] = kubemanagerConfigBuffer.String()

		var vncApiConfigBuffer bytes.Buffer
		configtemplates.ConfigAPIVNC.Execute(&vncApiConfigBuffer, struct {
			APIServerList string
			APIServerPort string
			CAFilePath    string
			AuthMode      string
		}{
			APIServerList: configApiIPListCommaSeparated,
			APIServerPort: strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:    certificates.SignerCAFilepath,
			AuthMode:      string(configNodesInformation.AuthMode),
		})
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

// SetPodsToReady sets Kubemanager PODs to ready.
func (c *Kubemanager) SetPodsToReady(podIPList []corev1.Pod, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// CreateSTS creates the STS.
func (c *Kubemanager) CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) error {
	return CreateSTS(sts, instanceType, request, reconcileClient)
}

// UpdateSTS updates the STS.
func (c *Kubemanager) UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	return UpdateSTS(sts, instanceType, request, reconcileClient, "rolling")
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Kubemanager) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, &c.Spec.CommonConfiguration, request, reconcileClient)
}

//PodsCertSubjects gets list of Kubemanager pods certificate subjets which can be passed to the certificate API
func (c *Kubemanager) PodsCertSubjects(podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// SetInstanceActive sets the Kubemanager instance to active.
func (c *Kubemanager) SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, sts, request, c)
}

// ManageNodeStatus updates node status
func (c *Kubemanager) ManageNodeStatus(podNameIPMap map[string]string, client client.Client) error {
	c.Status.Nodes = podNameIPMap
	return client.Status().Update(context.TODO(), c)
}

// EnsureServiceAccount creates ServiceAccoung, Secret, ClusterRole and ClusterRoleBinding
// objects if they are not exist.
func (c *Kubemanager) EnsureServiceAccount(
	serviceAccountName string,
	clusterRoleName string,
	clusterRoleBindingName string,
	secretName string,
	client client.Client,
	scheme *runtime.Scheme) error {

	return EnsureServiceAccount(serviceAccountName, clusterRoleName, clusterRoleBindingName, secretName,
		client, scheme, c)
}

// ConfigurationParameters creates KubemanagerConfiguration
func (c *Kubemanager) ConfigurationParameters(ci *k8s.ClusterInfo) (*KubemanagerConfiguration, error) {

	var useKubeadmConfig bool = KubernetesUseKubeadm
	var logLevel string = LogLevel
	var cloudOrchestrator string = CloudOrchestrator
	var kubernetesClusterName string = KubernetesClusterName
	var kubernetesApiServer string = KubernetesApiServer
	var kubernetesApiSSLPort int = KubernetesApiSSLPort
	var kubernetesApiPort int = KubernetesApiPort
	var podSubnets string = KubernetesPodSubnets
	var serviceSubnets string = KubernetesServiceSubnets
	var ipFabricSubnets string = KubernetesIpFabricSubnets
	var ipFabricForwarding bool = KubernetesIPFabricForwarding
	var ipFabricSnat bool = KubernetesIPFabricSnat
	var publicFIPPool string = KubernetesPublicFIPPool
	var hostNetworkService bool = KubernetesHostNetworkService

	cinfo, err := ci.ClusterParameters()
	if err != nil {
		return nil, err
	}

	if c.Spec.ServiceConfiguration.UseKubeadmConfig != nil {
		useKubeadmConfig = *c.Spec.ServiceConfiguration.UseKubeadmConfig
	}

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	}

	if c.Spec.ServiceConfiguration.CloudOrchestrator != "" {
		cloudOrchestrator = c.Spec.ServiceConfiguration.CloudOrchestrator
	}

	if c.Spec.ServiceConfiguration.KubernetesClusterName != "" {
		kubernetesClusterName = c.Spec.ServiceConfiguration.KubernetesClusterName
	} else {
		if useKubeadmConfig {
			kubernetesClusterName = cinfo.ClusterName
		}
	}

	if c.Spec.ServiceConfiguration.KubernetesAPIServer != "" {
		kubernetesApiServer = c.Spec.ServiceConfiguration.KubernetesAPIServer
	} else {
		if useKubeadmConfig {
			kubernetesApiServer, err = cinfo.KubernetesAPIServer()
			if err != nil {
				return nil, err
			}
		}
	}

	if c.Spec.ServiceConfiguration.KubernetesAPISSLPort != nil {
		kubernetesApiSSLPort = *c.Spec.ServiceConfiguration.KubernetesAPISSLPort
	} else {
		if useKubeadmConfig {
			kubernetesApiSSLPort, err = cinfo.KubernetesAPISSLPort()
			if err != nil {
				return nil, err
			}
		}
	}

	if c.Spec.ServiceConfiguration.KubernetesAPIPort != nil {
		kubernetesApiPort = *c.Spec.ServiceConfiguration.KubernetesAPIPort
	}

	if c.Spec.ServiceConfiguration.PodSubnets != "" {
		podSubnets = c.Spec.ServiceConfiguration.PodSubnets
	} else {
		if useKubeadmConfig {
			podSubnets = cinfo.Networking.PodSubnet
		}
	}

	if c.Spec.ServiceConfiguration.ServiceSubnets != "" {
		serviceSubnets = c.Spec.ServiceConfiguration.ServiceSubnets
	} else {
		if useKubeadmConfig {
			serviceSubnets = cinfo.Networking.ServiceSubnet
		}
	}

	if c.Spec.ServiceConfiguration.IPFabricSubnets != "" {
		ipFabricSubnets = c.Spec.ServiceConfiguration.IPFabricSubnets
	}

	if c.Spec.ServiceConfiguration.IPFabricForwarding != nil {
		ipFabricForwarding = *c.Spec.ServiceConfiguration.IPFabricForwarding
	}

	if c.Spec.ServiceConfiguration.HostNetworkService != nil {
		hostNetworkService = *c.Spec.ServiceConfiguration.HostNetworkService
	}

	if c.Spec.ServiceConfiguration.IPFabricSnat != nil {
		ipFabricSnat = *c.Spec.ServiceConfiguration.IPFabricSnat
	}

	if c.Spec.ServiceConfiguration.PublicFIPPool != "" {
		publicFIPPool = c.Spec.ServiceConfiguration.PublicFIPPool
	}

	kubemanagerConfiguration := &KubemanagerConfiguration{}
	kubemanagerConfiguration.LogLevel = logLevel
	kubemanagerConfiguration.CloudOrchestrator = cloudOrchestrator
	kubemanagerConfiguration.KubernetesClusterName = kubernetesClusterName
	kubemanagerConfiguration.KubernetesAPIServer = kubernetesApiServer
	kubemanagerConfiguration.KubernetesAPISSLPort = &kubernetesApiSSLPort
	kubemanagerConfiguration.KubernetesAPIPort = &kubernetesApiPort
	kubemanagerConfiguration.PodSubnets = podSubnets
	kubemanagerConfiguration.ServiceSubnets = serviceSubnets
	kubemanagerConfiguration.IPFabricSubnets = ipFabricSubnets
	kubemanagerConfiguration.IPFabricForwarding = &ipFabricForwarding
	kubemanagerConfiguration.HostNetworkService = &hostNetworkService
	kubemanagerConfiguration.UseKubeadmConfig = &useKubeadmConfig
	kubemanagerConfiguration.IPFabricSnat = &ipFabricSnat
	kubemanagerConfiguration.PublicFIPPool = publicFIPPool

	return kubemanagerConfiguration, nil
}
