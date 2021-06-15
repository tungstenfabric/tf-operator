package redis

import (
	"context"
	"time"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"

	"github.com/tungstenfabric/tf-operator/pkg/certificates"
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

var log = logf.Log.WithName("controller_redis")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.RedisList{}
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
			list := &v1alpha1.RedisList{}
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
			list := &v1alpha1.RedisList{}
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
			list := &v1alpha1.RedisList{}
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

// Add adds Redis controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	kubernetes := k8s.New(mgr.GetClient(), mgr.GetScheme())
	return &ReconcileRedis{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Manager: mgr, Kubernetes: kubernetes}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.

	c, err := controller.New("redis-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// Watch for changes to primary resource Redis.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Redis{}},
		&handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to PODs.
	serviceMap := map[string]string{"tf_manager": "redis"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Redis{},
	}
	stsPred := utils.STSStatusChange(utils.RedisGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileRedis implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileRedis{}

// ReconcileRedis reconciles a Redis object.
type ReconcileRedis struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles Redis
func (r *ReconcileRedis) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Start")
	instanceType := "redis"

	// Check ZIU status
	f, err := v1alpha1.CanReconcile("Redis", r.Client)
	if err != nil {
		log.Error(err, "When check redis ziu status")
		return reconcile.Result{}, err
	}
	if !f {
		log.Info("redis reconcile blocks by ZIU status")
		return reconcile.Result{Requeue: true, RequeueAfter: v1alpha1.ZiuRestartTime}, nil
	}

	instance := &v1alpha1.Redis{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		reqLogger.Error(err, "Failed to get redis obj")
		return reconcile.Result{}, nil
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

	utils.CleanupContainers(&statefulSet.Spec.Template.Spec, instance.Spec.ServiceConfiguration.Containers)
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
				command := []string{"bash", "-c", instance.CommonStartupScript(
					"mkdir -p /etc/stunnel /var/run/stunnel; "+
						"cat /etc/certificates/server-key-${POD_IP}.pem /etc/certificates/server-${POD_IP}.crt > /etc/stunnel/private.pem; "+
						"chmod 600 /etc/stunnel/private.pem; "+
						"exec stunnel /etc/contrailconfigmaps/stunnel.${POD_IP}",
					map[string]string{
						"stunnel.${POD_IP}": "",
					}),
				}

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
		if nodes, err := v1alpha1.GetControllerNodes(r.Client); err != nil || len(podIPList) < len(nodes) {
			// to avoid redundand sts-es reloading configure only as STS pods are ready
			reqLogger.Error(err, "Not enough pods are ready to generate configs %v < %v", len(podIPList), len(nodes))
			return requeueReconcile, err
		}

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
