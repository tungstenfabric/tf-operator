package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVrouterControlInstanceSelection(t *testing.T) {
	control1 := &Control{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control1",
		},
		Status: ControlStatus{
			CommonStatus: CommonStatus{
				Nodes: map[string]NodeInfo{
					"pod1": {IP: "1.1.1.1", Hostname: "node1"},
					"pod2": {IP: "2.2.2.2", Hostname: "node2"},
				},
			},
		},
	}

	control2 := &Control{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control2",
		},
		Status: ControlStatus{
			CommonStatus: CommonStatus{
				Nodes: map[string]NodeInfo{
					"pod3": {IP: "3.3.3.3", Hostname: "node3"},
				},
			},
		},
	}

	vrouter1 := Vrouter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vrouter1",
		},
		Spec: VrouterSpec{
			ServiceConfiguration: VrouterConfiguration{
				ControlInstance: "control1",
			},
		},
	}

	vrouter2 := Vrouter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vrouter1",
		},
		Spec: VrouterSpec{
			ServiceConfiguration: VrouterConfiguration{
				ControlInstance: "control2",
			},
		},
	}

	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	cl := fake.NewFakeClientWithScheme(scheme, control1, control2)

	vrouter1Controls, err := GetControlNodes(vrouter1.GetNamespace(),
		vrouter1.Spec.ServiceConfiguration.ControlInstance, vrouter1.Spec.ServiceConfiguration.DataSubnet, cl)
	require.NoError(t, err)
	vrouter2Controls, err := GetControlNodes(vrouter2.GetNamespace(),
		vrouter2.Spec.ServiceConfiguration.ControlInstance, vrouter2.Spec.ServiceConfiguration.DataSubnet, cl)
	require.NoError(t, err)

	assert.Equal(t, "node1,node2", vrouter1Controls)
	assert.Equal(t, "node3", vrouter2Controls)
}

func TestVrouterParamsTest(t *testing.T) {
	scheme, err := SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, corev1.SchemeBuilder.AddToScheme(scheme), "Failed to add CoreV1 into scheme")

	var hp256 int = 256
	vrouter := &Vrouter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vrouter1",
		},
		Spec: VrouterSpec{
			ServiceConfiguration: VrouterConfiguration{
				ControlInstance:   "control1",
				HugePages2M:       &hp256,
				L3MHCidr:          "172.1.1.0/24",
				PhysicalInterface: "ens3,ens4",
			},
		},
	}

	cl := fake.NewFakeClientWithScheme(scheme, vrouter)
	cfg, err := vrouter.VrouterConfigurationParameters(cl)
	require.NoError(t, err, "Failed to get VrouterConfigurationParameters")
	require.Equal(t, "ens3,ens4", cfg.PhysicalInterface)

	paramsStr, err := vrouter.GetParamsEnv(cl, &ClusterNodes{})
	require.NoError(t, err, "Failed to get GetParamsEnv")
	require.Contains(t, paramsStr, "PHYSICAL_INTERFACE=\"ens3,ens4\"")
}
