package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestCleanupContainers(t *testing.T) {
	statefulSet := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container1",
							Image: "image1",
						},
						{
							Name:  "container2",
							Image: "image2",
						},
						{
							Name:  "container3",
							Image: "image3",
						},
					},
				},
			},
		},
	}
	filter := []*v1alpha1.Container{
		{
			Name:  "container2",
			Image: "image2",
		},
	}
	CleanupContainers(&statefulSet.Spec.Template.Spec, filter)
	require.Equal(t, 1, len(statefulSet.Spec.Template.Spec.Containers))
	require.Equal(t, "container2", statefulSet.Spec.Template.Spec.Containers[0].Name)
	require.Equal(t, "image2", statefulSet.Spec.Template.Spec.Containers[0].Image)
}

func TestCleanupContainersSame(t *testing.T) {
	statefulSet := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container1",
							Image: "image1",
						},
						{
							Name:  "container2",
							Image: "image2",
						},
					},
				},
			},
		},
	}
	filter := []*v1alpha1.Container{
		{
			Name:  "container1",
			Image: "image1",
		},
		{
			Name:  "container2",
			Image: "image2",
		},
	}
	CleanupContainers(&statefulSet.Spec.Template.Spec, filter)
	require.Equal(t, 2, len(statefulSet.Spec.Template.Spec.Containers))
	require.Equal(t, "container1", statefulSet.Spec.Template.Spec.Containers[0].Name)
	require.Equal(t, "image1", statefulSet.Spec.Template.Spec.Containers[0].Image)
	require.Equal(t, "container2", statefulSet.Spec.Template.Spec.Containers[1].Name)
	require.Equal(t, "image2", statefulSet.Spec.Template.Spec.Containers[1].Image)
}
