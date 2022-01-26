package control

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"

	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"

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

var log = logf.Log.WithName("controller_control")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ControlList{}
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
			list := &v1alpha1.ControlList{}
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
			list := &v1alpha1.ControlList{}
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
			list := &v1alpha1.ControlList{}
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

// Add creates a new Control Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileControl{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Manager: mgr}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("control-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Control
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Control{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	ownerHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Control{},
	}

	if err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, ownerHandler); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Node{}}, nodeChangeHandler(mgr.GetClient())); err != nil {
		return err
	}

	// Watch for changes to PODs
	serviceMap := map[string]string{"tf_manager": "control"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

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

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsPred := utils.STSStatusChange(utils.ControlGroupKind())
	if err = c.Watch(srcSTS, ownerHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileControl implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileControl{}

// ReconcileControl reconciles a Control object
type ReconcileControl struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client  client.Client
	Scheme  *runtime.Scheme
	Manager manager.Manager
}

// Reconcile reconciles control resource
func (r *ReconcileControl) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Control")
	instanceType := "control"

	// Check ZIU status
	f, err := v1alpha1.CanReconcile("Control", r.Client)
	if err != nil {
		log.Error(err, "When check control ziu status")
		return reconcile.Result{}, err
	}
	if !f {
		log.Info("control reconcile blocks by ZIU status")
		return reconcile.Result{Requeue: true, RequeueAfter: v1alpha1.ZiuRestartTime}, nil
	}

	instance := &v1alpha1.Control{}
	cassandraInstance := v1alpha1.Cassandra{}
	rabbitmqInstance := v1alpha1.Rabbitmq{}
	configInstance := v1alpha1.Config{}

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

	rabbitmqActive := rabbitmqInstance.IsActive(v1alpha1.RabbitmqInstance, request.Namespace, r.Client)
	cassandraActive := cassandraInstance.IsActive(v1alpha1.CassandraInstance, request.Namespace, r.Client)
	configActive := configInstance.IsActive(v1alpha1.ConfigInstance, request.Namespace, r.Client)
	if !configActive || !cassandraActive || !rabbitmqActive {
		reqLogger.Info("Dependencies not ready", "db", cassandraActive, "rmq", rabbitmqActive, "api", configActive)
		return reconcile.Result{}, nil
	}

	configMapName := request.Name + "-" + instanceType + "-configmap"
	configMap, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "CreateConfigMap failed")
		return reconcile.Result{}, err
	}

	_, err = instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	if err = instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	configmapsVolumeName := request.Name + "-" + instanceType + "-volume"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMapName: configmapsVolumeName,
	})

	v1alpha1.AddCAVolumeToIntendedSTS(statefulSet)
	v1alpha1.AddSecretVolumesToIntendedSTS(statefulSet, request.Name)

	if instance.Spec.ServiceConfiguration.DataSubnet != "" {
		if statefulSet.Spec.Template.ObjectMeta.Annotations == nil {
			statefulSet.Spec.Template.ObjectMeta.Annotations = map[string]string{"dataSubnet": instance.Spec.ServiceConfiguration.DataSubnet}
		} else {
			statefulSet.Spec.Template.ObjectMeta.Annotations["dataSubnet"] = instance.Spec.ServiceConfiguration.DataSubnet
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
			})
		v1alpha1.AddCertsMounts(request.Name, container)
		v1alpha1.SetLogLevelEnv(instance.Spec.CommonConfiguration.LogLevel, container)

		container.Image = instanceContainer.Image

		if container.Command == nil {
			command := []string{"bash", fmt.Sprintf("/etc/contrailconfigmaps/run-%s.sh", container.Name)}
			container.Command = command
		}

		switch container.Name {
		case "provisioner":
			cfg := instance.ConfigurationParameters()
			container.Env = append(container.Env,
				corev1.EnvVar{
					Name:  "BGP_ASN",
					Value: strconv.Itoa(*cfg.ASNNumber),
				})
			if cfg.Subcluster != "" {
				container.Env = append(container.Env,
					corev1.EnvVar{
						Name:  "SUBCLUSTER",
						Value: instance.Spec.ServiceConfiguration.Subcluster,
					})
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

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
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
		// TODO: Services can be run on masters only, ensure that pods number is
		nodeselector := instance.Spec.CommonConfiguration.NodeSelector
		if nodes, err := v1alpha1.GetNodes(nodeselector, r.Client); err != nil || len(podIPList) < len(nodes) {
			// to avoid redundand sts-es reloading configure only as STS pods are ready
			reqLogger.Error(err, "Not enough pods are ready to generate configs (pods < nodes)", "pods", len(podIPList), "nodes", len(nodes))
			return requeueReconcile, err
		}

		if err = v1alpha1.EnsureCertificatesExist(instance, podIPList, instanceType, r.Client, r.Scheme); err != nil {
			reqLogger.Error(err, "Failed to ensure certificates exist.")
			return reconcile.Result{}, err
		}

		data, err := instance.InstanceConfiguration(podIPList, r.Client)
		if err != nil {
			reqLogger.Error(err, "Failed to get config data.")
			return reconcile.Result{}, err
		}
		if err = v1alpha1.UpdateConfigMap(instance, instanceType, data, r.Client); err != nil {
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

	falseVal := false
	if instance.Status.ConfigChanged == nil {
		instance.Status.ConfigChanged = &falseVal
	}
	beforeCheck := *instance.Status.ConfigChanged
	newConfigMap := &corev1.ConfigMap{}
	if err = r.Client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, newConfigMap); err != nil {
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
		reqLogger.Error(err, "Failed to set instance active.")
		return reconcile.Result{}, err
	}

	if !*instance.Status.Active {
		reqLogger.Info("Not Active => requeue reconcile")
		return requeueReconcile, nil
	}

	reqLogger.Info("Done")
	return reconcile.Result{}, nil
}
