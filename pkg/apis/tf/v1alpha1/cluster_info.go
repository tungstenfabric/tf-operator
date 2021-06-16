package v1alpha1

import (
	"fmt"
	"net"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesClusterConfig k8s cluster parameters
// +k8s:openapi-gen=true
type KubernetesClusterConfig struct {
	ControlPlaneEndpoint string                      `json:"controlPlaneEndpoint,omitempty"`
	ClusterName          string                      `json:"clusterName,omitempty"`
	Networking           KubernetesClusterNetworking `json:"networking,omitempty"`
}

// KubernetesClusterNetworking k8s cluster networking parameters
// +k8s:openapi-gen=true
type KubernetesClusterNetworking struct {
	DNSDomain     string    `json:"dnsDomain,omitempty"`
	PodSubnet     string    `json:"podSubnet,omitempty"`
	ServiceSubnet string    `json:"serviceSubnet,omitempty"`
	CNIConfig     CNIConfig `json:"cniConfig,omitempty"`
}

// CNIConfig k8s cluster cni parameters
// +k8s:openapi-gen=true
type CNIConfig struct {
	ConfigPath string `json:"configPath,omitempty"`
	BinaryPath string `json:"binaryPath,omitempty"`
}

var clusterInfoLog = logf.Log.WithName("cluster_info")

// ClusterDNSDomain returns Cluster DNS domain set in kubeadm config
func ClusterDNSDomain(client client.Client) (string, error) {
	cfg, err := ClusterParameters(client)
	if err != nil {
		return "", err
	}
	return cfg.Networking.DNSDomain, nil
}

func yamlToStruct(yamlString string, structPointer interface{}) error {
	jsonData, err := yaml.YAMLToJSON([]byte(yamlString))
	if err != nil {
		clusterInfoLog.Error(err, "YAMLToJSON failed", "jsonData", jsonData)
		return err
	}

	if err = yaml.Unmarshal([]byte(jsonData), structPointer); err != nil {
		clusterInfoLog.Error(err, "Unmarshal failed", "jsonData", jsonData)
		return err
	}

	return nil
}

var getConfigMapFromOtherNamespace = func(name string, namespace string) (*v1.ConfigMap, error) {
	// getConfigMapFromOtherNamespace use requests to k8s api. Thats why we
	// create it as a variable to have an ability to mock it in the unit tests.
	configMap, err := k8s.GetCoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			clusterInfoLog.Error(err, fmt.Sprintf("CoreV1.ConfigMaps failed to get %v", name))
		}
		return nil, err
	}

	return configMap, nil
}

func (to *KubernetesClusterConfig) replaceFields(from KubernetesClusterConfig) *KubernetesClusterConfig {
	if from.ClusterName != "" {
		to.ClusterName = from.ClusterName
	}
	if from.Networking.DNSDomain != "" {
		to.Networking.DNSDomain = from.Networking.DNSDomain
	}
	if from.Networking.PodSubnet != "" {
		to.Networking.PodSubnet = from.Networking.PodSubnet
	}
	if from.Networking.ServiceSubnet != "" {
		to.Networking.ServiceSubnet = from.Networking.ServiceSubnet
	}
	if from.Networking.CNIConfig.ConfigPath != "" {
		to.Networking.CNIConfig.ConfigPath = from.Networking.CNIConfig.ConfigPath
	}
	if from.Networking.CNIConfig.BinaryPath != "" {
		to.Networking.CNIConfig.BinaryPath = from.Networking.CNIConfig.BinaryPath
	}

	if from.ControlPlaneEndpoint != "" {
		to.ControlPlaneEndpoint = from.ControlPlaneEndpoint
	}

	return to
}

func (c *KubernetesClusterConfig) fillWithDefaultValues() {
	if !IsOpenshift() {
		c.ClusterName = KubernetesClusterName
		c.Networking.DNSDomain = KubernetesDNSDomainName
		c.Networking.PodSubnet = KubernetesPodSubnet
		c.Networking.ServiceSubnet = KubernetesServiceSubnet
		c.Networking.CNIConfig.ConfigPath = CNIConfigPath
		c.Networking.CNIConfig.BinaryPath = CNIBinaryPath
	} else {
		c.ClusterName = OpenShiftClusterName
		c.Networking.DNSDomain = OpenShiftDNSDomain
		c.Networking.PodSubnet = OpenShiftPodSubnet
		c.Networking.ServiceSubnet = OpenShiftServiceSubnet
		c.Networking.CNIConfig.ConfigPath = OpenShiftCNIConfigPath
		c.Networking.CNIConfig.BinaryPath = OpenShiftCNIBinaryPath
	}
}

func (c *KubernetesClusterConfig) fillWithKubeadmConfigMap() error {

	kubeadmConfigMap, err := getConfigMapFromOtherNamespace("kubeadm-config", "kube-system")
	if err != nil {
		// Which config map is used for containing parameters depends on method of deployment.
		// So, there is only one system config map at a time: kubeadm or cluster.
		// Thats why it's ok to be not found.
		if errors.IsNotFound(err) {
			return nil
		}
		clusterInfoLog.Error(err, "Failed to fill with kubeadm config map.")
		return err
	}

	yamlString := kubeadmConfigMap.Data["ClusterConfiguration"]
	if err = yamlToStruct(yamlString, c); err != nil {
		clusterInfoLog.Error(err, "Failed to yamlToStruct for kubeadmConfig ClusterConfiguration")
	}

	if c.ControlPlaneEndpoint != "" {
		return nil
	}

	var clusterStatusStruct struct {
		ApiEndpoints map[string]struct {
			AdvertiseAddress string
			BindPort         *int
		}
	}

	yamlString = kubeadmConfigMap.Data["ClusterStatus"]
	if err = yamlToStruct(yamlString, &clusterStatusStruct); err != nil {
		clusterInfoLog.Error(err, "Failed to yamlToStruct for kubeadmConfig CluserStatus")
		return err
	}

	if clusterStatusStruct.ApiEndpoints == nil {
		return nil
	}

	for _, val := range clusterStatusStruct.ApiEndpoints {
		if val.AdvertiseAddress != "" && val.BindPort != nil {
			c.ControlPlaneEndpoint = fmt.Sprintf("%v:%v", val.AdvertiseAddress, *val.BindPort)
			return nil
		}
	}
	return nil
}

func (c *KubernetesClusterConfig) fillWithClusterConfigMap() error {
	clusterConfigMap, err := getConfigMapFromOtherNamespace("cluster-config-v1", "kube-system")
	if err != nil {
		// Which config map is used for containing parameters depends on method of deployment.
		// So, there is only one system config map at a time: kubeadm or cluster.
		// Thats why it's ok to be not found.
		if errors.IsNotFound(err) {
			return nil
		}
		clusterInfoLog.Error(err, "Failed to fill with cluster config map.")
		return err
	}

	yamlString := clusterConfigMap.Data["install-config"]
	var clusterConfigStruct struct {
		BaseDomain string
		Metadata   struct {
			Name string
		}
		Networking struct {
			ClusterNetwork []struct {
				Cidr       string
				HostPrefix string
			}
			ServiceNetwork []string
		}
	}
	if err := yamlToStruct(yamlString, &clusterConfigStruct); err != nil {
		return err
	}

	c.ClusterName = clusterConfigStruct.Metadata.Name
	c.Networking.DNSDomain = clusterConfigStruct.BaseDomain
	if len(clusterConfigStruct.Networking.ClusterNetwork) > 0 {
		c.Networking.PodSubnet = clusterConfigStruct.Networking.ClusterNetwork[0].Cidr
	}
	if len(clusterConfigStruct.Networking.ServiceNetwork) > 0 {
		c.Networking.ServiceSubnet = clusterConfigStruct.Networking.ServiceNetwork[0]
	}

	return nil
}

// ClusterParameters returns cluster configuration, merged from manager configuration and system configmaps
func ClusterParameters(client client.Client) (*KubernetesClusterConfig, error) {
	var defaultConfig, kubeadmConfig, clusterConfig KubernetesClusterConfig

	defaultConfig.fillWithDefaultValues()

	if err := kubeadmConfig.fillWithKubeadmConfigMap(); err != nil {
		return nil, err
	}

	if err := clusterConfig.fillWithClusterConfigMap(); err != nil {
		return nil, err
	}


	resultConfig := defaultConfig.replaceFields(kubeadmConfig).
		replaceFields(clusterConfig)

	if resultConfig.ControlPlaneEndpoint == "" {
		resultConfig.ControlPlaneEndpoint = fmt.Sprintf("api.%v.%v:%v",
			resultConfig.ClusterName,
			resultConfig.Networking.DNSDomain,
			strconv.Itoa(KubernetesApiSSLPort),
		)
	}

	return resultConfig, nil
}

// KubernetesAPISSLPort gathers SSL Port from Kubernetes Cluster via kubeadm-config ConfigMap
func (c *KubernetesClusterConfig) KubernetesAPISSLPort() (int, error) {
	controlPlaneEndpoint := c.ControlPlaneEndpoint
	_, kubernetesAPISSLPort, err := net.SplitHostPort(controlPlaneEndpoint)
	if err != nil {
		return 0, err
	}
	kubernetesAPISSLPortInt, err := strconv.Atoi(kubernetesAPISSLPort)
	if err != nil {
		return 0, err
	}
	return kubernetesAPISSLPortInt, nil
}

// KubernetesAPIServer gathers API Server from Kubernetes Cluster via kubeadm-config ConfigMap
func (c *KubernetesClusterConfig) KubernetesAPIServer() (string, error) {
	controlPlaneEndpoint := c.ControlPlaneEndpoint
	kubernetesAPIServer, _, err := net.SplitHostPort(controlPlaneEndpoint)
	if err != nil {
		return "", err
	}
	return kubernetesAPIServer, nil
}
