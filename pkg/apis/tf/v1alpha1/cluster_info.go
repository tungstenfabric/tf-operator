package v1alpha1

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// TODO: rework, as cluster info is a kind of info be available overall components
// get Manager CommonConfig data from cluster
func getManager(client client.Client) (Manager, error) {
	var mngr Manager
	// Get manager manifest
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    "ManagerList",
		Version: "v1alpha1",
	})
	err := client.List(context.TODO(), ul)
	if err != nil {
		clusterInfoLog.Info(fmt.Sprintf("Error when try get manager list %v", err))
		return mngr, err
	}
	managerName := ul.Items[0].GetName()
	managerNamespace := ul.Items[0].GetNamespace()

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    "Manager",
		Version: "v1alpha1",
	})
	err = client.Get(context.TODO(), types.NamespacedName{Name: managerName, Namespace: managerNamespace}, u)
	if err != nil {
		clusterInfoLog.Info(fmt.Sprintf("Error when try get manager list %v", err))
		return mngr, err
	}
	// This is local Manager defined in this file above
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &mngr)
	if err != nil {
		clusterInfoLog.Info(fmt.Sprintf("Error DefaultUnstructuredConverter %v", err))
		return mngr, err
	}
	return mngr, err
}

// ClusterParameters get kube params from manager config or requsts kubeadm cluster configmap
func ClusterParameters(client client.Client) (*KubernetesClusterConfig, error) {

	mngr, err := getManager(client)
	if err != nil {
		clusterInfoLog.Info(fmt.Sprintf("Error DefaultUnstructuredConverter %v", err))
		return nil, err
	}
	useKubeadmConfig := KubernetesUseKubeadm
	if mngr.Spec.CommonConfiguration.UseKubeadmConfig != nil {
		useKubeadmConfig = *mngr.Spec.CommonConfiguration.UseKubeadmConfig
	}

	clusterConfigMap := KubernetesClusterConfig{}
	if useKubeadmConfig {
		// ===
		// TODO: by some reason client from reconcile cannot read othe namespaces, so get common client
		// kcm := &corev1.ConfigMap{}
		// err := client.Get(context.TODO(), types.NamespacedName{Name: "kubeadm-config", Namespace: "kube-system"}, kcm)
		config, err := k8s.GetClientConfig()
		if err != nil {
			clusterInfoLog.Error(err, "GetClientConfig failed")
			return nil, err
		}
		clientset, err := k8s.GetClientsetFromConfig(config)
		if err != nil {
			clusterInfoLog.Error(err, "GetClientsetFromConfig failed")
			return nil, err
		}
		kcm, err := clientset.CoreV1().ConfigMaps("kube-system").Get("kubeadm-config", metav1.GetOptions{})
		if err != nil {
			clusterInfoLog.Error(err, "CoreV1.ConfigMaps failed to get kubeadm-config")
			return nil, err
		}
		// ====

		jsonData, err := yaml.YAMLToJSON([]byte(kcm.Data["ClusterConfiguration"]))
		if err != nil {
			clusterInfoLog.Error(err, "YAMLToJSON failed", "jsonData", jsonData)
			return nil, err
		}
		err = yaml.Unmarshal([]byte(jsonData), &clusterConfigMap)
		if err != nil {
			clusterInfoLog.Error(err, "Unmarshal failed", "jsonData", jsonData)
			return nil, err
		}
		// TODO: for now use default paths for k8s
		clusterConfigMap.Networking.CNIConfig = CNIConfig{ConfigPath: CNIConfigPath, BinaryPath: CNIBinaryPath}
		return &clusterConfigMap, err
	}

	if mngr.Spec.CommonConfiguration.ClusterConfig != nil {
		clusterConfigMap = *mngr.Spec.CommonConfiguration.ClusterConfig
	}
	if clusterConfigMap.ClusterName == "" {
		clusterConfigMap.ClusterName = KubernetesClusterName
	}
	if clusterConfigMap.Networking.DNSDomain == "" {
		clusterConfigMap.Networking.DNSDomain = KubernetesClusterName
	}
	if clusterConfigMap.Networking.PodSubnet == "" {
		clusterConfigMap.Networking.PodSubnet = KubernetesPodSubnet
	}
	if clusterConfigMap.Networking.ServiceSubnet == "" {
		clusterConfigMap.Networking.ServiceSubnet = KubernetesServiceSubnet
	}
	if clusterConfigMap.Networking.CNIConfig.ConfigPath == "" {
		clusterConfigMap.Networking.CNIConfig.ConfigPath = CNIConfigPath
	}
	if clusterConfigMap.Networking.CNIConfig.BinaryPath == "" {
		clusterConfigMap.Networking.CNIConfig.BinaryPath = CNIBinaryPath
	}
	if clusterConfigMap.ControlPlaneEndpoint == "" {
		clusterConfigMap.ControlPlaneEndpoint = "api." + clusterConfigMap.ClusterName + "." +
			clusterConfigMap.Networking.DNSDomain + ":" + strconv.Itoa(KubernetesApiSSLPort)
	}
	return &clusterConfigMap, err
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
