package k8s

import (
	"net"
	"strconv"

	"github.com/ghodss/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedCorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// ClusterInfo interface to get cluster info form kubernetes api
type ClusterInfo struct {
	Client typedCorev1.CoreV1Interface
}

// ClusterInfoInstance returns ClusterInfo object
func ClusterInfoInstance() (*ClusterInfo, error) {
	clientset, err := GetClientset()
	if err != nil {
		return nil, err
	}
	return &ClusterInfo{Client: clientset.CoreV1()}, nil
}

// KubernetesClusterConfig k8s cluster parameters
type KubernetesClusterConfig struct {
	ControlPlaneEndpoint string                      `json:"controlPlaneEndpoint,omitempty"`
	ClusterName          string                      `json:"clusterName,omitempty"`
	Networking           KubernetesClusterNetworking `json:"networking,omitempty"`
}

// KubernetesClusterNetworking k8s cluster networking parameters
type KubernetesClusterNetworking struct {
	DNSDomain     string `json:"dnsDomain,omitempty"`
	PodSubnet     string `json:"podSubnet,omitempty"`
	ServiceSubnet string `json:"serviceSubnet,omitempty"`
}

// ClusterDNSDomain returns Cluster DNS domain set in kubeadm config
func ClusterDNSDomain() (string, error) {
	cinfo, err := ClusterInfoInstance()
	if err != nil {
		return "", err
	}
	cfg, err := cinfo.ClusterParameters()
	if err != nil {
		return "", err
	}
	return cfg.Networking.DNSDomain, nil
}

// ClusterParameters requsts kubeadm cluster configmap
func (c *ClusterInfo) ClusterParameters() (*KubernetesClusterConfig, error) {
	kcm, err := c.Client.ConfigMaps("kube-system").Get("kubeadm-config", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	jsonData, err := yaml.YAMLToJSON([]byte(kcm.Data["ClusterConfiguration"]))
	if err != nil {
		return nil, err
	}
	clusterConfigMap := KubernetesClusterConfig{}
	err = yaml.Unmarshal([]byte(jsonData), &clusterConfigMap)
	return &clusterConfigMap, err
}

// KubernetesAPISSLPort gathers SSL Port from Kubernetes Cluster via kubeadm-config ConfigMap
func (c KubernetesClusterConfig) KubernetesAPISSLPort() (int, error) {
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
func (c KubernetesClusterConfig) KubernetesAPIServer() (string, error) {
	controlPlaneEndpoint := c.ControlPlaneEndpoint
	kubernetesAPIServer, _, err := net.SplitHostPort(controlPlaneEndpoint)
	if err != nil {
		return "", err
	}
	return kubernetesAPIServer, nil
}
