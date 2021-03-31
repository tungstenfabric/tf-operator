package v1alpha1

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var clusterInfoTestKubeadmConfigMap = corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "kubeadm-config",
		Namespace: "kube-system",
	},
	Data: map[string]string{
		"ClusterConfiguration": `apiServer:
  extraArgs:
    authorization-mode: Node,RBAC
  timeoutForControlPlane: 4m0s
apiVersion: kubeadm.k8s.io/v1beta2
certificatesDir: /etc/kubernetes/pki
clusterName: kubernetes
controllerManager: {}
dns:
  type: CoreDNS
etcd:
  local:
    dataDir: /var/lib/etcd
imageRepository: k8s.gcr.io
kind: ClusterConfiguration
kubernetesVersion: v1.20.4
networking:
  dnsDomain: cluster.local
  serviceSubnet: 10.96.0.0/12
scheduler: {}`,
		"ClusterStatus": `apiEndpoints:
  k8s-aio:
    advertiseAddress: 172.16.125.120
    bindPort: 6443
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterStatus`,
	},
}

var clusterInfoTestClusterConfigConfigMap = corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cluster-config-v1",
		Namespace: "kube-system",
	},
	Data: map[string]string{
		"install-config": `apiVersion: v1
baseDomain: test.example.com
compute:
- hyperthreading: Enabled
  name: worker
  platform: {}
  replicas: 0
controlPlane:
  hyperthreading: Enabled
  name: master
  platform: {}
  replicas: 3
metadata:
  creationTimestamp: null
  name: test
networking:
  clusterNetwork:
  - cidr: 10.128.0.0/14
    hostPrefix: 23
  machineCIDR: 10.0.0.0/16
  networkType: OpenShiftSDN
  serviceNetwork:
  - 172.30.0.0/16
platform:
  vsphere:
    datacenter: prod-dc
    defaultDatastore: Datastore
    password: ""
    username: ""
    vCenter: vmware.example.com
pullSecret: ""
sshKey: ssh-rsa`,
	},
}

var clusterInfoTestManager = Manager{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cluster-info-test-manager",
		Namespace: "cluster-info-test-namespace",
	},
	Spec: ManagerSpec{
		CommonConfiguration: ManagerConfiguration{
			ClusterConfig: &KubernetesClusterConfig{
				ClusterName: "cluster-name-from-manager",
				Networking: KubernetesClusterNetworking{
					CNIConfig: CNIConfig{
						BinaryPath: "cni-binary-path-from-manager",
					},
				},
			},
		},
	},
}

func TestClusterParametersKubeadmExists(t *testing.T) {
	// Mock functions using k8s api requests
	oldConfigMapFromOtherNamespace := getConfigMapFromOtherNamespace
	getConfigMapFromOtherNamespace = func(name string, namespace string) (*corev1.ConfigMap, error) {
		if name == "kubeadm-config" && namespace == "kube-system" {
			return &clusterInfoTestKubeadmConfigMap, nil
		} else {
			return nil, errors.NewNotFound(schema.GroupResource{Group: "corev1", Resource: "configmap"}, name)
		}
	}
	defer func() { getConfigMapFromOtherNamespace = oldConfigMapFromOtherNamespace }()

	// Create fake schema, client and our objects
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	client := fake.NewFakeClientWithScheme(scheme, &clusterInfoTestManager)

	// Test
	clusterParameters, err := ClusterParameters(client)
	require.NoError(t, err, "ClusterParameters ends with error")

	// Fields from manager
	assert.Equal(t, "cluster-name-from-manager", clusterParameters.ClusterName)
	assert.Equal(t, "cni-binary-path-from-manager", clusterParameters.Networking.CNIConfig.BinaryPath)
	// Fields from kubeadm-config
	assert.Equal(t, "cluster.local", clusterParameters.Networking.DNSDomain)
	assert.Equal(t, "10.96.0.0/12", clusterParameters.Networking.ServiceSubnet)
	assert.Equal(t, "172.16.125.120:"+strconv.Itoa(KubernetesApiSSLPort), clusterParameters.ControlPlaneEndpoint)
	// Default fields
	assert.Equal(t, KubernetesPodSubnet, clusterParameters.Networking.PodSubnet)
	assert.Equal(t, CNIConfigPath, clusterParameters.Networking.CNIConfig.ConfigPath)

	// TEST endpoints priority:
	// ClusterConfiguration.ControlPlaneEndpoint > ClusterStatus.ApiEndpoints
	oldKubeadmDataClusterConfiguration := clusterInfoTestKubeadmConfigMap.Data["ClusterConfiguration"]
	defer func() {
		clusterInfoTestKubeadmConfigMap.Data["ClusterConfiguration"] = oldKubeadmDataClusterConfiguration
	}()

	clusterInfoTestKubeadmConfigMap.Data["ClusterConfiguration"] = `controlPlaneEndpoint: my-control-plane-endpoint`
	clusterParameters, err = ClusterParameters(client)
	require.NoError(t, err, "ClusterParameters for Endpoints Priority ends with error")

	assert.Equal(t, "my-control-plane-endpoint", clusterParameters.ControlPlaneEndpoint)
}

func TestClusterParametersClusterConfigExists(t *testing.T) {
	// Mock functions using k8s api requests
	oldConfigMapFromOtherNamespace := getConfigMapFromOtherNamespace
	getConfigMapFromOtherNamespace = func(name string, namespace string) (*corev1.ConfigMap, error) {
		if name == "cluster-config-v1" && namespace == "kube-system" {
			return &clusterInfoTestClusterConfigConfigMap, nil
		} else {
			return nil, errors.NewNotFound(schema.GroupResource{Group: "corev1", Resource: "configmap"}, name)
		}
	}
	defer func() { getConfigMapFromOtherNamespace = oldConfigMapFromOtherNamespace }()

	// Create fake schema, client and our objects
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	client := fake.NewFakeClientWithScheme(scheme, &clusterInfoTestManager)

	// Test
	clusterParameters, err := ClusterParameters(client)
	require.NoError(t, err, "ClusterParameters ends with error")

	// Fields from manager
	assert.Equal(t, "cluster-name-from-manager", clusterParameters.ClusterName)
	assert.Equal(t, "cni-binary-path-from-manager", clusterParameters.Networking.CNIConfig.BinaryPath)
	// Fields from kubeadm-config
	assert.Equal(t, "test.example.com", clusterParameters.Networking.DNSDomain)
	assert.Equal(t, "172.30.0.0/16", clusterParameters.Networking.ServiceSubnet)
	assert.Equal(t, "10.128.0.0/14", clusterParameters.Networking.PodSubnet)
	// Default fields
	assert.Equal(t, CNIConfigPath, clusterParameters.Networking.CNIConfig.ConfigPath)

	assert.Equal(t, "api.cluster-name-from-manager.test.example.com:"+strconv.Itoa(KubernetesApiSSLPort), clusterParameters.ControlPlaneEndpoint)
}
