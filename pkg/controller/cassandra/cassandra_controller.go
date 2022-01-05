package cassandra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"

	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_cassandra")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.CassandraList{}
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
			list := &v1alpha1.CassandraList{}
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
			list := &v1alpha1.CassandraList{}
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
			list := &v1alpha1.CassandraList{}
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

// Add adds Cassandra controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	kubernetes := k8s.New(mgr.GetClient(), mgr.GetScheme())
	return &ReconcileCassandra{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Manager: mgr, Kubernetes: kubernetes}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.

	c, err := controller.New("cassandra-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// Watch for changes to primary resource Cassandra.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Cassandra{}},
		&handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Cassandra{},
	}

	if err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, ownerHandler); err != nil {
		return err
	}

	// Watch for changes to PODs.
	serviceMap := map[string]string{"tf_manager": "cassandra"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)
	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsPred := utils.STSStatusChange(utils.CassandraGroupKind())
	if err = c.Watch(srcSTS, ownerHandler, stsPred); err != nil {
		return err
	}

	srcConfig := &source.Kind{Type: &v1alpha1.Config{}}
	configHandler := resourceHandler(mgr.GetClient())
	predConfigSizeChange := utils.ConfigActiveChange()
	if err = c.Watch(srcConfig, configHandler, predConfigSizeChange); err != nil {
		return err
	}

	srcAnalytics := &source.Kind{Type: &v1alpha1.Analytics{}}
	analyticsHandler := resourceHandler(mgr.GetClient())
	predAnalyticsSizeChange := utils.AnalyticsActiveChange()
	if err = c.Watch(srcAnalytics, analyticsHandler, predAnalyticsSizeChange); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCassandra implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileCassandra{}

// ReconcileCassandra reconciles a Cassandra object.
type ReconcileCassandra struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles cassandra.
func (r *ReconcileCassandra) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Cassandra")
	instanceType := v1alpha1.CassandraInstanceType

	// Check ZIU status
	f, err := v1alpha1.CanReconcile("Cassandra", r.Client)
	if err != nil {
		log.Error(err, "When check cassandra ziu status")
		return reconcile.Result{}, err
	}
	if !f {
		log.Info("cassandra reconcile blocks by ZIU status")
		return reconcile.Result{Requeue: true, RequeueAfter: v1alpha1.ZiuRestartTime}, nil
	}

	instance := &v1alpha1.Cassandra{}
	err = r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMapName := request.Name + "-" + instanceType + "-configmap"
	if _, err = instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request); err != nil {
		return reconcile.Result{}, err
	}

	cassandraConfig := instance.ConfigurationParameters()

	databaseNodeType, err := v1alpha1.GetDatabaseNodeType(r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if strings.HasPrefix(request.Name, "analyticsdb") {
		databaseNodeType = "database"
	}
	statefulSet := GetSTS(cassandraConfig, databaseNodeType)
	if err = instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	configmapsVolumeName := request.Name + "-" + instanceType + "-volume"
	secretVolumeName := request.Name + "-secret-certificates"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMapName: configmapsVolumeName,
	})

	v1alpha1.AddCAVolumeToIntendedSTS(statefulSet)

	instance.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: secretVolumeName})

	utils.CleanupContainers(&statefulSet.Spec.Template.Spec, instance.Spec.ServiceConfiguration.Containers)
	for idx := range statefulSet.Spec.Template.Spec.Containers {

		container := &statefulSet.Spec.Template.Spec.Containers[idx]

		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer.Command != nil {
			container.Command = instanceContainer.Command
		}

		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      configmapsVolumeName,
				MountPath: "/etc/contrailconfigmaps",
			},
		)
		v1alpha1.AddCertsMounts(request.Name, container)
		v1alpha1.SetLogLevelEnv(instance.Spec.CommonConfiguration.LogLevel, container)

		container.Image = instanceContainer.Image

		if container.Command == nil {
			command := []string{"bash", fmt.Sprintf("/etc/contrailconfigmaps/run-%s.sh", container.Name)}
			container.Command = command
		}

		if container.Name == "cassandra" {
			var jvmOpts string
			if instance.Spec.ServiceConfiguration.MinHeapSize != "" {
				jvmOpts = "-Xms" + instance.Spec.ServiceConfiguration.MinHeapSize
			}
			if instance.Spec.ServiceConfiguration.MaxHeapSize != "" {
				jvmOpts = jvmOpts + " -Xmx" + instance.Spec.ServiceConfiguration.MaxHeapSize
			}
			if jvmOpts != "" {
				container.Env = append(container.Env, corev1.EnvVar{
					Name:  "JVM_EXTRA_OPTS",
					Value: jvmOpts,
				})
			}
		}
	}

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

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec, instance.Spec.CommonConfiguration)
	v1alpha1.DefaultSecurityContext(&statefulSet.Spec.Template.Spec)

	// Create statefulset if it doesn't exist
	if created, err := v1alpha1.CreateServiceSTS(instance, instanceType, statefulSet, r.Client); err != nil || created {
		if err != nil {
			reqLogger.Error(err, "Failed to create the stateful set.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, err
	}

	// Update StatefulSet if changed
	if updated, err := v1alpha1.UpdateServiceSTS(instance, instanceType, statefulSet, false, r.Client); err != nil || updated {
		if err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update the stateful set.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	// Preapare / udpate configmaps if pods are created
	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	if updated, err := v1alpha1.UpdatePodsAnnotations(podIPList, r.Client); updated || err != nil {
		if err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update pods annotations.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	var minPods int32
	if replicas, err := v1alpha1.GetReplicas(r.Client, statefulSet.Spec.Template.Spec.NodeSelector); err != nil {
		return reconcile.Result{}, err
	} else {
		minPods = replicas/2 + 1
	}
	if len(podIPList) >= int(minPods) {
		// TODO: Services can be run on masters only, ensure that pods number is
		if nodes, err := v1alpha1.GetControllerNodes(r.Client); err != nil || len(podIPList) < len(nodes) {
			// to avoid redundand sts-es reloading configure only as STS pods are ready
			reqLogger.Error(err, "Not enough pods are ready to generate configs (pods < nodes)", "pods", len(podIPList), "nodes", len(nodes))
			return requeueReconcile, err
		}

		if err := v1alpha1.EnsureCertificatesExist(instance, podIPList, instanceType, r.Client, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}

		if err = instance.InstanceConfiguration(request, podIPList, r.Client); err != nil {
			return reconcile.Result{}, err
		}
	}

	currentSTS, err := instance.QuerySTS(statefulSet.Name, statefulSet.Namespace, r.Client)
	if err != nil {
		reqLogger.Error(err, "QuerySTS failed")
		return reconcile.Result{}, err
	}
	if instance.UpdateStatus(cassandraConfig, podIPMap, currentSTS) {
		reqLogger.Info("Update Status")
		if err = r.Client.Status().Update(context.TODO(), instance); err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Update Status failed")
			return reconcile.Result{}, err
		}
		reqLogger.Info("End and requeue reconcile")
		return requeueReconcile, nil
	}

	if !*instance.Status.Active {
		reqLogger.Info("Not Active => requeue reconcile")
		return requeueReconcile, nil
	}

	reqLogger.Info("End")
	return reconcile.Result{}, nil
}
