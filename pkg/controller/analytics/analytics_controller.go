package analytics

import (
	"context"
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

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
)

var log = logf.Log.WithName("controller_analytics")

var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.AnalyticsList{}
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
			list := &v1alpha1.AnalyticsList{}
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
			list := &v1alpha1.AnalyticsList{}
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
			list := &v1alpha1.AnalyticsList{}
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

// Add adds the Analytics controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAnalytics{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Manager:    mgr,
		Kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
}
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New("analytics-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Analytics
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Analytics{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	serviceMap := map[string]string{"tf_manager": "analytics"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Analytics{},
	}); err != nil {
		return err
	}

	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
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

	srcRedis := &source.Kind{Type: &v1alpha1.Redis{}}
	redisHandler := resourceHandler(mgr.GetClient())
	predRedisSizeChange := utils.RedisActiveChange()
	if err = c.Watch(srcRedis, redisHandler, predRedisSizeChange); err != nil {
		return err
	}

	srcConfig := &source.Kind{Type: &v1alpha1.Config{}}
	configHandler := resourceHandler(mgr.GetClient())
	predConfigSizeChange := utils.ConfigActiveChange()
	if err = c.Watch(srcConfig, configHandler, predConfigSizeChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Analytics{},
	}
	stsPred := utils.STSStatusChange(utils.ConfigGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCAnalytics implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileAnalytics{}

// ReconcileAnalytics reconciles a Analytics object.
type ReconcileAnalytics struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles Analytics
func (r *ReconcileAnalytics) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Start")
	instanceType := "analytics"
	// Check ZIU status
	f, err := v1alpha1.CanReconcile("Analytics", r.Client)
	if err != nil {
		log.Error(err, "When check analytics ziu status")
		return reconcile.Result{}, err
	}
	if !f {
		log.Info("analytics reconcile blocks by ZIU status")
		return reconcile.Result{Requeue: true, RequeueAfter: v1alpha1.ZiuRestartTime}, nil
	}
	instance := &v1alpha1.Analytics{}
	cassandraInstance := &v1alpha1.Cassandra{}
	zookeeperInstance := &v1alpha1.Zookeeper{}
	rabbitmqInstance := &v1alpha1.Rabbitmq{}
	redisInstance := &v1alpha1.Redis{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		reqLogger.Error(err, "Failed to get analytics obj")
		return reconcile.Result{}, nil
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("Analytics is deleting, skip reconcile")
		return reconcile.Result{}, nil
	}

	analyticsCassandraInstance, err := v1alpha1.GetAnalyticsCassandraInstance(r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}

	cassandraActive := cassandraInstance.IsActive(analyticsCassandraInstance, request.Namespace, r.Client)
	rabbitmqActive := rabbitmqInstance.IsActive(v1alpha1.RabbitmqInstance, request.Namespace, r.Client)
	zookeeperActive := zookeeperInstance.IsActive(v1alpha1.ZookeeperInstance, request.Namespace, r.Client)
	redisActive := redisInstance.IsActive(v1alpha1.RedisInstance, request.Namespace, r.Client)
	if !cassandraActive || !rabbitmqActive || !zookeeperActive || !redisActive {
		reqLogger.Info("Dependencies not ready", "db", cassandraActive, "zk", zookeeperActive, "rmq", rabbitmqActive, "redis", redisActive)
		return reconcile.Result{}, nil
	}

	servicePortsMap := map[int32]string{
		int32(v1alpha1.AnalyticsApiPort): "analytics",
	}
	analyticsService := r.Kubernetes.Service(request.Name+"-"+instanceType, corev1.ServiceTypeClusterIP, servicePortsMap, instanceType, instance)

	if err := analyticsService.EnsureExists(); err != nil {
		reqLogger.Error(err, "Analytics service doesnt exist")
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
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance); err != nil {
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

		case "analyticsapi":
			if container.Command == nil {
				command := []string{"bash", "-c", instance.CommonStartupScript(
					"exec /usr/bin/contrail-analytics-api -c /etc/contrailconfigmaps/analyticsapi.${POD_IP} -c /etc/contrailconfigmaps/contrail-keystone-auth.conf.${POD_IP}",
					map[string]string{
						"analyticsapi.${POD_IP}":                "",
						"contrail-keystone-auth.conf.${POD_IP}": "",
						"vnc_api_lib.ini.${POD_IP}":             "vnc_api_lib.ini",
					}),
				}
				container.Command = command
			}

		case "collector":
			if container.Command == nil {
				command := []string{"bash", "-c", instance.CommonStartupScript(
					"exec /usr/bin/contrail-collector --conf_file /etc/contrailconfigmaps/collector.${POD_IP}",
					map[string]string{
						"collector.${POD_IP}":       "",
						"vnc_api_lib.ini.${POD_IP}": "vnc_api_lib.ini",
					}),
				}
				container.Command = command
			}

		case "nodemanager":
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/analytics-nodemanager-runner.sh"}
				container.Command = command
			}

		case "provisioner":
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/analytics-provisioner.sh"}
				container.Command = command
			}
		}
	}

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec, instance.Spec.CommonConfiguration)
	v1alpha1.DefaultSecurityContext(&statefulSet.Spec.Template.Spec)

	if created, err := v1alpha1.CreateServiceSTS(instance, instanceType, statefulSet, r.Client); err != nil || created {
		if err != nil {
			reqLogger.Error(err, "Failed to create the stateful set.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, err
	}

	if updated, err := v1alpha1.UpdateServiceSTS(instance, instanceType, statefulSet, false, r.Client); err != nil || updated {
		if err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update the stateful set.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(request, r.Client)
	if err != nil {
		reqLogger.Error(err, "Failed to get pod ip list from instance.")
		return reconcile.Result{}, err
	}
	if updated, err := v1alpha1.UpdatePodsAnnotations(podIPList, r.Client); updated || err != nil {
		if err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update pods annotations.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	if len(podIPMap) > 0 {
		if err := v1alpha1.EnsureCertificatesExist(instance, podIPList, instanceType, r.Client, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to ensure CertificatesExist")
			return reconcile.Result{}, err
		}

		data, err := instance.InstanceConfiguration(podIPList, r.Client)
		if err != nil {
			reqLogger.Error(err, "Failed to get config data.")
			return reconcile.Result{}, err
		}
		if err = v1alpha1.UpdateConfigMap(instance, instanceType, instance.Spec.CommonConfiguration.AuthParameters, data, r.Client); err != nil {
			reqLogger.Error(err, "Failed to update config map.")
			return reconcile.Result{}, err
		}

		if updated, err := instance.ManageNodeStatus(podIPMap, r.Client); err != nil || updated {
			if err != nil && !v1alpha1.IsOKForRequeque(err) {
				reqLogger.Error(err, "Failed to manage node status")
				return reconcile.Result{}, err
			}
			return requeueReconcile, nil
		}
	}

	if instance.Status.Endpoint != analyticsService.ClusterIP() {
		if err = instance.SetEndpointInStatus(r.Client, analyticsService.ClusterIP()); err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to set endpointIn status")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	falseVal := false
	if instance.Status.ConfigChanged == nil {
		instance.Status.ConfigChanged = &falseVal
	}
	beforeCheck := *instance.Status.ConfigChanged
	newConfigMap := &corev1.ConfigMap{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, newConfigMap); err != nil {
		reqLogger.Error(err, "Failed to get the config map.")
		return reconcile.Result{}, err
	}
	*instance.Status.ConfigChanged = !v1alpha1.CmpConfigMaps(configMap, newConfigMap)

	if *instance.Status.ConfigChanged {
		reqLogger.Info("Update StatefulSet: ConfigChanged")
		if _, err := v1alpha1.UpdateServiceSTS(instance, instanceType, statefulSet, true, r.Client); err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Update StatefulSet failed")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	if beforeCheck != *instance.Status.ConfigChanged {
		reqLogger.Info("Update Status: ConfigChanged")
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Update Status failed")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	instance.Status.Active = new(bool)
	instance.Status.Degraded = new(bool)
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, instance.Status.Degraded, statefulSet, request); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			return requeueReconcile, nil
		}
		reqLogger.Error(err, "Failed to set instance active")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Done")
	return reconcile.Result{}, nil
}
