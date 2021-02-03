package kubemanager

import (
	"context"
	"reflect"

	"github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/Juniper/contrail-operator/pkg/certificates"

	"github.com/Juniper/contrail-operator/pkg/controller/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
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

var log = logf.Log.WithName("controller_kubemanager")

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.KubemanagerList{}
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
			list := &v1alpha1.KubemanagerList{}
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
			list := &v1alpha1.KubemanagerList{}
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
			list := &v1alpha1.KubemanagerList{}
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

// Add creates a new Kubemanager Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, ci v1alpha1.KubemanagerClusterInfo) error {
	return add(mgr, newReconciler(mgr, ci))
}

func newReconciler(mgr manager.Manager, ci v1alpha1.KubemanagerClusterInfo) reconcile.Reconciler {
	return NewReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), ci)
}

// NewReconciler returns a new reconcile.Reconciler.
func NewReconciler(client client.Client, scheme *runtime.Scheme, cfg *rest.Config, ci v1alpha1.KubemanagerClusterInfo) reconcile.Reconciler {
	return &ReconcileKubemanager{Client: client, Scheme: scheme,
		Config: cfg, clusterInfo: ci}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New("kubemanager-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Kubemanager.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Kubemanager{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to PODs.
	serviceMap := map[string]string{"contrail_manager": "kubemanager"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predInitStatus := utils.PodInitStatusChange(serviceMap)
	predPodIPChange := utils.PodIPChange(serviceMap)
	predInitRunning := utils.PodInitRunning(serviceMap)

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

	srcConfig := &source.Kind{Type: &v1alpha1.Config{}}
	configHandler := resourceHandler(mgr.GetClient())
	predConfigSizeChange := utils.ConfigActiveChange()
	if err = c.Watch(srcConfig, configHandler, predConfigSizeChange); err != nil {
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
	err = c.Watch(srcZookeeper, zookeeperHandler, predZookeeperSizeChange)
	if err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Kubemanager{},
	}
	stsPred := utils.STSStatusChange(utils.KubemanagerGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKubemanager implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileKubemanager{}

// ReconcileKubemanager reconciles a Kubemanager object.
type ReconcileKubemanager struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client      client.Client
	Scheme      *runtime.Scheme
	Config      *rest.Config
	clusterInfo v1alpha1.KubemanagerClusterInfo
}

// Reconcile reads that state of the cluster for a Kubemanager object and makes changes based on the state read
// and what is in the Kubemanager.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKubemanager) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Kubemanager")
	instanceType := "kubemanager"
	instance := &v1alpha1.Kubemanager{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	currentConfigMap, currentConfigExists := instance.CurrentConfigMapExists(request.Name+"-"+instanceType+"-configmap", r.Client, r.Scheme, request)

	configMap, err := instance.CreateConfigMap(request.Name+"-"+instanceType+"-configmap", r.Client, r.Scheme, request)
	if err != nil {
		log.Error(err, "CreateConfigMap failed")
		return reconcile.Result{}, err
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		log.Error(err, "CreateSecret failed")
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	if err = instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		log.Error(err, "PrepareSTS failed")
		return reconcile.Result{}, err
	}

	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMap.Name:                     request.Name + "-" + instanceType + "-volume",
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})
	instance.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

	var serviceAccountName string
	if instance.Spec.ServiceConfiguration.ServiceAccount != "" {
		serviceAccountName = instance.Spec.ServiceConfiguration.ServiceAccount
	} else {
		serviceAccountName = "contrail-kubemanager-service-account"
	}

	var clusterRoleName string
	if instance.Spec.ServiceConfiguration.ClusterRole != "" {
		clusterRoleName = instance.Spec.ServiceConfiguration.ClusterRole
	} else {
		clusterRoleName = "contrail-kubemanager-cluster-role"
	}

	var clusterRoleBindingName string
	if instance.Spec.ServiceConfiguration.ClusterRoleBinding != "" {
		clusterRoleBindingName = instance.Spec.ServiceConfiguration.ClusterRoleBinding
	} else {
		clusterRoleBindingName = "contrail-kubemanager-cluster-role-binding"
	}

	var secretName string
	if instance.Spec.ServiceConfiguration.SecretName != "" {
		secretName = instance.Spec.ServiceConfiguration.SecretName
	} else {
		secretName = "contrail-kubemanager-secret"
	}

	if err = instance.EnsureServiceAccount(serviceAccountName, clusterRoleName, clusterRoleBindingName, secretName, r.Client, r.Scheme); err != nil {
		log.Error(err, "EnsureServiceAccount failed")
		return reconcile.Result{}, err
	}

	statefulSet.Spec.Template.Spec.ServiceAccountName = serviceAccountName
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
		if container.Name == "kubemanager" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vnc.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"exec /usr/bin/contrail-kube-manager -c /etc/contrailconfigmaps/kubemanager.${POD_IP}"}
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&statefulSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}

			volumeMountList := []corev1.VolumeMount{}
			if len((&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts
			}
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-" + instanceType + "-volume",
				MountPath: "/etc/contrailconfigmaps",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
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
		}
	}

	for idx, container := range statefulSet.Spec.Template.Spec.InitContainers {
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
		if instanceContainer.Command != nil {
			(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Command = instanceContainer.Command
		}
	}

	configChanged := false
	if instance.Status.ConfigChanged != nil {
		configChanged = *instance.Status.ConfigChanged
	}

	if err = instance.CreateSTS(statefulSet, instanceType, request, r.Client); err != nil {
		return reconcile.Result{}, err
	}

	strategy := "rolling"
	if err = instance.UpdateSTS(statefulSet, instanceType, request, r.Client, strategy); err != nil {
		log.Error(err, "UpdateSTS failed")
		return reconcile.Result{}, err
	}

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
	if err != nil {
		log.Error(err, "PodIPListAndIPMapFromInstance failed")
		return reconcile.Result{}, err
	}
	if len(podIPList.Items) > 0 {
		if err = instance.InstanceConfiguration(request, podIPList, r.Client, r.clusterInfo); err != nil {
			log.Error(err, "InstanceConfiguration failed")
			return reconcile.Result{}, err
		}

		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			log.Error(err, "ensureCertificatesExist failed")
			return reconcile.Result{}, err
		}

		if err = instance.SetPodsToReady(podIPList, r.Client); err != nil {
			log.Error(err, "SetPodsToReady failed")
			return reconcile.Result{}, err
		}

		if err = instance.ManageNodeStatus(podIPMap, r.Client); err != nil {
			log.Error(err, "ManageNodeStatus failed")
			return reconcile.Result{}, err
		}
	}

	if currentConfigExists {
		newConfigMap := &corev1.ConfigMap{}
		_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: request.Name + "-" + instanceType + "-configmap", Namespace: request.Namespace}, newConfigMap)
		if !reflect.DeepEqual(currentConfigMap.Data, newConfigMap.Data) {
			configChanged = true
		} else {
			configChanged = false
		}
		instance.Status.ConfigChanged = &configChanged
	}

	if instance.Status.Active == nil {
		active := false
		instance.Status.Active = &active
	}
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, statefulSet, request); err != nil {
		log.Error(err, "SetInstanceActive failed")
		return reconcile.Result{}, err
	}
	if instance.Status.ConfigChanged != nil {
		if *instance.Status.ConfigChanged {
			return reconcile.Result{Requeue: true}, nil
		}
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileKubemanager) ensureCertificatesExist(config *v1alpha1.Kubemanager, pods *corev1.PodList, instanceType string) error {
	subjects := config.PodsCertSubjects(pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, config, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
