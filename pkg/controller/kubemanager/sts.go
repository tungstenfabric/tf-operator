package kubemanager

import (
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSTS returns StatefulSet with Kubemanager
func GetSTS() *apps.StatefulSet {
	var replicas = int32(1)
	var labelsMountPermission int32 = 0644

	var podIPEnv = core.EnvVar{
		Name: "POD_IP",
		ValueFrom: &core.EnvVarSource{
			FieldRef: &core.ObjectFieldSelector{
				FieldPath: "status.podIP",
			},
		},
	}

	var podContainers = []core.Container{
		{
			Name:  "kubemanager",
			Image: "tungstenfabric/contrail-kubernetes-kube-manager:latest",
			Env: []core.EnvVar{
				podIPEnv,
			},
		},
	}

	var podVolumes = []core.Volume{
		{
			Name: "contrail-logs",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/log/contrail/kubemanager",
				},
			},
		},
		{
			Name: "host-usr-bin",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/usr/bin",
				},
			},
		},
		{
			Name: "status",
			VolumeSource: core.VolumeSource{
				DownwardAPI: &core.DownwardAPIVolumeSource{
					Items: []core.DownwardAPIVolumeFile{
						{
							Path: "pod_labels",
							FieldRef: &core.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.labels",
							},
						},
						{
							Path: "pod_labelsx",
							FieldRef: &core.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.labels",
							},
						},
					},
					DefaultMode: &labelsMountPermission,
				},
			},
		},
	}

	var podTolerations = []core.Toleration{
		{
			Operator: "Exists",
			Effect:   "NoSchedule",
		},
		{
			Operator: "Exists",
			Effect:   "NoExecute",
		},
	}

	var podSpec = core.PodSpec{
		Volumes:       podVolumes,
		Containers:    podContainers,
		RestartPolicy: "Always",
		DNSPolicy:     "ClusterFirstWithHostNet",
		HostNetwork:   true,
		Tolerations:   podTolerations,
		NodeSelector:  map[string]string{"node-role.kubernetes.io/master": ""},
	}

	var stsTemplate = core.PodTemplateSpec{
		ObjectMeta: meta.ObjectMeta{
			Labels: map[string]string{
				"app":        "kubemanager",
				"tf_manager": "kubemanager",
			},
		},
		Spec: podSpec,
	}

	var stsSelector = meta.LabelSelector{
		MatchLabels: map[string]string{"app": "kubemanager"},
	}

	return &apps.StatefulSet{
		TypeMeta: meta.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      "",
			Namespace: "default",
		},
		Spec: apps.StatefulSetSpec{
			Selector:    &stsSelector,
			ServiceName: "kubemanager",
			Replicas:    &replicas,
			UpdateStrategy: apps.StatefulSetUpdateStrategy{
				Type: apps.OnDeleteStatefulSetStrategyType,
			},
			Template: stsTemplate,
		},
	}
}
