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
			Nodes: map[string]string{
				"pod1": "1.1.1.1",
				"pod2": "2.2.2.2",
			},
		},
	}

	control2 := &Control{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control2",
		},
		Status: ControlStatus{
			Nodes: map[string]string{
				"pod3": "3.3.3.3",
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

	vrouter1Controls, err := vrouter1.GetControlNodes(cl)
	require.NoError(t, err)
	vrouter2Controls, err := vrouter2.GetControlNodes(cl)
	require.NoError(t, err)

	assert.Equal(t, "1.1.1.1,2.2.2.2", vrouter1Controls)
	assert.Equal(t, "3.3.3.3", vrouter2Controls)
}
