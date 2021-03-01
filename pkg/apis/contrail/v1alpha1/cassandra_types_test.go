package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var cassandraPodList = []corev1.Pod{
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

var cassandraRequest = reconcile.Request{
	NamespacedName: types.NamespacedName{
		Name:      "cassandra1",
		Namespace: "test-ns",
	},
}

var cassandraCM = &corev1.ConfigMap{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cassandra1-cassandra-configmap",
		Namespace: "test-ns",
	},
}

var cassandraSecret = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "cassandra1-secret",
		Namespace: "test-ns",
	},
	Data: map[string][]byte{
		"keystorePassword":   []byte("test_keystone_pass"),
		"truststorePassword": []byte("test_truestore_pass"),
	},
}

var config = &Config{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "config1",
		Namespace: "test-ns",
	},
	Spec: ConfigSpec{
		ServiceConfiguration: ConfigConfiguration{},
	},
}

type paramsStruct struct {
	ConcurrentReads                  int    `yaml:"concurrent_reads"`
	ConcurrentWrites                 int    `yaml:"concurrent_writes"`
	ConcurrentCounterWrites          int    `yaml:"concurrent_counter_writes"`
	ConcurrentMaterializedViewWrites int    `yaml:"concurrent_materialized_view_writes"`
	ConcurrentCompactors             int    `yaml:"concurrent_compactors"`
	MemtableFlushWriters             int    `yaml:"memtable_flush_writers"`
	MemtableAllocationType           string `yaml:"memtable_allocation_type"`
	CompactionThroughputMbPerSec     int    `yaml:"compaction_throughput_mb_per_sec"`
}

func TestCassandraConfigMapsWithDefaultValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme, cassandraCM, cassandraSecret, config)
	cassandra := Cassandra{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cassandra1",
			Namespace: "test-ns",
		},
		Spec: CassandraSpec{
			ServiceConfiguration: CassandraConfiguration{
				ConfigInstance: "config1",
			},
		},
	}

	require.NoError(t, cassandra.InstanceConfiguration(cassandraRequest, cassandraPodList, cl))

	var cassandraConfigMap = &corev1.ConfigMap{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "cassandra1-cassandra-configmap", Namespace: "test-ns"}, cassandraConfigMap), "Error while gathering cassandra config map")

	var cassandraConfig paramsStruct
	err = yaml.Unmarshal([]byte(cassandraConfigMap.Data["cassandra.1.1.1.1.yaml"]), &cassandraConfig)
	require.NoError(t, err)

	assert.Equal(t, 16, cassandraConfig.CompactionThroughputMbPerSec)
	assert.Equal(t, 32, cassandraConfig.ConcurrentReads)
	assert.Equal(t, 32, cassandraConfig.ConcurrentWrites)
	assert.Equal(t, "heap_buffers", cassandraConfig.MemtableAllocationType)
	assert.Equal(t, 1, cassandraConfig.ConcurrentCompactors)
	assert.Equal(t, 2, cassandraConfig.MemtableFlushWriters)
	assert.Equal(t, 32, cassandraConfig.ConcurrentCounterWrites)
	assert.Equal(t, 32, cassandraConfig.ConcurrentMaterializedViewWrites)

	err = yaml.Unmarshal([]byte(cassandraConfigMap.Data["cassandra.2.2.2.2.yaml"]), &cassandraConfig)
	require.NoError(t, err)

	assert.Equal(t, 16, cassandraConfig.CompactionThroughputMbPerSec)
	assert.Equal(t, 32, cassandraConfig.ConcurrentReads)
	assert.Equal(t, 32, cassandraConfig.ConcurrentWrites)
	assert.Equal(t, "heap_buffers", cassandraConfig.MemtableAllocationType)
	assert.Equal(t, 1, cassandraConfig.ConcurrentCompactors)
	assert.Equal(t, 2, cassandraConfig.MemtableFlushWriters)
	assert.Equal(t, 32, cassandraConfig.ConcurrentCounterWrites)
	assert.Equal(t, 32, cassandraConfig.ConcurrentMaterializedViewWrites)
}

func TestCassandraConfigMapsWithCustomValues(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme, cassandraCM, cassandraSecret, config)
	cassandra := Cassandra{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cassandra1",
			Namespace: "test-ns",
		},
		Spec: CassandraSpec{
			ServiceConfiguration: CassandraConfiguration{
				ConfigInstance: "config1",
				CassandraParameters: CassandraConfigParameters{
					CompactionThroughputMbPerSec:     22,
					ConcurrentReads:                  33,
					ConcurrentWrites:                 44,
					MemtableAllocationType:           "offheap_buffers",
					ConcurrentCompactors:             55,
					MemtableFlushWriters:             66,
					ConcurrentCounterWrites:          77,
					ConcurrentMaterializedViewWrites: 88,
				},
			},
		},
	}

	require.NoError(t, cassandra.InstanceConfiguration(cassandraRequest, cassandraPodList, cl))

	var cassandraConfigMap = &corev1.ConfigMap{}
	require.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: "cassandra1-cassandra-configmap", Namespace: "test-ns"}, cassandraConfigMap), "Error while gathering cassandra config map")

	var cassandraConfig paramsStruct
	err = yaml.Unmarshal([]byte(cassandraConfigMap.Data["cassandra.1.1.1.1.yaml"]), &cassandraConfig)
	require.NoError(t, err)

	assert.Equal(t, 22, cassandraConfig.CompactionThroughputMbPerSec)
	assert.Equal(t, 33, cassandraConfig.ConcurrentReads)
	assert.Equal(t, 44, cassandraConfig.ConcurrentWrites)
	assert.Equal(t, "offheap_buffers", cassandraConfig.MemtableAllocationType)
	assert.Equal(t, 55, cassandraConfig.ConcurrentCompactors)
	assert.Equal(t, 66, cassandraConfig.MemtableFlushWriters)
	assert.Equal(t, 77, cassandraConfig.ConcurrentCounterWrites)
	assert.Equal(t, 88, cassandraConfig.ConcurrentMaterializedViewWrites)

	err = yaml.Unmarshal([]byte(cassandraConfigMap.Data["cassandra.2.2.2.2.yaml"]), &cassandraConfig)
	require.NoError(t, err)

	assert.Equal(t, 22, cassandraConfig.CompactionThroughputMbPerSec)
	assert.Equal(t, 33, cassandraConfig.ConcurrentReads)
	assert.Equal(t, 44, cassandraConfig.ConcurrentWrites)
	assert.Equal(t, "offheap_buffers", cassandraConfig.MemtableAllocationType)
	assert.Equal(t, 55, cassandraConfig.ConcurrentCompactors)
	assert.Equal(t, 66, cassandraConfig.MemtableFlushWriters)
	assert.Equal(t, 77, cassandraConfig.ConcurrentCounterWrites)
	assert.Equal(t, 88, cassandraConfig.ConcurrentMaterializedViewWrites)
}
