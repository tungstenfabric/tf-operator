package v1alpha1

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	configtemplates "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var vrouter_log = logf.Log.WithName("controller_vrouter")

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Vrouter is the Schema for the vrouters API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Vrouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VrouterSpec   `json:"spec,omitempty"`
	Status VrouterStatus `json:"status,omitempty"`
}

// VrouterStatus is the Status for vrouter API.
// +k8s:openapi-gen=true
// TODO: after update to controllter-tool v0.4 rework AgentStatus
// to make it map instead of [] for performance
// (https://github.com/operator-framework/operator-sdk/issues/2485
// https://github.com/kubernetes-sigs/controller-tools/pull/317)
type VrouterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Ports               ConfigStatusPorts `json:"ports,omitempty"`
	Nodes               map[string]string `json:"nodes,omitempty"`
	Active              *bool             `json:"active,omitempty"`
	ActiveOnControllers *bool             `json:"activeOnControllers,omitempty"`
	Agents              []*AgentStatus    `json:"agents,omitempty"`
}

// AgentStatus is the Status of the agent.
// +k8s:openapi-gen=true
type AgentStatus struct {
	Name            string             `json:"name,omitempty"`
	Status          AgentServiceStatus `json:"status,omitempty"`
	ControlNodes    string             `json:"controlNodes,omitempty"`
	ConfigNodes     string             `json:"configNodes,omitempty"`
	EncryptedParams string             `json:"encryptedParams,omitempty"`
}

// AgentServiceStatus is the status value.
// +k8s:openapi-gen=true
type AgentServiceStatus string

const (
	// Starting - agent status  is starting.
	// +k8s:openapi-gen=true
	Starting AgentServiceStatus = "Starting"
	// Ready - agent status  is ready.
	// +k8s:openapi-gen=true
	Ready AgentServiceStatus = "Ready"
	// Updating - agent status  is updating.
	// +k8s:openapi-gen=true
	Updating AgentServiceStatus = "Updating"
)

// VrouterSpec is the Spec for the vrouter API.
// +k8s:openapi-gen=true
type VrouterSpec struct {
	CommonConfiguration  PodConfiguration     `json:"commonConfiguration,omitempty"`
	ServiceConfiguration VrouterConfiguration `json:"serviceConfiguration"`
}

// VrouterConfiguration is the Spec for the vrouter API.
// +k8s:openapi-gen=true
type VrouterConfiguration struct {
	Containers         []*Container  `json:"containers,omitempty"`
	Gateway            string        `json:"gateway,omitempty"`
	PhysicalInterface  string        `json:"physicalInterface,omitempty"`
	MetaDataSecret     string        `json:"metaDataSecret,omitempty"`
	Distribution       *Distribution `json:"distribution,omitempty"`
	ServiceAccount     string        `json:"serviceAccount,omitempty"`
	ClusterRole        string        `json:"clusterRole,omitempty"`
	ClusterRoleBinding string        `json:"clusterRoleBinding,omitempty"`
	// What is it doing?
	// VrouterEncryption   bool              `json:"vrouterEncryption,omitempty"`
	// What is it doing?
	ContrailStatusImage string `json:"contrailStatusImage,omitempty"`
	// What is it doing?
	EnvVariablesConfig map[string]string `json:"envVariablesConfig,omitempty"`

	// New params for vrouter configuration
	CloudOrchestrator string `json:"cloudOrchestrator,omitempty"`
	HypervisorType    string `json:"hypervisorType,omitempty"`

	// Collector
	StatsCollectorDestinationPath string `json:"statsCollectorDestinationPath,omitempty"`
	CollectorPort                 string `json:"collectorPort,omitempty"`

	// Config
	ConfigApiPort             string `json:"configApiPort,omitempty"`
	ConfigApiServerCaCertfile string `json:"configApiServerCaCertfile,omitempty"`
	ConfigApiSslEnable        *bool  `json:"configApiSslEnable,omitempty"`

	// DNS
	DnsServerPort string `json:"dnsServerPort,omitempty"`

	// Host
	DpdkUioDriver         string `json:"dpdkUioDriver,omitempty"`
	SriovPhysicalInterace string `json:"sriovPhysicalInterface,omitempty"`
	SriovPhysicalNetwork  string `json:"sriovPhysicalNetwork,omitempty"`
	SriovVf               string `json:"sriovVf,omitempty"`

	// Introspect
	IntrospectListenAll *bool `json:"introspectListenAll,omitempty"`
	IntrospectSslEnable *bool `json:"introspectSslEnable,omitempty"`

	// Keystone authentication
	KeystoneAuthAdminPort         string `json:"keystoneAuthAdminPort,omitempty"`
	KeystoneAuthCaCertfile        string `json:"keystoneAuthCaCertfile,omitempty"`
	KeystoneAuthCertfile          string `json:"keystoneAuthCertfile,omitempty"`
	KeystoneAuthHost              string `json:"keystoneAuthHost,omitempty"`
	KeystoneAuthInsecure          *bool  `json:"keystoneAuthInsecure,omitempty"`
	KeystoneAuthKeyfile           string `json:"keystoneAuthKeyfile,omitempty"`
	KeystoneAuthProjectDomainName string `json:"keystoneAuthProjectDomainName,omitempty"`
	KeystoneAuthProto             string `json:"keystoneAuthProto,omitempty"`
	KeystoneAuthRegionName        string `json:"keystoneAuthRegionName,omitempty"`
	KeystoneAuthUrlTokens         string `json:"keystoneAuthUrlTokens,omitempty"`
	KeystoneAuthUrlVersion        string `json:"keystoneAuthUrlVersion,omitempty"`
	KeystoneAuthUserDomainName    string `json:"keystoneAuthUserDomainName,omitempty"`
	KeystoneAuthAdminPassword     string `json:"keystoneAuthAdminPassword,omitempty"`

	// Kubernetes
	K8sToken                string `json:"k8sToken,omitempty"`
	K8sTokenFile            string `json:"k8sTokenFile,omitempty"`
	KubernetesApiPort       string `json:"kubernetesApiPort,omitempty"`
	KubernetesApiSecurePort string `json:"kubernetesApiSecurePort,omitempty"`
	KubernetesPodSubnets    string `json:"kubernetesPodSubnets,omitempty"`
	KubernetesClusterName   string `json:"kubernetesClusterName,omitempty"`
	UseKubeadmConfig        *bool  `json:"useKubeadmConfig,omitempty"`

	// Logging
	LogDir   string `json:"logDir,omitempty"`
	LogLevel string `json:"logLevel,omitempty"`
	LogLocal *int   `json:"logLocal,omitempty"`

	// Metadata
	MetadataProxySecret   string `json:"metadataProxySecret,omitempty"`
	MetadataSslCaCertfile string `json:"metadataSslCaCertfile,omitempty"`
	MetadataSslCertfile   string `json:"metadataSslCertfile,omitempty"`
	MetadataSslCertType   string `json:"metadataSslCertType,omitempty"`
	MetadataSslEnable     string `json:"metadataSslEnable,omitempty"`
	MetadataSslKeyfile    string `json:"metadataSslKeyfile,omitempty"`

	// Openstack
	BarbicanTenantName string `json:"barbicanTenantName,omitempty"`
	BarbicanPassword   string `json:"barbicanPassword,omitempty"`
	BarbicanUser       string `json:"barbicanUser,omitempty"`

	// Sandesh
	SandeshCaCertfile string `json:"sandeshCaCertfile,omitempty"`
	SandeshCertfile   string `json:"sandeshCertfile,omitempty"`
	SandeshKeyfile    string `json:"sandeshKeyfile,omitempty"`
	SandeshSslEnable  *bool  `json:"sandeshSslEnable,omitempty"`

	// Server SSL
	ServerCaCertfile string `json:"serverCaCertfile,omitempty"`
	ServerCertfile   string `json:"serverCertfile,omitempty"`
	ServerKeyfile    string `json:"serverKeyfile,omitempty"`
	SslEnable        *bool  `json:"sslEnable,omitempty"`
	SslInsecure      *bool  `json:"sslInsecure,omitempty"`

	// TSN
	TsnAgentMode string `json:"tsnAgentMode,omitempty"`

	// vRouter
	AgentMode                       string `json:"agentMode,omitempty"`
	FabricSnatHashTableSize         string `json:"fabricSntHashTableSize,omitempty"`
	PriorityBandwidth               string `json:"priorityBandwidth,omitempty"`
	PriorityId                      string `json:"priorityId,omitempty"`
	PriorityScheduling              string `json:"priorityScheduling,omitempty"`
	PriorityTagging                 *bool  `json:"priorityTagging,omitempty"`
	QosDefHwQueue                   *bool  `json:"qosDefHwQueue,omitempty"`
	QosLogicalQueues                string `json:"qosLogicalQueues,omitempty"`
	QosQueueId                      string `json:"qosQueueId,omitempty"`
	RequiredKernelVrouterEncryption string `json:"requiredKernelVrouterEncryption,omitempty"`
	SampleDestination               string `json:"sampleDestination,omitempty"`
	SloDestination                  string `json:"sloDestination,omitempty"`
	VrouterCryptInterface           string `json:"vrouterCryptInterface,omitempty"`
	VrouterDecryptInterface         string `json:"vrouterDecryptInterface,omitempty"`
	VrouterDecyptKey                string `json:"vrouterDecryptKey,omitempty"`
	VrouterEncryption               *bool  `json:"vrouterEncryption,omitempty"`
	VrouterGateway                  string `json:"vrouterGateway,omitempty"`

	// XMPP
	Subclaster           string `json:"subclaster,omitempty"`
	XmppServerCaCertfile string `json:"xmppServerCaCertfile,omitempty"`
	XmppServerCertfile   string `json:"xmppServerCertfile,omitempty"`
	XmppServerKeyfile    string `json:"xmppServerKeyfile,omitempty"`
	XmppServerPort       string `json:"xmppServerPort,omitempty"`
	XmppSslEnable        *bool  `json:"xmmpSslEnable,omitempty"`

	// HugePages
	HugePages2M *int `json:"hugePages2M,omitempty"`
	HugePages1G *int `json:"hugePages1G,omitempty"`
}

// VrouterNodesConfiguration is the static configuration for vrouter.
// +k8s:openapi-gen=true
type VrouterNodesConfiguration struct {
	ControlNodesConfiguration *ControlClusterConfiguration `json:"controlNodesConfiguration,omitempty"`
	ConfigNodesConfiguration  *ConfigClusterConfiguration  `json:"configNodesConfiguration,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VrouterList contains a list of Vrouter.
type VrouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vrouter `json:"items"`
}

type Distribution string

const (
	RHEL   Distribution = "rhel"
	CENTOS Distribution = "centos"
	UBUNTU Distribution = "ubuntu"
)

const (
	VrouterAgentConfigMountPath string = "/etc/agentconfigmaps"
)

var VrouterDefaultContainers = []*Container{
	{
		Name:  "init",
		Image: "python:3.8.2-alpine",
	},
	{
		Name:  "nodeinit",
		Image: "tungstenfabric/contrail-node-init",
	},
	{
		Name:  "vrouteragent",
		Image: "tungstenfabric/contrail-vrouter-agent",
	},
	{
		Name:  "vroutercni",
		Image: "tungstenfabric/contrail-kubernetes-cni-init",
	},
	{
		Name:  "vrouterkernelbuildinit",
		Image: "tungstenfabric/contrail-vrouter-kernel-build-init",
	},
	{
		Name:  "vrouterkernelinit",
		Image: "tungstenfabric/contrail-vrouter-kernel-init",
	},
	{
		Name:  "multusconfig",
		Image: "busybox:1.31",
	},
}

var DefaultVrouter = VrouterConfiguration{
	Containers: VrouterDefaultContainers,
}

func init() {
	SchemeBuilder.Register(&Vrouter{}, &VrouterList{})
}

// CreateConfigMap creates configMap with specified name
func (c *Vrouter) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"vrouter",
		c)
}

// CreateSecret creates a secret.
func (c *Vrouter) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"vrouter",
		c)
}

// PrepareDaemonSet prepares the intended podList.
func (c *Vrouter) PrepareDaemonSet(ds *appsv1.DaemonSet,
	commonConfiguration *PodConfiguration,
	request reconcile.Request,
	scheme *runtime.Scheme,
	client client.Client) error {
	instanceType := "vrouter"
	SetDSCommonConfiguration(ds, commonConfiguration)
	ds.SetName(request.Name + "-" + instanceType + "-daemonset")
	ds.SetNamespace(request.Namespace)
	ds.SetLabels(map[string]string{"contrail_manager": instanceType,
		instanceType: request.Name})
	ds.Spec.Selector.MatchLabels = map[string]string{"contrail_manager": instanceType,
		instanceType: request.Name}
	ds.Spec.Template.SetLabels(map[string]string{"contrail_manager": instanceType,
		instanceType: request.Name})
	ds.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      instanceType,
						Operator: "Exists",
					}},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}
	err := controllerutil.SetControllerReference(c, ds, scheme)
	if err != nil {
		return err
	}
	return nil
}

// AddSecretVolumesToIntendedDS adds volumes to the Rabbitmq deployment.
func (c *Vrouter) AddSecretVolumesToIntendedDS(ds *appsv1.DaemonSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedDS(ds, volumeConfigMapMap)
}

// SetDSCommonConfiguration takes common configuration parameters
// and applies it to the pod.
func SetDSCommonConfiguration(ds *appsv1.DaemonSet,
	commonConfiguration *PodConfiguration) {
	if len(commonConfiguration.Tolerations) > 0 {
		ds.Spec.Template.Spec.Tolerations = commonConfiguration.Tolerations
	}
	if len(commonConfiguration.NodeSelector) > 0 {
		ds.Spec.Template.Spec.NodeSelector = commonConfiguration.NodeSelector
	}
	if commonConfiguration.HostNetwork != nil {
		ds.Spec.Template.Spec.HostNetwork = *commonConfiguration.HostNetwork
	} else {
		ds.Spec.Template.Spec.HostNetwork = false
	}
	if len(commonConfiguration.ImagePullSecrets) > 0 {
		imagePullSecretList := []corev1.LocalObjectReference{}
		for _, imagePullSecretName := range commonConfiguration.ImagePullSecrets {
			imagePullSecret := corev1.LocalObjectReference{
				Name: imagePullSecretName,
			}
			imagePullSecretList = append(imagePullSecretList, imagePullSecret)
		}
		ds.Spec.Template.Spec.ImagePullSecrets = imagePullSecretList
	}
}

// AddVolumesToIntendedDS adds volumes to a deployment.
func (c *Vrouter) AddVolumesToIntendedDS(ds *appsv1.DaemonSet, volumeConfigMapMap map[string]string) {
	volumeList := ds.Spec.Template.Spec.Volumes
	for configMapName, volumeName := range volumeConfigMapMap {
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		}
		volumeList = append(volumeList, volume)
	}
	ds.Spec.Template.Spec.Volumes = volumeList
}

// CreateDS creates the daemonset.
func (c *Vrouter) CreateDS(ds *appsv1.DaemonSet,
	commonConfiguration *PodConfiguration,
	instanceType string,
	request reconcile.Request,
	scheme *runtime.Scheme,
	reconcileClient client.Client) error {
	foundDS := &appsv1.DaemonSet{}
	err := reconcileClient.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-" + instanceType + "-daemonset", Namespace: request.Namespace}, foundDS)
	if err != nil {
		if errors.IsNotFound(err) {
			ds.Spec.Template.ObjectMeta.Labels["version"] = "1"
			err = reconcileClient.Create(context.TODO(), ds)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdateDS updates the daemonset.
func (c *Vrouter) UpdateDS(ds *appsv1.DaemonSet,
	commonConfiguration *PodConfiguration,
	instanceType string,
	request reconcile.Request,
	scheme *runtime.Scheme,
	reconcileClient client.Client) error {
	currentDS := &appsv1.DaemonSet{}
	err := reconcileClient.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-" + instanceType + "-daemonset", Namespace: request.Namespace}, currentDS)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	imagesChanged := false
	for _, intendedContainer := range ds.Spec.Template.Spec.Containers {
		for _, currentContainer := range currentDS.Spec.Template.Spec.Containers {
			if intendedContainer.Name == currentContainer.Name {
				if intendedContainer.Image != currentContainer.Image {
					imagesChanged = true
				}
			}
		}
	}
	if imagesChanged {

		ds.Spec.Template.ObjectMeta.Labels["version"] = currentDS.Spec.Template.ObjectMeta.Labels["version"]

		err = reconcileClient.Update(context.TODO(), ds)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetInstanceActive sets the instance to active.
func (c *Vrouter) SetInstanceActive(client client.Client, activeStatus *bool, ds *appsv1.DaemonSet, request reconcile.Request, object runtime.Object) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: request.Namespace},
		ds); err != nil {
		return err
	}
	active := false
	if ds.Status.DesiredNumberScheduled == ds.Status.NumberReady {
		active = true
	}

	*activeStatus = active
	if err := client.Status().Update(context.TODO(), object); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Vrouter) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client, getPhysicalInterface bool, getPhysicalInterfaceMac bool, getPrefixLength bool, getGateway bool) (*corev1.PodList, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, &c.Spec.CommonConfiguration, request, reconcileClient, false, getPhysicalInterface, getPhysicalInterfaceMac, getPrefixLength, getGateway)
}

//PodsCertSubjects gets list of Vrouter pods certificate subjets which can be passed to the certificate API
func (c *Vrouter) PodsCertSubjects(podList *corev1.PodList) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// CreateEnvConfigMap creates vRouter configMaps with rendered values
func (c *Vrouter) CreateEnvConfigMap(instanceType string, client client.Client, scheme *runtime.Scheme, request reconcile.Request) (*corev1.ConfigMap, error) {
	envVariablesConfigMapName := request.Name + "-" + instanceType + "-configmap-env"
	envVariablesConfigMap, err := c.CreateConfigMap(envVariablesConfigMapName, client, scheme, request)
	if err != nil {
		return nil, err
	}
	envVariablesConfigMap.Data = c.getVrouterEnvironmentData()
	return envVariablesConfigMap, client.Update(context.TODO(), envVariablesConfigMap)
}

// CreateCNIConfigMap creates vRouter configMaps with rendered values
func (c *Vrouter) CreateCNIConfigMap(client client.Client, scheme *runtime.Scheme, request reconcile.Request) (*corev1.ConfigMap, error) {
	config, err := c.GetCNIConfig(client, request)
	if err != nil {
		return nil, err
	}
	configMap, err := c.CreateConfigMap(request.Name+"-cni-config", client, scheme, request)
	if err != nil {
		return nil, err
	}
	configMap.Data["10-tf-cni.conf"] = config
	return configMap, client.Update(context.TODO(), configMap)
}

// SetPodsToReady sets Kubemanager PODs to ready.
func (c *Vrouter) SetPodsToReady(podIPList *corev1.PodList, client client.Client) error {
	return SetPodsToReady(podIPList, client)
}

// ManageNodeStatus manages nodes status
func (c *Vrouter) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) error {
	c.Status.Nodes = podNameIPMap
	err := client.Status().Update(context.TODO(), c)
	if err != nil {
		return err
	}
	return nil
}

// VrouterConfigurationParameters is a method for gathering data used in rendering vRouter configuration
func (c *Vrouter) VrouterConfigurationParameters() *VrouterConfiguration {
	vrouterConfiguration := c.Spec.ServiceConfiguration.DeepCopy()

	trueVal := true
	falseVal := false
	defCert := "/etc/certificates/server-${POD_IP}.crt"
	defKey := "/etc/certificates/server-key-${POD_IP}.pem"

	if vrouterConfiguration.LogLevel == "" {
		vrouterConfiguration.LogLevel = LogLevel
	}
	if vrouterConfiguration.LogLocal == nil {
		ll := LogLocal
		vrouterConfiguration.LogLocal = &ll
	}

	if vrouterConfiguration.VrouterEncryption == nil {
		vrouterConfiguration.VrouterEncryption = &falseVal
	}

	if vrouterConfiguration.CloudOrchestrator == "" {
		vrouterConfiguration.CloudOrchestrator = "kubernetes"
	}

	if vrouterConfiguration.HypervisorType == "" {
		vrouterConfiguration.HypervisorType = "kvm"
	}

	if vrouterConfiguration.SslEnable == nil {
		vrouterConfiguration.SslEnable = &trueVal
	}
	if vrouterConfiguration.ServerCaCertfile == "" {
		vrouterConfiguration.ServerCaCertfile = certificates.SignerCAFilepath
	}
	if vrouterConfiguration.ServerCertfile == "" {
		vrouterConfiguration.ServerCertfile = defCert
	}
	if vrouterConfiguration.ServerKeyfile == "" {
		vrouterConfiguration.ServerKeyfile = defKey
	}

	if vrouterConfiguration.ConfigApiSslEnable == nil {
		vrouterConfiguration.ConfigApiSslEnable = vrouterConfiguration.SslEnable
	}
	if vrouterConfiguration.ConfigApiServerCaCertfile == "" {
		vrouterConfiguration.ConfigApiServerCaCertfile = vrouterConfiguration.ServerCaCertfile
	}

	if vrouterConfiguration.XmppSslEnable == nil {
		vrouterConfiguration.XmppSslEnable = vrouterConfiguration.SslEnable
	}
	if vrouterConfiguration.XmppServerCaCertfile == "" {
		vrouterConfiguration.XmppServerCaCertfile = vrouterConfiguration.ServerCaCertfile
	}
	if vrouterConfiguration.XmppServerCertfile == "" {
		vrouterConfiguration.XmppServerCertfile = vrouterConfiguration.ServerCertfile
	}
	if vrouterConfiguration.XmppServerKeyfile == "" {
		vrouterConfiguration.XmppServerKeyfile = vrouterConfiguration.ServerKeyfile
	}

	if vrouterConfiguration.SandeshSslEnable == nil {
		vrouterConfiguration.SandeshSslEnable = vrouterConfiguration.SslEnable
	}
	if vrouterConfiguration.SandeshCaCertfile == "" {
		vrouterConfiguration.SandeshCaCertfile = vrouterConfiguration.ServerCaCertfile
	}
	if vrouterConfiguration.SandeshCertfile == "" {
		vrouterConfiguration.SandeshCertfile = vrouterConfiguration.ServerCertfile
	}
	if vrouterConfiguration.SandeshKeyfile == "" {
		vrouterConfiguration.SandeshKeyfile = vrouterConfiguration.ServerKeyfile
	}

	if vrouterConfiguration.IntrospectSslEnable == nil {
		vrouterConfiguration.IntrospectSslEnable = vrouterConfiguration.SslEnable
	}

	return vrouterConfiguration
}

func (c *Vrouter) getVrouterEnvironmentData() map[string]string {
	vrouterConfig := c.VrouterConfigurationParameters()
	envVariables := make(map[string]string)
	for key, value := range vrouterConfig.EnvVariablesConfig {
		envVariables[key] = value
	}
	return envVariables
}

// GetNodeDSPod returns daemonset pod by name
func (c *Vrouter) GetNodeDSPod(nodeName string, daemonset *appsv1.DaemonSet, clnt client.Client) *corev1.Pod {
	allPods := &corev1.PodList{}
	// var pod corev1.Pod
	_ = clnt.List(context.Background(), allPods)
	for _, pod := range allPods.Items {

		if pod.ObjectMeta.OwnerReferences != nil &&
			len(pod.ObjectMeta.OwnerReferences) > 0 &&
			pod.ObjectMeta.OwnerReferences[0].Name == daemonset.Name &&
			pod.Spec.NodeName == nodeName {
			return &pod
		}
	}
	return nil
}

// GetAgentNodes
func (c *Vrouter) GetAgentNodes(daemonset *appsv1.DaemonSet, clnt client.Client) *corev1.NodeList {

	// TODO get nodes based on node selector
	// for ns_key, ns_value := range daemonset.Spec.Template.Spec.NodeSelector {
	//   log.Info(fmt.Sprintf("Node selector = '%v' : '%v'",ns_key,ns_value))
	// }

	// Get Nodes for check agent Status
	// Using a typed object.
	nodeList := &corev1.NodeList{}
	_ = clnt.List(context.Background(), nodeList)

	return nodeList
}

// GetNodesByLabels
func (c *Vrouter) GetNodesByLabels(clnt client.Client, labels client.MatchingLabels) (string, error) {
	pods := &corev1.PodList{}
	if err := clnt.List(context.Background(), pods, labels); err != nil {
		return "", err
	}

	arrIps := []string{}
	for _, pod := range pods.Items {
		arrIps = append(arrIps, pod.Status.PodIP)
	}
	sort.Strings(arrIps)

	ips := strings.Join(arrIps[:], ", ")
	return ips, nil
}

type ClusterParams struct {
	ConfigNodes  string
	ControlNodes string
}

// GetControlNodes
func (c *Vrouter) GetControlNodes(clnt client.Client) string {
	ips, _ := c.GetNodesByLabels(clnt, client.MatchingLabels{"contrail_manager": "control"})
	return ips
}

// GetConfigNodes
func (c *Vrouter) GetConfigNodes(clnt client.Client) string {
	ips, _ := c.GetNodesByLabels(clnt, client.MatchingLabels{"contrail_manager": "config"})
	return ips
}

// GetParamsEnv
func (c *Vrouter) GetParamsEnv(clnt client.Client) string {
	vrouterConfig := c.VrouterConfigurationParameters()
	clusterParams := ClusterParams{ConfigNodes: c.GetConfigNodes(clnt), ControlNodes: c.GetConfigNodes(clnt)}

	var vrouterManifestParamsEnv bytes.Buffer
	configtemplates.VRouterAgentParams.Execute(&vrouterManifestParamsEnv, struct {
		ServiceConfig VrouterConfiguration
		ClusterParams ClusterParams
	}{
		ServiceConfig: *vrouterConfig,
		ClusterParams: clusterParams,
	})
	return vrouterManifestParamsEnv.String()
}

// SetParamsToAgents use `params.env` file from GetParamsEnv and throw it to all agents
func (c *Vrouter) SetParamsToAgents(request reconcile.Request, clnt client.Client) error {
	configName := types.NamespacedName{
		Name:      request.Name + "-vrouter-agent-config",
		Namespace: request.Namespace,
	}

	config := &corev1.ConfigMap{}
	if err := clnt.Get(context.Background(), configName, config); err != nil {
		return err
	}

	data := config.Data
	if data == nil {
		data = make(map[string]string)
	}
	data["params.env"] = c.GetParamsEnv(clnt)

	config.Data = data
	if err := clnt.Update(context.Background(), config); err != nil {
		return err
	}

	return nil
}

// EncryptString
func EncryptString(str string) string {
	h := sha1.New()
	io.WriteString(h, str)
	key := hex.EncodeToString(h.Sum(nil))

	return string(key)
}

// SetParams
func (agentStatus AgentStatus) SetParams(params string) {
	agentStatus.EncryptedParams = EncryptString(params)
}

// SaveClusterStatus
func (c *Vrouter) SaveClusterStatus(nodeName string, clnt client.Client) error {
	if s := c.LookupAgentStatus(nodeName); s != nil {
		s.ControlNodes = c.GetControlNodes(clnt)
		s.ConfigNodes = c.GetConfigNodes(clnt)
	}
	return nil
}

// VrouterPod is a pod, created by vrouter.
type VrouterPod struct {
	Pod *corev1.Pod
}

// GetAgentContainerStatus gets the vrouteragent container status.
func (vrouterPod *VrouterPod) GetAgentContainerStatus() (*corev1.ContainerStatus, error) {
	containerStatuses := vrouterPod.Pod.Status.ContainerStatuses
	// Iterate over all pod's containers
	var agentContainerStatus corev1.ContainerStatus
	for _, containerStatus := range containerStatuses {
		if containerStatus.Name == "vrouteragent" {
			agentContainerStatus = containerStatus
		}
	}
	// Check if container was found in pod
	if &agentContainerStatus == nil {
		//log.Info("ERROR: Container vrouteragent not found in vrouteragent pod")
		return nil, fmt.Errorf("ERROR: Container vrouteragent not found for pod %v", vrouterPod.Pod)
	}

	return &agentContainerStatus, nil
}

// ExecToAgentContainer uninterractively exec to the vrouteragent container.
func (vrouterPod *VrouterPod) ExecToAgentContainer(command []string, stdin io.Reader) (string, string, error) {
	stdout, stderr, err := k8s.ExecToPodThroughAPI(command,
		"vrouteragent",
		vrouterPod.Pod.ObjectMeta.Name,
		vrouterPod.Pod.ObjectMeta.Namespace,
		stdin,
	)
	return stdout, stderr, err
}

// IsAgentRunning checks if agent running on the vrouteragent container.
func (vrouterPod *VrouterPod) IsAgentRunning() bool {
	command := []string{"/usr/bin/test", "-f", "/var/run/vrouter-agent.pid"}
	if _, _, err := vrouterPod.ExecToAgentContainer(command, nil); err != nil {
		return false
	}
	return true
}

// GetEncryptedFileFromAgentContainer gets encrypted file from vrouteragent container
func (vrouterPod *VrouterPod) GetEncryptedFileFromAgentContainer(path string) (string, error) {
	command := []string{"/usr/bin/sha1sum", path}
	stdout, _, err := vrouterPod.ExecToAgentContainer(command, nil)
	shakey := strings.Split(stdout, " ")[0]
	return shakey, err
}

// IsFileInAgentConainerEqualTo
func (vrouterPod *VrouterPod) IsFileInAgentContainerEqualTo(path string, content string) (bool, error) {
	shakey1, err := vrouterPod.GetEncryptedFileFromAgentContainer(path)
	if err != nil {
		return false, err
	}
	shakey2 := EncryptString(content)

	if shakey1 == shakey2 {
		return true, nil
	}
	return false, nil
}

// RecalculateAgentParametrs recalculates parameters for agent from `/etc/contrail/params.env` to `/parametrs.sh`
func (vrouterPod *VrouterPod) RecalculateAgentParameters() error {
	command := fmt.Sprintf("source %v/params.env; source /actions.sh; source /common.sh; source /agent-functions.sh; prepare_agent_config_vars", VrouterAgentConfigMountPath)
	_, _, err := vrouterPod.ExecToAgentContainer([]string{"/usr/bin/bash", "-c", command}, nil)
	return err
}

// GetAgentParaments gets parametrs from `/parametrs.sh`
func (vrouterPod *VrouterPod) GetAgentParameters(hostParams *map[string]string) error {
	command := []string{"/usr/bin/bash", "-c", "source /actions.sh; get_parameters"}
	stdio, _, err := vrouterPod.ExecToAgentContainer(command, nil)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(stdio))
	for scanner.Scan() {
		keyValue := strings.SplitAfterN(scanner.Text(), "=", 2)
		key := strings.TrimSuffix(keyValue[0], "=")
		value := removeQuotes(keyValue[1])
		(*hostParams)[key] = value
	}
	return nil
}

// ReloadAgentConfigs sends SIGHUP to the vrouteragent container process to reload config file.
func (vrouterPod *VrouterPod) ReloadAgentConfigs() error {
	command := []string{"/usr/bin/bash", "-c", "source /contrail-functions.sh; reload_config"}
	_, _, err := vrouterPod.ExecToAgentContainer(command, nil)
	return err
}

func removeQuotes(str string) string {
	if len(str) > 0 && str[0] == '"' {
		str = str[1:]
	}
	if len(str) > 0 && str[len(str)-1] == '"' {
		str = str[:len(str)-1]
	}
	return str
}

// GetAgentConfigsForPod returns correct values of `/etc/agentconfigmaps/config_name.{$pod_ip}` files
func (c *Vrouter) GetAgentConfigsForPod(vrouterPod *VrouterPod, hostVars *map[string]string) (agentConfig, lbaasAuthConfig, vncAPILibIniConfig, nodemgrConfig string) {
	newMap := make(map[string]string)
	for key, val := range *hostVars {
		newMap[key] = val
	}
	newMap["Hostname"] = vrouterPod.Pod.Annotations["hostname"]

	var agentConfigBuffer bytes.Buffer
	configtemplates.VRouterAgentConfig.Execute(&agentConfigBuffer, newMap)
	agentConfig = agentConfigBuffer.String()

	var lbaasAuthConfigBuffer bytes.Buffer
	configtemplates.VRouterLbaasAuthConfig.Execute(&lbaasAuthConfigBuffer, newMap)
	lbaasAuthConfig = lbaasAuthConfigBuffer.String()

	var vncAPILibIniConfigBuffer bytes.Buffer
	configtemplates.VRouterVncApiLibIni.Execute(&vncAPILibIniConfigBuffer, newMap)
	vncAPILibIniConfig = vncAPILibIniConfigBuffer.String()

	var nodemgrConfigBuffer bytes.Buffer
	configtemplates.VrouterNodemanagerConfig.Execute(&nodemgrConfigBuffer, newMap)
	nodemgrConfig = nodemgrConfigBuffer.String()

	return
}

// GetCNIConfig creates CNI plugin config
func (c *Vrouter) GetCNIConfig(client client.Client, request reconcile.Request) (string, error) {
	// TODO: it might be not good to have here this code
	cinfo, err := k8s.ClusterInfoInstance()
	if err != nil {
		return "", err
	}
	cfg, err := cinfo.ClusterParameters()
	if err != nil {
		return "", err
	}

	var useKubeadmConfig bool = KubernetesUseKubeadm
	if c.Spec.ServiceConfiguration.UseKubeadmConfig != nil {
		useKubeadmConfig = *c.Spec.ServiceConfiguration.UseKubeadmConfig
	}
	kubernetesClusterName := KubernetesClusterName
	if c.Spec.ServiceConfiguration.KubernetesClusterName != "" {
		kubernetesClusterName = c.Spec.ServiceConfiguration.KubernetesClusterName
	} else {
		if useKubeadmConfig {
			kubernetesClusterName = cfg.ClusterName
		}
	}

	var contrailCNIBuffer bytes.Buffer
	configtemplates.ContrailCNIConfig.Execute(&contrailCNIBuffer, struct {
		KubernetesClusterName string
	}{
		KubernetesClusterName: kubernetesClusterName,
	})
	return contrailCNIBuffer.String(), nil
}

// UpdateAgentConfigMapForPod recalculates files `/etc/agentconfigmaps/config_name.{$pod_ip}` in the agent configMap
func (c *Vrouter) UpdateAgentConfigMapForPod(vrouterPod *VrouterPod,
	hostVars *map[string]string,
	client client.Client,
) error {
	configMapNamespacedName := types.NamespacedName{
		Name:      c.ObjectMeta.Name + "-vrouter-agent-config",
		Namespace: c.ObjectMeta.Namespace,
	}

	configMap := &corev1.ConfigMap{}
	if err := client.Get(context.Background(), configMapNamespacedName, configMap); err != nil {
		return err
	}

	agentConfig, lbaasAuthConfig, vncAPILibIniConfig, nodemgrConfig := c.GetAgentConfigsForPod(vrouterPod, hostVars)

	podIP := vrouterPod.Pod.Status.PodIP
	data := configMap.Data
	data["contrail-vrouter-agent.conf."+podIP] = agentConfig
	data["contrail-lbaas.auth.conf."+podIP] = lbaasAuthConfig
	data["vnc_api_lib.ini."+podIP] = vncAPILibIniConfig
	data["nodemanager.conf."+podIP] = nodemgrConfig

	// Save config data
	configMap.Data = data
	err := client.Update(context.Background(), configMap)
	return err
}

// IsClusterChanged
func (c *Vrouter) IsClusterChanged(nodeName string, clnt client.Client) bool {
	if s := c.LookupAgentStatus(nodeName); s != nil {
		return s.ControlNodes != c.GetControlNodes(clnt) || s.ConfigNodes != c.GetConfigNodes(clnt)
	}
	return false
}

// UpdateAgent
func (c *Vrouter) UpdateAgent(nodeName string, vrouterPod *VrouterPod, clnt client.Client, reconsFlag *bool) error {
	eq, err := vrouterPod.IsFileInAgentContainerEqualTo(VrouterAgentConfigMountPath+"/params.env", c.GetParamsEnv(clnt))
	if err != nil || !eq {
		*reconsFlag = true
		return err
	}

	if err := vrouterPod.RecalculateAgentParameters(); err != nil {
		*reconsFlag = true
		return err
	}

	hostVars := make(map[string]string)
	if err := vrouterPod.GetAgentParameters(&hostVars); err != nil {
		*reconsFlag = true
		return err
	}

	if err := c.UpdateAgentConfigMapForPod(vrouterPod, &hostVars, clnt); err != nil {
		*reconsFlag = true
		return err
	}

	eq, err = vrouterPod.IsAgentConfigsAvaliable(c, &hostVars, clnt)
	if err != nil || !eq {
		*reconsFlag = true
		return err
	}

	if err := c.SaveClusterStatus(nodeName, clnt); err != nil {
		*reconsFlag = true
		return err
	}

	agentStatus := c.LookupAgentStatus(nodeName)
	if agentStatus != nil {
		agentStatus.EncryptedParams = EncryptString(c.GetParamsEnv(clnt))
	}

	// Send SIGHUP то container process to reload config file
	if err = vrouterPod.ReloadAgentConfigs(); err != nil {
		*reconsFlag = true
		return err
	}

	if agentStatus != nil {
		agentStatus.Status = "Ready"
	}

	return nil
}

// IsAgentConfigsAvaliable
func (vrouterPod *VrouterPod) IsAgentConfigsAvaliable(vrouter *Vrouter, hostVars *map[string]string, clnt client.Client) (bool, error) {
	podIP := vrouterPod.Pod.Status.PodIP

	agentConfig, lbaasAuthConfig, vncAPILibIniConfig, nodemgrConfig := vrouter.GetAgentConfigsForPod(vrouterPod, hostVars)

	path := VrouterAgentConfigMountPath + "/contrail-vrouter-agent.conf." + podIP
	eq, err := vrouterPod.IsFileInAgentContainerEqualTo(path, agentConfig)
	if err != nil || !eq {
		return eq, err
	}

	path = VrouterAgentConfigMountPath + "/contrail-lbaas.auth.conf." + podIP
	eq, err = vrouterPod.IsFileInAgentContainerEqualTo(path, lbaasAuthConfig)
	if err != nil || !eq {
		return eq, err
	}

	path = VrouterAgentConfigMountPath + "/vnc_api_lib.ini." + podIP
	eq, err = vrouterPod.IsFileInAgentContainerEqualTo(path, vncAPILibIniConfig)
	if err != nil || !eq {
		return eq, nil
	}

	path = VrouterAgentConfigMountPath + "/nodemanager.conf." + podIP
	eq, err = vrouterPod.IsFileInAgentContainerEqualTo(path, nodemgrConfig)
	if err != nil || !eq {
		return eq, nil
	}

	return true, nil
}

// isParamsEnvEqual
func (c *Vrouter) isParamsEnvEqual(clnt client.Client, vrouterPod *VrouterPod) (bool, error) {
	shakey1, err := vrouterPod.GetEncryptedFileFromAgentContainer(VrouterAgentConfigMountPath + "/params.env")
	if err != nil {
		return false, err
	}

	shakey2 := EncryptString(c.GetParamsEnv(clnt))

	if shakey1 == shakey2 {
		return true, nil
	}
	return false, nil
}

func (c *Vrouter) IsActiveOnControllers(clnt client.Client) (bool, error) {
	if c.Status.Agents == nil {
		return false, nil
	}
	controllerNodes := &corev1.NodeList{}
	labels := client.MatchingLabels{"node-role.kubernetes.io/master": ""}
	if err := clnt.List(context.Background(), controllerNodes, labels); err != nil {
		return false, err
	}
	for _, node := range controllerNodes.Items {
		if s := c.LookupAgentStatus(node.Name); s != nil && s.Status != "Ready" {
			return false, nil
		}
	}
	return true, nil
}

func (c *Vrouter) LookupAgentStatus(name string) *AgentStatus {
	if c.Status.Agents == nil {
		return nil
	}
	for _, s := range c.Status.Agents {
		if s.Name == name {
			return s
		}
	}
	return nil
}
