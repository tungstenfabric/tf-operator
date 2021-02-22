package config

import (
	"context"
	"reflect"
	"time"

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

	"github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
)

var log = logf.Log.WithName("controller_config")

var restartTime, _ = time.ParseDuration("1s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

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

	// Watch for changes to primary resource Config
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

	srcCassandra := &source.Kind{Type: &v1alpha1.Cassandra{}}
	cassandraHandler := resourceHandler(mgr.GetClient())
	predCassandraSizeChange := utils.CassandraActiveChange()
	if err = c.Watch(srcCassandra, cassandraHandler, predCassandraSizeChange); err != nil {
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

// Reconcile reconciles Config
func (r *ReconcileConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Start")
	instanceType := "config"
	instance := &v1alpha1.Config{}
	cassandraInstance := &v1alpha1.Cassandra{}
	zookeeperInstance := &v1alpha1.Zookeeper{}
	rabbitmqInstance := &v1alpha1.Rabbitmq{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		reqLogger.Error(err, "Failed to get config obj")
		return reconcile.Result{}, nil
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("Config is deleting, skip reconcile")
		return reconcile.Result{}, nil
	}

	cassandraActive := cassandraInstance.IsActive(instance.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, r.Client)
	zookeeperActive := zookeeperInstance.IsActive(instance.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, r.Client)
	rabbitmqActive := rabbitmqInstance.IsActive(instance.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, r.Client)
	if !cassandraActive || !rabbitmqActive || !zookeeperActive {
		reqLogger.Info("Dependencies not ready", "db", cassandraActive, "zk", zookeeperActive, "rmq", rabbitmqActive)
		return reconcile.Result{}, nil
	}

	servicePortsMap := map[int32]string{
		int32(v1alpha1.ConfigApiPort):    "api",
		int32(v1alpha1.AnalyticsApiPort): "analytics",
	}
	configService := r.Kubernetes.Service(request.Name+"-"+instanceType, corev1.ServiceTypeClusterIP, servicePortsMap, instanceType, instance)

	if err := configService.EnsureExists(); err != nil {
		reqLogger.Error(err, "Config service doesnt exist")
		return reconcile.Result{}, err
	}

	configMapName := request.Name + "-" + instanceType + "-configmap"
	configMap, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "Failed to create configmap")
		return reconcile.Result{}, err
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "Failed to create secret")
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	if err = instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		reqLogger.Error(err, "Failed to prepare stateful set")
		return reconcile.Result{}, err
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec, instanceType, r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	configmapsVolumeName := request.Name + "-" + instanceType + "-volume"
	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMapName:                      configmapsVolumeName,
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})
	instance.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

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
	for idx := range statefulSet.Spec.Template.Spec.Containers {

		container := &statefulSet.Spec.Template.Spec.Containers[idx]

		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer.Command != nil {
			container.Command = instanceContainer.Command
		}

		container.Image = instanceContainer.Image

		container.VolumeMounts = append(container.VolumeMounts,
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
			})

		switch container.Name {

		case "api":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-api --conf_file /etc/contrailconfigmaps/api.${POD_IP} --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP} --worker_id 0",
				}
				container.Command = command
			}

		case "devicemanager":
			if container.Command == nil {
				// exec /usr/bin/contrail-device-manager is importnant as
				// as dm use search byt inclusion in cmd line to kill others
				// and occasionally kill parent bash (and himself indirectly)
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-device-manager --conf_file /etc/contrailconfigmaps/devicemanager.${POD_IP} " +
						" --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}" +
						" --fabric_ansible_conf_file /etc/contrailconfigmaps/contrail-fabric-ansible.conf.${POD_IP} /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				container.Command = command
			}

			container.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_PTRACE", "KILL"},
				},
			}

			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      "tftp",
					MountPath: "/var/lib/tftp",
				},
				corev1.VolumeMount{
					Name:      "dnsmasq",
					MountPath: "/var/lib/dnsmasq",
				},
			)

		case "dnsmasq":
			if container.Command == nil {
				// exec dnsmasq is important because dm does process
				// management by names and search in cmd line
				// (to avoid wrong trigger of search on bash cmd)
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini ; " +
						"mkdir -p /var/lib/dnsmasq ; " +
						"rm -f /var/lib/dnsmasq/base.conf ; " +
						"cp /etc/contrailconfigmaps/dnsmasq_base.${POD_IP} /var/lib/dnsmasq/base.conf ; " +
						"exec dnsmasq -k -p0 --conf-file=/etc/contrailconfigmaps/dnsmasq.${POD_IP}",
				}
				container.Command = command
			}

			container.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			}

			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      "tftp",
					MountPath: "/var/lib/tftp",
				},
				corev1.VolumeMount{
					Name:      "dnsmasq",
					MountPath: "/var/lib/dnsmasq",
				},
			)

		case "servicemonitor":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-svc-monitor --conf_file /etc/contrailconfigmaps/servicemonitor.${POD_IP} --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				container.Command = command
			}

		case "schematransformer":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-schema --conf_file /etc/contrailconfigmaps/schematransformer.${POD_IP}  --conf_file /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				container.Command = command
			}

		case "analyticsapi":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-analytics-api -c /etc/contrailconfigmaps/analyticsapi.${POD_IP} -c /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
				}
				container.Command = command
			}

		case "queryengine":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-query-engine --conf_file /etc/contrailconfigmaps/queryengine.${POD_IP}",
				}
				container.Command = command
			}

		case "collector":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-collector --conf_file /etc/contrailconfigmaps/collector.${POD_IP}",
				}
				container.Command = command
			}

		case "redis":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"exec redis-server --lua-time-limit 15000 --dbfilename '' --bind 127.0.0.1 --port 6379",
				}
				container.Command = command
			}

			readinessProbe := corev1.Probe{
				FailureThreshold: 3,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h 127.0.0.1 -p 6379 ping"},
					},
				},
			}
			startupProbe := corev1.Probe{
				FailureThreshold: 30,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h 127.0.0.1 -p 6379 ping"},
					},
				},
			}
			container.ReadinessProbe = &readinessProbe
			container.StartupProbe = &startupProbe

		case "stunnel":
			if container.Command == nil {
				command := []string{"bash", "-c",
					"mkdir -p /etc/stunnel /var/run/stunnel; " +
						"while [ ! -e /etc/certificates/server-key-${POD_IP}.pem ] ; do sleep 1; done ; " +
						"while [ ! -e /etc/certificates/server-${POD_IP}.crt ] ; do sleep 1; done ; " +
						"while [ ! -e /etc/contrailconfigmaps/stunnel.${POD_IP} ] ; do sleep 1; done ; " +
						"cat /etc/certificates/server-key-${POD_IP}.pem /etc/certificates/server-${POD_IP}.crt > /etc/stunnel/private.pem; " +
						"chmod 600 /etc/stunnel/private.pem; " +
						"exec stunnel /etc/contrailconfigmaps/stunnel.${POD_IP}",
				}
				container.Command = command
			}

		case "nodemanagerconfig":
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/config-nodemanager-runner.sh"}
				container.Command = command
			}

		case "nodemanageranalytics":
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/analytics-nodemanager-runner.sh"}
				container.Command = command
			}

		case "provisioneranalytics", "provisionerconfig":
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/config-provisioner.sh"}
				container.Command = command
			}
		}
	}

	// Configure InitContainers
	for idx := range statefulSet.Spec.Template.Spec.InitContainers {
		container := &statefulSet.Spec.Template.Spec.InitContainers[idx]
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer.Command != nil {
			container.Command = instanceContainer.Command
		}
		container.Image = instanceContainer.Image
	}

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec)

	if created, err := instance.CreateSTS(statefulSet, instanceType, request, r.Client); err != nil || created {
		if err != nil {
			return reconcile.Result{}, err
		}
		return requeueReconcile, err
	}

	if updated, err := instance.UpdateSTS(statefulSet, instanceType, request, r.Client); err != nil || updated {
		if err != nil {
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(request, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(podIPMap) > 0 {
		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			reqLogger.Error(err, "Failed to ensure CertificatesExist")
			return reconcile.Result{}, err
		}

		if err = instance.InstanceConfiguration(configMapName, request, podIPList, r.Client); err != nil {
			reqLogger.Error(err, "Failed to create InstanceConfiguration")
			return reconcile.Result{}, err
		}

		if err = instance.SetPodsToReady(podIPList, r.Client); err != nil {
			reqLogger.Error(err, "Failed to set pods to ready")
			return reconcile.Result{}, err
		}

		if err = instance.ManageNodeStatus(podIPMap, r.Client); err != nil {
			reqLogger.Error(err, "Failed manager node status")
			return reconcile.Result{}, err
		}
	}

	if err = instance.SetEndpointInStatus(r.Client, configService.ClusterIP()); err != nil {
		reqLogger.Error(err, "Failed to set endpointIn status")
		return reconcile.Result{}, err
	}

	falseVal := false
	if instance.Status.ConfigChanged == nil {
		instance.Status.ConfigChanged = &falseVal
	}
	beforeCheck := *instance.Status.ConfigChanged
	newConfigMap := &corev1.ConfigMap{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, newConfigMap); err != nil {
		return reconcile.Result{}, err
	}
	*instance.Status.ConfigChanged = !reflect.DeepEqual(configMap.Data, newConfigMap.Data)

	if *instance.Status.ConfigChanged {
		reqLogger.Info("Update StatefulSet: ConfigChanged")
		if err := r.Client.Update(context.TODO(), statefulSet); err != nil {
			reqLogger.Error(err, "Update StatefulSet failed")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	if beforeCheck != *instance.Status.ConfigChanged {
		reqLogger.Info("Update Status: ConfigChanged")
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
			reqLogger.Error(err, "Update Status failed")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	instance.Status.Active = &falseVal
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, statefulSet, request); err != nil {
		reqLogger.Error(err, "Failed to set instance active")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Done")
	return reconcile.Result{}, nil
}

func (r *ReconcileConfig) ensureCertificatesExist(instance *v1alpha1.Config, pods []corev1.Pod, instanceType string) error {
	domain, err := v1alpha1.ClusterDNSDomain(r.Client)
	if err != nil {
		return err
	}
	subjects := instance.PodsCertSubjects(domain, pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, instance, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
