package config

import (
	"context"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	configtemplates "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/controller/utils"
	"github.com/Juniper/contrail-operator/pkg/k8s"
)

var log = logf.Log.WithName("controller_config")

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ConfigList{}
			err := myclient.List(context.TODO(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      app.GetName(),
						Namespace: e.Meta.GetNamespace(),
					}})
				}
			}
		},
		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.MetaNew.GetNamespace()}
			list := &v1alpha1.ConfigList{}
			err := myclient.List(context.TODO(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      app.GetName(),
						Namespace: e.MetaNew.GetNamespace(),
					}})
				}
			}
		},
		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ConfigList{}
			err := myclient.List(context.TODO(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      app.GetName(),
						Namespace: e.Meta.GetNamespace(),
					}})
				}
			}
		},
		GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ConfigList{}
			err := myclient.List(context.TODO(), list, listOps)
			if err == nil {
				for _, app := range list.Items {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      app.GetName(),
						Namespace: e.Meta.GetNamespace(),
					}})
				}
			}
		},
	}
	return appHandler
}

// Add adds the Config controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileConfig{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Manager:    mgr,
		Kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
}
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New("config-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Config.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Config{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	serviceMap := map[string]string{"contrail_manager": "config"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predInitStatus := utils.PodInitStatusChange(serviceMap)
	predPodIPChange := utils.PodIPChange(serviceMap)
	predInitRunning := utils.PodInitRunning(serviceMap)

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Config{},
	}); err != nil {
		return err
	}

	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}
	if err = c.Watch(srcPod, podHandler, predInitStatus); err != nil {
		return err
	}
	if err = c.Watch(srcPod, podHandler, predInitRunning); err != nil {
		return err
	}

	// srcCassandra := &source.Kind{Type: &v1alpha1.Cassandra{}}
	// cassandraHandler := resourceHandler(mgr.GetClient())
	// predCassandraSizeChange := utils.CassandraActiveChange()
	// if err = c.Watch(srcCassandra, cassandraHandler, predCassandraSizeChange); err != nil {
	// 	return err
	// }
	if err = c.Watch(
		&source.Kind{Type: &appsv1.StatefulSet{}},
		resourceHandler(mgr.GetClient()),
		utils.STSStatusChange(utils.CassandraGroupKind())); err != nil {

		return err
	}

	srcRabbitmq := &source.Kind{Type: &v1alpha1.Rabbitmq{}}
	rabbitmqHandler := resourceHandler(mgr.GetClient())
	predRabbitmqSizeChange := utils.RabbitmqActiveChange()
	if err = c.Watch(srcRabbitmq, rabbitmqHandler, predRabbitmqSizeChange); err != nil {
		return err
	}

	srcZookeeper := &source.Kind{Type: &v1alpha1.Zookeeper{}}
	zookeeperHandler := resourceHandler(mgr.GetClient())
	predZookeeperSizeChange := utils.ZookeeperActiveChange()
	if err = c.Watch(srcZookeeper, zookeeperHandler, predZookeeperSizeChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Config{},
	}
	stsPred := utils.STSStatusChange(utils.ConfigGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileConfig implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileConfig{}

// ReconcileConfig reconciles a Config object.
type ReconcileConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles Config.
func (r *ReconcileConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithValues("Request.Namespace", request.Namespace,
		"Request.Name", request.Name)
	reqLogger.Info("Start")
	instanceType := "config"
	config := &v1alpha1.Config{}
	cassandraInstance := &v1alpha1.Cassandra{}
	zookeeperInstance := &v1alpha1.Zookeeper{}
	rabbitmqInstance := &v1alpha1.Rabbitmq{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, config); err != nil && errors.IsNotFound(err) {
		reqLogger.Error(err, "Failed to get config obj")
		return reconcile.Result{}, nil
	}

	if !config.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("Config is deleting, skip reconcile")
		return reconcile.Result{}, nil
	}
	// cassandraActive := cassandraInstance.IsActive(config.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, r.Client)
	cassandraScheduled := cassandraInstance.IsScheduled(config.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, r.Client)
	zookeeperActive := zookeeperInstance.IsActive(config.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, r.Client)
	rabbitmqActive := rabbitmqInstance.IsActive(config.Labels["contrail_cluster"], request.Namespace, r.Client)

	if !cassandraScheduled || !rabbitmqActive || !zookeeperActive {
		reqLogger.Info("Config DB is not ready", "cassandraScheduled", cassandraScheduled,
			"zookeeperActive", zookeeperActive, "rabbitmqActive", rabbitmqActive)
		return reconcile.Result{}, nil
	}
	servicePortsMap := map[int32]string{
		int32(v1alpha1.ConfigApiPort):    "api",
		int32(v1alpha1.AnalyticsApiPort): "analytics",
	}
	configService := r.Kubernetes.Service(request.Name+"-"+instanceType, corev1.ServiceTypeClusterIP, servicePortsMap, instanceType, config)

	if err := configService.EnsureExists(); err != nil {
		reqLogger.Error(err, "Config service doesnt exist")
		return reconcile.Result{}, err
	}
	currentConfigMap, currentConfigExists := config.CurrentConfigMapExists(request.Name+"-"+instanceType+"-configmap", r.Client, r.Scheme, request)

	configMap, err := config.CreateConfigMap(request.Name+"-"+instanceType+"-configmap", r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "Failed to create configmap")
		return reconcile.Result{}, err
	}

	secretCertificates, err := config.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "Failed to create secret")
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	// DeviceManager pushes configuration to dnsmasq service and then needs to restart it by sending a signal.
	// Therefore those services needs to share a one process namespace
	// TODO: Move device manager and dnsmasq to a separate pod. They are separate services which requires
	// persistent volumes and capabilities
	trueVal := true
	statefulSet.Spec.Template.Spec.ShareProcessNamespace = &trueVal

	if err = config.PrepareSTS(statefulSet, &config.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		reqLogger.Error(err, "Failed to prepare stateful set")
		return reconcile.Result{}, err
	}

	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	config.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMap.Name:                     request.Name + "-" + instanceType + "-volume",
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})
	config.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

	statefulSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      instanceType,
						Operator: "In",
						Values:   []string{request.Name},
					}},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}
	for idx, container := range statefulSet.Spec.Template.Spec.Containers {

		switch container.Name {
		case "api":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-api --conf_file /etc/contrailconfigmaps/api.${POD_IP} --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP} --worker_id 0",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "devicemanager":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				// exec /usr/bin/contrail-device-manager is importnant as
				// as dm use search byt inclusion in cmd line to kill others
				// and occasionally kill parent bash (and himself indirectly)
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-device-manager --conf_file /etc/contrailconfigmaps/devicemanager.${POD_IP} " +
						" --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}" +
						" --fabric_ansible_conf_file /etc/contrailconfigmaps/contrail-fabric-ansible.conf.${POD_IP} /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			(&statefulSet.Spec.Template.Spec.Containers[idx]).SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_PTRACE", "KILL"},
				},
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
				corev1.VolumeMount{
					Name:      "tftp",
					MountPath: "/var/lib/tftp",
				},
				corev1.VolumeMount{
					Name:      "dnsmasq",
					MountPath: "/var/lib/dnsmasq",
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "dnsmasq":
			container := &statefulSet.Spec.Template.Spec.Containers[idx]
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				// exec dnsmasq is important because dm does process
				// management by names and search in cmd line
				// (to avoid wrong trigger of search on bash cmd)
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini ; " +
						"mkdir -p /var/lib/dnsmasq ; " +
						"rm -f /var/lib/dnsmasq/base.conf ; " +
						"cp /etc/contrailconfigmaps/dnsmasq_base.${POD_IP} /var/lib/dnsmasq/base.conf ; " +
						"exec dnsmasq -k -p0 --conf-file=/etc/contrailconfigmaps/dnsmasq.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			container.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
				corev1.VolumeMount{
					Name:      "tftp",
					MountPath: "/var/lib/tftp",
				},
				corev1.VolumeMount{
					Name:      "dnsmasq",
					MountPath: "/var/lib/dnsmasq",
				},
			)
			// DNSMasq container requires those variables to be set
			container.VolumeMounts = volumeMountList
			container.Image = instanceContainer.Image

		case "servicemonitor":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-svc-monitor --conf_file /etc/contrailconfigmaps/servicemonitor.${POD_IP} --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

		case "schematransformer":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-schema --conf_file /etc/contrailconfigmaps/schematransformer.${POD_IP}  --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "analyticsapi":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-analytics-api -c /etc/contrailconfigmaps/analyticsapi.${POD_IP} -c /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "queryengine":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-query-engine --conf_file /etc/contrailconfigmaps/queryengine.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "collector":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-collector --conf_file /etc/contrailconfigmaps/collector.${POD_IP}",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
				corev1.VolumeMount{
					Name:      request.Name + "-secret-certificates",
					MountPath: "/etc/certificates",
				},
				corev1.VolumeMount{
					Name:      csrSignerCaVolumeName,
					MountPath: certificates.SignerCAMountPath,
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		case "redis":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"exec redis-server --lua-time-limit 15000 --dbfilename '' --bind 127.0.0.1 ${POD_IP} --port 6379",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
			)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
			readinessProbe := corev1.Probe{
				FailureThreshold: 3,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h ${POD_IP} -p 6379 ping"},
					},
				},
			}
			startupProbe := corev1.Probe{
				FailureThreshold: 30,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h ${POD_IP} -p 6379 ping"},
					},
				},
			}
			(&statefulSet.Spec.Template.Spec.Containers[idx]).ReadinessProbe = &readinessProbe
			(&statefulSet.Spec.Template.Spec.Containers[idx]).StartupProbe = &startupProbe

		case "nodemanagerconfig":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"ln -sf /etc/contrailconfigmaps/nodemanagerconfig.${POD_IP} /etc/contrail/contrail-config-nodemgr.conf; " +
						"exec /usr/bin/contrail-nodemgr --nodetype=contrail-config",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
			)
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

		case "nodemanageranalytics":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"ln -sf /etc/contrailconfigmaps/nodemanageranalytics.${POD_IP} /etc/contrail/contrail-analytics-nodemgr.conf; " +
						"exec /usr/bin/contrail-nodemgr --nodetype=contrail-analytics",
				}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := statefulSet.Spec.Template.Spec.Containers[idx].VolumeMounts
			volumeMountList = append(volumeMountList,
				corev1.VolumeMount{
					Name:      request.Name + "-" + instanceType + "-volume",
					MountPath: "/etc/contrailconfigmaps",
				},
			)
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

		case "provisioneranalytics", "provisionerconfig":
			instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command != nil {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}

			volumeMountList := []corev1.VolumeMount{}
			if len((&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts
			}
			volumeMountList = append(volumeMountList, corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			})
			volumeMountList = append(volumeMountList, corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			})
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList

			envList := []corev1.EnvVar{}
			if len((&statefulSet.Spec.Template.Spec.Containers[idx]).Env) > 0 {
				envList = (&statefulSet.Spec.Template.Spec.Containers[idx]).Env
			}
			envList = append(envList, corev1.EnvVar{
				Name:  "SSL_ENABLE",
				Value: "True",
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_CA_CERTFILE",
				Value: certificates.SignerCAFilepath,
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_CERTFILE",
				Value: "/etc/certificates/server-$(POD_IP).crt",
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_KEYFILE",
				Value: "/etc/certificates/server-key-$(POD_IP).pem",
			})
			configNodesInformation, err := v1alpha1.NewConfigClusterConfiguration(config.Labels["contrail_cluster"], request.Namespace, r.Client)
			if err != nil {
				reqLogger.Error(err, "Failed to create ConfigClusterConfiguration")
				return reconcile.Result{}, err
			}
			configNodeList := configNodesInformation.APIServerIPList
			envList = append(envList, corev1.EnvVar{
				Name:  "CONFIG_NODES",
				Value: configtemplates.JoinListWithSeparator(configNodeList, ","),
			})
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Env = envList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
		}
	}

	// Configure InitContainers
	for idx, container := range statefulSet.Spec.Template.Spec.InitContainers {
		instanceContainer := utils.GetContainerFromList(container.Name, config.Spec.ServiceConfiguration.Containers)
		(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
		if instanceContainer.Command != nil {
			(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Command = instanceContainer.Command
		}
	}

	configChanged := false
	if config.Status.ConfigChanged != nil {
		configChanged = *config.Status.ConfigChanged
	}

	if err = config.CreateSTS(statefulSet, instanceType, request, r.Client); err != nil {
		reqLogger.Error(err, "Failed to create stateful set")
		return reconcile.Result{}, err
	}

	strategy := "deleteFirst"
	if err = config.UpdateSTS(statefulSet, instanceType, request, r.Client, strategy); err != nil {
		reqLogger.Error(err, "Failed to update stateful set")
		return reconcile.Result{}, err
	}

	podIPList, podIPMap, err := config.PodIPListAndIPMapFromInstance(request, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(podIPMap) > 0 {
		if err = config.InstanceConfiguration(request, podIPList, r.Client); err != nil {
			reqLogger.Error(err, "Failed to create InstanceConfiguration")
			return reconcile.Result{}, err
		}

		if err := r.ensureCertificatesExist(config, podIPList, instanceType); err != nil {
			reqLogger.Error(err, "Failed to ensure CertificatesExist")
			return reconcile.Result{}, err
		}

		if err = config.SetPodsToReady(podIPList, r.Client); err != nil {
			reqLogger.Error(err, "Failed to set pods to ready")
			return reconcile.Result{}, err
		}

		if err = config.WaitForPeerPods(request, r.Client); err != nil {
			reqLogger.Error(err, "Failed to wait peer pods")
			return reconcile.Result{}, err
		}

		if err = config.ManageNodeStatus(podIPMap, r.Client); err != nil {
			reqLogger.Error(err, "Failed manager node status")
			return reconcile.Result{}, err
		}
	}

	if err = config.SetEndpointInStatus(r.Client, configService.ClusterIP()); err != nil {
		reqLogger.Error(err, "Failed to set endpointIn status")
		return reconcile.Result{}, err
	}

	if currentConfigExists {
		newConfigMap := &corev1.ConfigMap{}
		_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-" + instanceType + "-configmap", Namespace: request.Namespace}, newConfigMap)
		if !reflect.DeepEqual(currentConfigMap.Data, newConfigMap.Data) {
			configChanged = true
		} else {
			configChanged = false
		}
		config.Status.ConfigChanged = &configChanged
	}

	if config.Status.Active == nil {
		active := false
		config.Status.Active = &active
	}
	if err = config.SetInstanceActive(r.Client, config.Status.Active, statefulSet, request); err != nil {
		reqLogger.Error(err, "Failed to set instance active")
		return reconcile.Result{}, err
	}
	if config.Status.ConfigChanged != nil {
		if *config.Status.ConfigChanged {
			reqLogger.Info("Done: Result{Requeue: true}")
			return reconcile.Result{Requeue: true}, nil
		}
	}
	reqLogger.Info("Done: Result{}")
	return reconcile.Result{}, nil
}

func (r *ReconcileConfig) ensureCertificatesExist(config *v1alpha1.Config, pods *corev1.PodList, instanceType string) error {
	subjects := config.PodsCertSubjects(pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, config, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
