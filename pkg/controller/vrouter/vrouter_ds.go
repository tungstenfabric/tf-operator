package vrouter

import (
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func contains(n string, l []corev1.EnvVar) (*corev1.EnvVar, bool) {
	for _, v := range l {
		if v.Name == n {
			return &v, true
		}
	}
	return nil, false
}

func updateContainerEnv(v *v1alpha1.Vrouter, c *corev1.Container) {
	for n, v := range v.Spec.ServiceConfiguration.EnvVariablesConfig {
		if pe, ok := contains(n, c.Env); ok {
			pe.Value = v
		} else {
			c.Env = append(c.Env, corev1.EnvVar{Name: n, Value: v})
		}
	}
}

func updateEnv(v *v1alpha1.Vrouter, ds *apps.DaemonSet) {
	for idx := range ds.Spec.Template.Spec.Containers {
		updateContainerEnv(v, &ds.Spec.Template.Spec.Containers[idx])
	}
	for idx := range ds.Spec.Template.Spec.InitContainers {
		updateContainerEnv(v, &ds.Spec.Template.Spec.InitContainers[idx])
	}
}

//GetDaemonset returns DaemonSet object for vRouter
func GetDaemonset(c *v1alpha1.Vrouter, cniCfg *v1alpha1.CNIConfig, cloudOrchestrator string) *apps.DaemonSet {
	var labelsMountPermission int32 = 0644
	var trueVal = true

	envList := []corev1.EnvVar{
		{
			Name:  "NODE_TYPE",
			Value: "vrouter",
		},
		{
			Name: "POD_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "CLOUD_ORCHESTRATOR",
			Value: cloudOrchestrator,
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

	var podInitContainers = []corev1.Container{
		{
			Name:  "vrouterkernelinit",
			Image: "tungstenfabric/contrail-vrouter-kernel-init:latest",
			Env:   envList,
			VolumeMounts: []corev1.VolumeMount{
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
			SecurityContext: &corev1.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vroutercni",
			Image: "tungstenfabric/contrail-kubernetes-cni-init:latest",
			VolumeMounts: []corev1.VolumeMount{
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
			Value: v1alpha1.SignerCAFilepath,
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
	podInitContainerMounts := []corev1.VolumeMount{
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
	if !k8s.IsOpenshift() {
		podInitContainerMounts = append(podInitContainerMounts,
			corev1.VolumeMount{
				Name:      "host-sysctl",
				MountPath: "/etc/sysctl.d",
			})
	}
	podInitContainers = append(podInitContainers,
		corev1.Container{
			Name:         "nodeinit",
			Image:        "tungstenfabric/contrail-node-init:latest",
			Env:          envListNodeInit,
			VolumeMounts: podInitContainerMounts,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &trueVal,
			},
		},
		// for password protected it is needed to prefetch contrail-status image as it
		// is not available w/o image secret
		corev1.Container{
			Name:    "nodeinit-status-prefetch",
			Image:   "tungstenfabric/contrail-status:latest",
			Command: []string{"sh", "-c", "exit 0"},
		},
		// for password protected it is needed to prefetch contrail-tools image as it
		// is not available w/o image secret
		corev1.Container{
			Name:    "nodeinit-tools-prefetch",
			Image:   "tungstenfabric/contrail-tools:latest",
			Command: []string{"sh", "-c", "exit 0"},
		},
	)

	var resources corev1.ResourceRequirements
	if c.Spec.ServiceConfiguration.HugePages1G != nil {
		// To request hugepages it is needed to request either cpu or mem
		// (that is kuberenetes api requirement)
		const onePage int64 = 1024 * 1024 * 1024
		qmem := *resource.NewQuantity(onePage*int64(*c.Spec.ServiceConfiguration.HugePages1G), resource.BinarySI)
		resources.Limits = corev1.ResourceList{
			corev1.ResourceHugePagesPrefix + "1Gi": qmem,
		}
		resources.Requests = corev1.ResourceList{
			corev1.ResourceHugePagesPrefix + "1Gi": qmem,
			corev1.ResourceMemory:                  qmem,
		}
	}
	if c.Spec.ServiceConfiguration.HugePages2M != nil {
		// To request hugepages it is needed to request either cpu or mem
		// (that is kuberenetes api requirement)
		const onePage int64 = 2 * 1024 * 1024
		qmem := *resource.NewQuantity(onePage*int64(*c.Spec.ServiceConfiguration.HugePages2M), resource.BinarySI)
		resources.Limits = corev1.ResourceList{
			corev1.ResourceHugePagesPrefix + "2Mi": qmem,
		}
		resources.Requests = corev1.ResourceList{
			corev1.ResourceMemory:                  qmem,
			corev1.ResourceHugePagesPrefix + "2Mi": qmem,
		}
	}

	var podContainers = []corev1.Container{
		{
			Name:  "provisioner",
			Image: "tungstenfabric/contrail-provisioner:latest",
			Env:   envList,
		},
		{
			Name:  "nodemanager",
			Image: "tungstenfabric/contrail-nodemgr:latest",
			Env:   envList,
			SecurityContext: &corev1.SecurityContext{
				Privileged: &trueVal,
			},
		},
		{
			Name:  "vrouteragent",
			Image: "tungstenfabric/contrail-vrouter-agent:latest",
			Env:   envList,
			VolumeMounts: []corev1.VolumeMount{
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
			SecurityContext: &corev1.SecurityContext{
				Privileged: &trueVal,
			},
			Resources: resources,
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"/clean-up.sh"},
					},
				},
			},
		},
	}

	var podVolumes = []corev1.Volume{
		{
			Name: "contrail-logs",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/log/contrail/vrouter-agent",
				},
			},
		},
		{
			Name: "host-usr-bin",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/bin",
				},
			},
		},
		{
			Name: "var-lib-contrail",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/contrail",
				},
			},
		},
		{
			Name: "lib-modules",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/lib/modules",
				},
			},
		},
		{
			Name: "usr-src",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/src",
				},
			},
		},
		{
			Name: "network-scripts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/sysconfig/network-scripts",
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev",
				},
			},
		},
		{
			Name: "cni-config-files",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: cniCfg.ConfigPath,
				},
			},
		},
		{
			Name: "cni-bin",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: cniCfg.BinaryPath,
				},
			},
		},
		{
			Name: "multus-cni",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/multus",
				},
			},
		},
		{
			Name: "status",
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: "pod_labels",
							FieldRef: &corev1.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.labels",
							},
						},
						{
							Path: "pod_labelsx",
							FieldRef: &corev1.ObjectFieldSelector{
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
	if !k8s.IsOpenshift() {
		podVolumes = append(podVolumes,
			corev1.Volume{
				Name: "host-sysctl",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/sysctl.d",
					},
				},
			})
	}
	var podTolerations = []corev1.Toleration{
		{
			Operator: "Exists",
			Effect:   "NoSchedule",
		},
		{
			Operator: "Exists",
			Effect:   "NoExecute",
		},
	}

	var podSpec = corev1.PodSpec{
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

	var daemonsetTemplate = corev1.PodTemplateSpec{
		ObjectMeta: meta.ObjectMeta{},
		Spec:       podSpec,
	}

	var daemonSet = &apps.DaemonSet{
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

	updateEnv(c, daemonSet)
	return daemonSet
}
