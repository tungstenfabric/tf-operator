package vrouter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func requireTestEnv(t *testing.T, c *corev1.Container) {
	for _, v := range c.Env {
		if v.Name == "TEST" {
			require.Equal(t, "TESTVAL", v.Value)
			return
		}
	}
	require.FailNow(t, "Env variable TEST not found")
}

func TestVrouterEnv(t *testing.T) {
	vrouter := v1alpha1.Vrouter{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vrouter1",
		},
		Spec: v1alpha1.VrouterSpec{
			ServiceConfiguration: v1alpha1.VrouterConfiguration{
				ControlInstance: "control1",
				EnvVariablesConfig: map[string]string{
					"TEST": "TESTVAL",
				},
			},
		},
	}
	cniConfig := &v1alpha1.CNIConfig{
		BinaryPath: "",
		ConfigPath: "",
	}
	ds := GetDaemonset(&vrouter, cniConfig, "kubernetes")
	require.Greater(t, len(ds.Spec.Template.Spec.Containers), 0)
	for _, c := range ds.Spec.Template.Spec.Containers {
		requireTestEnv(t, &c)
	}
	require.Greater(t, len(ds.Spec.Template.Spec.InitContainers), 0)
	for _, c := range ds.Spec.Template.Spec.InitContainers {
		requireTestEnv(t, &c)
	}
}
