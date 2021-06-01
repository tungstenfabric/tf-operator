package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var zookeeperPodList = []corev1.Pod{
	{
		Status: corev1.PodStatus{PodIP: "1.1.1.1"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod1",
			Annotations: map[string]string{
				"hostname": "pod1-host",
			},
		},
	},
	{
		Status: corev1.PodStatus{PodIP: "2.2.2.2"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod2",
			Annotations: map[string]string{
				"hostname": "pod2-host",
			},
		},
	},
}

func TestZookeeperConfigMapWithDefaultValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme)
	zookeeper := Zookeeper{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zookeeper1",
			Namespace: "test-ns",
		},
	}
	data, err := zookeeper.InstanceConfiguration(zookeeperPodList, cl)
	require.NoError(t, err)

	zookeeperConfig, err := ini.Load([]byte(data["log4j.properties"]))
	require.NoError(t, err)

	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.root.logger").String())
	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.console.threshold").String())
	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.log.threshold").String())
}

func TestZookeeperConfigMapWithCustomValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme)
	zookeeper := Zookeeper{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zookeeper1",
			Namespace: "test-ns",
		},
		Spec: ZookeeperSpec{
			CommonConfiguration: PodConfiguration{
				LogLevel: "debug",
			},
		},
	}
	data, err := zookeeper.InstanceConfiguration(zookeeperPodList, cl)
	require.NoError(t, err)

	zookeeperConfig, err := ini.Load([]byte(data["log4j.properties"]))
	require.NoError(t, err)

	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.root.logger").String())
	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.console.threshold").String())
	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.log.threshold").String())
}
