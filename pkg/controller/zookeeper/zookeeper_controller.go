package zookeeper

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/label"

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
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_zookeeper")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileZookeeper{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller

	c, err := controller.New("zookeeper-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Zookeeper{}},
		&handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	serviceMap := map[string]string{"tf_manager": "zookeeper"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Zookeeper{},
	}
	stsPred := utils.STSStatusChange(utils.ZookeeperGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileZookeeper implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileZookeeper{}

// ReconcileZookeeper reconciles a Zookeeper object.
type ReconcileZookeeper struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client client.Client
	Scheme *runtime.Scheme
}

// Reconcile reconciles zookeeper.
func (r *ReconcileZookeeper) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Zookeeper")
	instanceType := "zookeeper"
	instance := &v1alpha1.Zookeeper{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	configMapName := instance.Name + "-" + instanceType + "-configmap"
	if _, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request); err != nil {
		return reconcile.Result{}, err
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		reqLogger.Error(err, "CreateSecret failed")
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	if err := instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMapName:                      request.Name + "-" + instanceType + "-volume",
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})

	instance.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

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
			},
		)

		if container.Name == "zookeeper" {
			if container.Command == nil {
				command := []string{"bash", "-c", instance.CommonStartupScript(
					"zkServer.sh --config /var/lib/zookeeper start-foreground",
					map[string]string{
						"log4j.properties":        "log4j.properties",
						"configuration.xsl":       "configuration.xsl",
						"zoo.cfg":                 "zoo.cfg",
						"zoo.cfg.dynamic.$POD_IP": "zoo.cfg.dynamic",
						"myid.$POD_IP":            "myid",
					}),
				}
				container.Command = command
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

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec)
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

	podIPList, _, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
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

	if len(podIPList) > 0 {
		if err = instance.InstanceConfiguration(request, configMapName, podIPList, r.Client); err != nil {
			reqLogger.Error(err, "Failed to configurate instance.")
			return reconcile.Result{}, err
		}
		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			reqLogger.Error(err, "Failed to ensure certificates exist.")
			return reconcile.Result{}, err
		}
		if requeueNeeded, err := instance.ManageNodeStatus(podIPList, r.Client); err != nil || requeueNeeded {
			if err != nil {
				reqLogger.Error(err, "Failed to manage node status.")
				return reconcile.Result{}, err
			}
			return requeueReconcile, nil
		}
	}

	if err = r.ensurePodDisruptionBudgetExists(instance); err != nil {
		reqLogger.Error(err, "Failed to ensure pod disruption budget exists.")
		return reconcile.Result{}, err
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
	return reconcile.Result{}, nil
}
func (r *ReconcileZookeeper) ensurePodDisruptionBudgetExists(zookeeper *v1alpha1.Zookeeper) error {
	pdb := &policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zookeeper.Name + "-zookeeper",
			Namespace: zookeeper.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, pdb, func() error {
		oneVal := intstr.FromInt(1)
		pdb.ObjectMeta.Labels = label.New("zookeeper", zookeeper.Name)
		pdb.Spec.MaxUnavailable = &oneVal
		pdb.Spec.Selector = metav1.SetAsLabelSelector(label.New("zookeeper", zookeeper.Name))
		return controllerutil.SetControllerReference(zookeeper, pdb, r.Scheme)
	})

	return err
}

func (r *ReconcileZookeeper) ensureCertificatesExist(instance *v1alpha1.Zookeeper, pods []corev1.Pod, instanceType string) error {
	domain, err := v1alpha1.ClusterDNSDomain(r.Client)
	if err != nil {
		return err
	}
	subjects := instance.PodsCertSubjects(domain, pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, instance, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
