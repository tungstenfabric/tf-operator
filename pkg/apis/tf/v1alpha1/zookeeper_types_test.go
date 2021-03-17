package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

var zookeeperRequest = reconcile.Request{
	NamespacedName: types.NamespacedName{
		Name:      "zookeeper1",
		Namespace: "test-ns",
	},
}

var zookeeperCM = &corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "zookeeper1-zookeeper-configmap",
		Namespace: "test-ns",
	},
}

func TestZookeeperConfigMapWithDefaultValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme, zookeeperCM)
	zookeeper := Zookeeper{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zookeeper1",
			Namespace: "test-ns",
		},
	}
	_ = zookeeper.InstanceConfiguration(zookeeperRequest, "zookeeper1-zookeeper-configmap", zookeeperPodList, cl)

	var zookeeperConfigMap = &corev1.ConfigMap{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "zookeeper1-zookeeper-configmap", Namespace: "test-ns"}, zookeeperConfigMap), "Error while gathering zookeeper config map")

	zookeeperConfig, err := ini.Load([]byte(zookeeperConfigMap.Data["log4j.properties"]))
	require.NoError(t, err)

	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.root.logger").String())
	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.console.threshold").String())
	assert.Equal(t, "INFO", zookeeperConfig.Section("").Key("zookeeper.log.threshold").String())
}

func TestZookeeperConfigMapWithCustomValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme, zookeeperCM)
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
	_ = zookeeper.InstanceConfiguration(zookeeperRequest, "zookeeper1-zookeeper-configmap", zookeeperPodList, cl)

	var zookeeperConfigMap = &corev1.ConfigMap{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "zookeeper1-zookeeper-configmap", Namespace: "test-ns"}, zookeeperConfigMap), "Error while gathering zookeeper config map")

	zookeeperConfig, err := ini.Load([]byte(zookeeperConfigMap.Data["log4j.properties"]))
	require.NoError(t, err)

	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.root.logger").String())
	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.console.threshold").String())
	assert.Equal(t, "DEBUG", zookeeperConfig.Section("").Key("zookeeper.log.threshold").String())
}
