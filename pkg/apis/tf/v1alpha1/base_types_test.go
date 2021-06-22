package v1alpha1

import (
	"os"
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func getCassandras(names []string) []*Cassandra {
	var res []*Cassandra
	for _, n := range names {
		res = append(res, &Cassandra{
			ObjectMeta: metav1.ObjectMeta{
				Name:      n,
				Namespace: "tf",
			},
		})
	}
	return res
}

func getManager(dbs []string) *Manager {
	return &Manager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "tf",
		},
		Spec: ManagerSpec{
			Services: Services{
				Cassandras: getCassandras(dbs),
			},
		},
	}
}

func init() {
	os.Setenv(k8sutil.WatchNamespaceEnvVar, "tf")
}

func TestGetDatabaseNodeTypeSingleDB(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme, getManager([]string{"configdb1"}))
	var nodeType string
	nodeType, err = GetDatabaseNodeType(c)
	require.NoError(t, err)
	assert.Equal(t, nodeType, "database")
}

func TestGetDatabaseNodeTypeTwoDB(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme, getManager([]string{"configdb1", "analyticsdb1"}))
	var nodeType string
	nodeType, err = GetDatabaseNodeType(c)
	require.NoError(t, err)
	assert.Equal(t, nodeType, "config-database")
}

func TestGetAnalyticsCassandraInstanceSingleDB(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme, getManager([]string{"configdb1"}))
	var name string
	name, err = GetAnalyticsCassandraInstance(c)
	require.NoError(t, err)
	assert.Equal(t, CassandraInstance, name)
}

func TestGetAnalyticsCassandraInstanceTwoDB(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme, getManager([]string{"configdb1", "analyticsdb1"}))
	var name string
	name, err = GetAnalyticsCassandraInstance(c)
	require.NoError(t, err)
	assert.Equal(t, AnalyticsCassandraInstance, name)
}

func TestGetAnalyticsCassandraInstanceNoDBs(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme, getManager([]string{}))
	var name string
	name, err = GetAnalyticsCassandraInstance(c)
	require.Error(t, err)
	assert.Equal(t, "", name)
}

func TestGetAnalyticsCassandraInstanceNoManager(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err)
	c := fake.NewFakeClientWithScheme(scheme)
	var name string
	name, err = GetAnalyticsCassandraInstance(c)
	require.Error(t, err)
	assert.Equal(t, "", name)
}
