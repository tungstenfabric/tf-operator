package vrouter

import (
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetDaemonset returns DaemonSet object for vRouter
func GetDaemonset(c *v1alpha1.Vrouter, cniCfg *v1alpha1.CNIConfig, cloudOrchestrator string) *apps.DaemonSet {
	var labelsMountPermission int32 = 0644
	var trueVal = true

	envList := []corev1.EnvVar{
		{
			Name:  "VENDOR_DOMAIN",
			Value: "io.tungsten",
		},
		{
			Name:  "NODE_TYPE",
			Value: "vrouter",
		},
		{
			Name: "POD_IP",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			// TODO: remove after tf-container-builder support PROVISION_HOSTNAME
			Name: "VROUTER_HOSTNAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['hostname']",
				},
			},
		},
		{
			Name: "PROVISION_HOSTNAME",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['hostname']",
				},
			},
		},
		{
			Name:  "CLOUD_ORCHESTRATOR",
			Value: cloudOrchestrator,
		},
		{
			Name: "PHYSICAL_INTERFACE",
			ValueFrom: &core.EnvVarSource{
				FieldRef: &core.ObjectFieldSelector{
					FieldPath: "metadata.annotations['physicalInterface']",
				},
			},
		},
		{
			Name:  "INTROSPECT_SSL_ENABLE",
			Value: "True",
		},
		{
			Name:  "SSL_ENABLE",
			Value: "True",
		},
	}

	var podInitContainers = []core.Container{
		{
			Name:  "vrouterkernelinit",
			Image: "tungstenfabric/contrail-vrouter-kernel-init:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "network-scripts",
					MountPath: "/etc/sysconfig/network-scripts",
				},
				{
					Name:      "host-usr-bin",
					MountPath: "/host/bin",
				},
				{
					Name:      "usr-src",
					MountPath: "/usr/src",
				},
				{
					Name:      "lib-modules",
					MountPath: "/lib/modules",
				},
			},
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vroutercni",
			Image: "tungstenfabric/contrail-kubernetes-cni-init:latest",
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "var-lib-contrail",
					MountPath: "/var/lib/contrail",
				},
				{
					Name:      "cni-config-files",
					MountPath: "/host/etc_cni",
				},
				{
					Name:      "cni-bin",
					MountPath: "/host/opt_cni_bin",
				},
				{
					Name:      "multus-cni",
					MountPath: "/var/run/multus",
				},
			},
		},
	}

	envListNodeInit := append(envList,
		corev1.EnvVar{
			Name:  "SERVER_CA_CERTFILE",
			Value: certificates.SignerCAFilepath,
	},
		corev1.EnvVar{
			Name:  "SERVER_CERTFILE",
			Value: "/etc/certificates/server-${POD_IP}.crt",
		},
		corev1.EnvVar{
			Name:  "SERVER_KEYFILE",
			Value: "/etc/certificates/server-key-${POD_IP}.pem",
		},
	)
	podInitContainerMounts := []core.VolumeMount{
		{
			Name:      "host-usr-bin",
			MountPath: "/host/usr/bin",
		},
		{
			Name:      "var-run",
			MountPath: "/var/run",
		},
		{
			Name:      "dev",
			MountPath: "/dev",
		},
	}
	if c.Spec.CommonConfiguration.TuneSysctl != nil && *c.Spec.CommonConfiguration.TuneSysctl {
		podInitContainerMounts = append(podInitContainerMounts,
			core.VolumeMount{
				Name:      "host-sysctl",
				MountPath: "/etc/sysctl.d",
			})
	}
	podInitContainers = append(podInitContainers,
		core.Container{
			Name:         "nodeinit",
			Image:        "tungstenfabric/contrail-node-init:latest",
			Env:          envListNodeInit,
			VolumeMounts: podInitContainerMounts,
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
		},
		// for password protected it is needed to prefetch contrail-status image as it
		// is not available w/o image secret
		core.Container{
			Name:    "nodeinit-status-prefetch",
			Image:	 "tungstenfabric/contrail-status:latest",
			Command: []string{"sh", "-c", "exit 0"},
		},
		// for password protected it is needed to prefetch contrail-tools image as it
		// is not available w/o image secret
		core.Container{
			Name:    "nodeinit-tools-prefetch",
			Image:	  "tungstenfabric/contrail-tools:latest",
			Command: []string{"sh", "-c", "exit 0"},
		},
	)


	var podContainers = []core.Container{
		{
			Name:  "provisioner",
			Image: "tungstenfabric/contrail-provisioner:latest",
			Env:   envList,
		},
		{
			Name:  "nodemanager",
			Image: "tungstenfabric/contrail-nodemgr:latest",
			Env:   envList,
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vrouteragent",
			Image: "tungstenfabric/contrail-vrouter-agent:latest",
			Env:   envList,
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "dev",
					MountPath: "/dev",
				},
				{
					Name:      "network-scripts",
					MountPath: "/etc/sysconfig/network-scripts",
				},
				{
					Name:      "host-usr-bin",
					MountPath: "/host/bin",
				},
				{
					Name:      "usr-src",
					MountPath: "/usr/src",
				},
				{
					Name:      "lib-modules",
					MountPath: "/lib/modules",
				},
				// declared in AddNodemanagerVolumes
				{
					Name:      "var-run",
					MountPath: "/var/run",
				},
				{
					Name:      "var-lib-contrail",
					MountPath: "/var/lib/contrail",
				},
			},
			SecurityContext: &core.SecurityContext{
				Privileged: &trueVal,
			},
			Lifecycle: &core.Lifecycle{
				PreStop: &core.Handler{
					Exec: &core.ExecAction{
						Command: []string{"/clean-up.sh"},
					},
				},
			},
		},
	}

	var podVolumes = []core.Volume{
		{
			Name: "contrail-logs",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/log/contrail/vrouter-agent",
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
			Name: "var-lib-contrail",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/lib/contrail",
				},
			},
		},
		{
			Name: "lib-modules",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/lib/modules",
				},
			},
		},
		{
			Name: "usr-src",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/usr/src",
				},
			},
		},
		{
			Name: "network-scripts",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/etc/sysconfig/network-scripts",
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/dev",
				},
			},
		},
		{
			Name: "cni-config-files",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: cniCfg.ConfigPath,
				},
			},
		},
		{
			Name: "cni-bin",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: cniCfg.BinaryPath,
				},
			},
		},
		{
			Name: "multus-cni",
			VolumeSource: core.VolumeSource{
				HostPath: &core.HostPathVolumeSource{
					Path: "/var/run/multus",
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
	if c.Spec.CommonConfiguration.TuneSysctl != nil && *c.Spec.CommonConfiguration.TuneSysctl {
		podVolumes = append(podVolumes,
			core.Volume{
				Name: "host-sysctl",
				VolumeSource: core.VolumeSource{
					HostPath: &core.HostPathVolumeSource{
						Path: "/etc/sysctl.d",
					},
				},
			})
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
	}

	v1alpha1.AddCommonVolumes(&podSpec, c.Spec.CommonConfiguration)
	v1alpha1.DefaultSecurityContext(&podSpec)

	var daemonSetSelector = meta.LabelSelector{
		MatchLabels: map[string]string{"app": "vrouter"},
	}

	var daemonsetTemplate = core.PodTemplateSpec{
		ObjectMeta: meta.ObjectMeta{},
		Spec:       podSpec,
	}

	var daemonSet = apps.DaemonSet{
		TypeMeta: meta.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      "vrouter",
			Namespace: "default",
		},
		Spec: apps.DaemonSetSpec{
			Selector: &daemonSetSelector,
			Template: daemonsetTemplate,
		},
	}

	return &daemonSet
}
