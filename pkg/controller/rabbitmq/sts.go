package rabbitmq

import (
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSTS create default StatefulSet Rabbitmq object
func GetSTS() *apps.StatefulSet {
	var replicas = int32(1)
	var labelsMountPermission int32 = 0644

	var nodeEnv = []core.EnvVar{
		{
			Name: "POD_IP",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		// TODO: dont provide till 2 DBs be supported
		// {
		// 	Name:  "NODE_TYPE",
		// 	Value: "config-database",
		// },
	}

	var podInitContainers = []core.Container{
		{
			Name:  "init",
			Image: "busybox",
			Command: []string{
				"sh",
				"-c",
				"until grep ready /tmp/podinfo/pod_labels > /dev/null 2>&1; do sleep 1; done",
			},
			Env: nodeEnv,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "status",
					MountPath: "/tmp/podinfo",
				},
			},
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: core.TerminationMessageReadFile,
		},
	}

	var podContainers = []core.Container{
		{
			Name:  "rabbitmq",
			Image: "tungstenfabric/contrail-external-rabbitmq:latest",
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "rabbitmq-data",
					MountPath: "/var/lib/rabbitmq",
				},
				{
					Name:      "rabbitmq-logs",
					MountPath: "/var/log/rabbitmq",
				},
			},
			Env: nodeEnv,
			StartupProbe: &core.Probe{
				FailureThreshold: 30,
				PeriodSeconds:    3,
				Handler: core.Handler{
					Exec: &core.ExecAction{
						Command: []string{
							"/bin/bash",
							"-c",
							"cluster_status=$(rabbitmqctl cluster_status);nodes=$(echo $cluster_status | sed -e 's/.*disc,\\[\\(.*\\)]}]}, {.*/\\1/' | grep -oP \"(?<=rabbit@).*?(?=')\"); for node in $(cat /etc/rabbitmq/rabbitmq.nodes); do echo ${nodes} |grep ${node}; if [[ $? -ne 0 ]]; then exit -1; fi; done",
						},
					},
				},
			},
			ReadinessProbe: &core.Probe{
				FailureThreshold: 3,
				PeriodSeconds:    3,
				Handler: core.Handler{
					Exec: &core.ExecAction{
						Command: []string{
							"/bin/bash",
							"-c",
							"cluster_status=$(rabbitmqctl cluster_status);nodes=$(echo $cluster_status | sed -e 's/.*disc,\\[\\(.*\\)]}]}, {.*/\\1/' | grep -oP \"(?<=rabbit@).*?(?=')\"); for node in $(cat /etc/rabbitmq/rabbitmq.nodes); do echo ${nodes} |grep ${node}; if [[ $? -ne 0 ]]; then exit -1; fi; done",
						},
					},
				},
			},
		},
	}

	var podVolumes = []core.Volume{
		{
			Name: "rabbitmq-data",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/lib/contrail/rabbitmq",
				},
			},
		},
		{
			Name: "rabbitmq-logs",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/log/contrail/rabbitmq",
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
		Volumes:        podVolumes,
		InitContainers: podInitContainers,
		Containers:     podContainers,
		RestartPolicy:  "Always",
		DNSPolicy:      "ClusterFirstWithHostNet",
		HostNetwork:    true,
		Tolerations:    podTolerations,
		NodeSelector:   map[string]string{"node-role.kubernetes.io/master": ""},
	}

	var stsTemplate = core.PodTemplateSpec{
		ObjectMeta: meta.ObjectMeta{
			Labels: map[string]string{
				"app":              "rabbitmq",
				"contrail_manager": "rabbitmq",
			},
		},
		Spec: podSpec,
	}

	var stsSelector = meta.LabelSelector{
		MatchLabels: map[string]string{"app": "rabbitmq"},
	}

	return &apps.StatefulSet{
		TypeMeta: meta.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      "rabbitmq",
			Namespace: "default",
		},
		Spec: apps.StatefulSetSpec{
			Selector:    &stsSelector,
			ServiceName: "rabbitmq",
			Replicas:    &replicas,
			Template:    stsTemplate,
		},
	}
}
