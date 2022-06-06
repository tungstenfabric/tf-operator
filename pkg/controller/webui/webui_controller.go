package webui

import (
	"context"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

var log = logf.Log.WithName("controller_webui")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.WebuiList{}
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
			list := &v1alpha1.WebuiList{}
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
			list := &v1alpha1.WebuiList{}
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
			list := &v1alpha1.WebuiList{}
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

// Add creates a new Webui Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler.
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWebui{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Manager:    mgr,
		Kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("webui-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Webui.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Webui{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	serviceMap := map[string]string{"tf_manager": "webui"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Webui{},
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

	srcConfig := &source.Kind{Type: &v1alpha1.Config{}}
	configHandler := resourceHandler(mgr.GetClient())
	predConfigSizeChange := utils.ConfigActiveChange()
	if err = c.Watch(srcConfig, configHandler, predConfigSizeChange); err != nil {
		return err
	}

	srcControl := &source.Kind{Type: &v1alpha1.Control{}}
	controlHandler := resourceHandler(mgr.GetClient())
	predControlSizeChange := utils.ControlActiveChange()
	if err = c.Watch(srcControl, controlHandler, predControlSizeChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Webui{},
	}
	stsPred := utils.STSStatusChange(utils.WebuiGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileWebui implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileWebui{}

// ReconcileWebui reconciles a Webui object.
type ReconcileWebui struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reads that state of the cluster for a Webui object and makes changes based on the state read
// and what is in the Webui.Spec.
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example.
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWebui) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Webui")
	instanceType := "webui"
	instance := &v1alpha1.Webui{}
	configInstance := v1alpha1.Config{}
	controlInstance := v1alpha1.Control{}
	cassandraInstance := v1alpha1.Cassandra{}

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

	webuiService := r.Kubernetes.Service(request.Name+"-"+instanceType, corev1.ServiceTypeClusterIP, map[int32]string{int32(v1alpha1.WebuiHttpsListenPort): ""}, instanceType, instance)
	if err := webuiService.EnsureExists(); err != nil {
		return reconcile.Result{}, err
	}

	cassandraActive := cassandraInstance.IsActive(instance.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, r.Client)
	configActive := configInstance.IsActive(instance.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, r.Client)
	controlActive := controlInstance.IsActive(instance.Spec.ServiceConfiguration.ControlInstance, request.Namespace, r.Client)
	if !configActive || !cassandraActive || !controlActive {
		reqLogger.Info("Dependencies not ready", "db", cassandraActive, "api", configActive, "control", controlActive)
		return reconcile.Result{}, nil
	}

	configMap, err := instance.CreateConfigMap(request.Name+"-"+instanceType+"-configmap", r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
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

	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		configMap.Name:                     request.Name + "-" + instanceType + "-volume",
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})

	instance.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

	for idx, container := range statefulSet.Spec.Template.Spec.Containers {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "ANALYTICS_ALARM_ENABLE",
				Value: strconv.FormatBool(true),
			},
			corev1.EnvVar{
				Name:  "ANALYTICS_SNMP_ENABLE",
				Value: strconv.FormatBool(true),
			},
			corev1.EnvVar{
				Name:  "ANALYTICSDB_ENABLE",
				Value: strconv.FormatBool(true),
			},
		)
		if container.Name == "webuiweb" {
			command := []string{"bash", "-c", instance.CommonStartupScript(
				// use copy as webui resolves symlinks just to "..data/config.global.js.10.0.0.206"
				// instead of resolve like
				//    readlink -e /etc/contrailconfigmaps/config.global.js.10.0.0.206
				//    /etc/contrailconfigmaps/..2021_02_28_17_21_52.558864405/config.global.js.10.0.0.206
				"rm -rf /etc/contrail; mkdir -p /etc/contrail; "+
					"cp /etc/contrailconfigmaps/config.global.js.${POD_IP} /etc/contrail/config.global.js; "+
					"cp /etc/contrailconfigmaps/contrail-webui-userauth.js.${POD_IP} /etc/contrail/contrail-webui-userauth.js; "+
					"exec /usr/bin/node /usr/src/contrail/contrail-web-core/webServerStart.js",
				map[string]string{
					"config.global.js.${POD_IP}":           "",
					"contrail-webui-userauth.js.${POD_IP}": "",
				}),
			}

			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
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
			readinessProbe := corev1.Probe{
				FailureThreshold: 3,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Scheme: corev1.URISchemeHTTPS,
						Path:   "/",
						Port:   intstr.IntOrString{IntVal: int32(v1alpha1.WebuiHttpsListenPort)},
					},
				},
			}
			startUpProbe := corev1.Probe{
				FailureThreshold: 30,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Scheme: corev1.URISchemeHTTPS,
						Path:   "/",
						Port:   intstr.IntOrString{IntVal: int32(v1alpha1.WebuiHttpsListenPort)},
					},
				},
			}
			(&statefulSet.Spec.Template.Spec.Containers[idx]).ReadinessProbe = &readinessProbe
			(&statefulSet.Spec.Template.Spec.Containers[idx]).StartupProbe = &startUpProbe
		}
		if container.Name == "webuijob" {
			command := []string{"bash", "-c", instance.CommonStartupScript(
				// use copy as webui resolves symlinks just to "..data/config.global.js.10.0.0.206"
				// instead of resolve like
				//    readlink -e /etc/contrailconfigmaps/config.global.js.10.0.0.206
				//    /etc/contrailconfigmaps/..2021_02_28_17_21_52.558864405/config.global.js.10.0.0.206
				"rm -rf /etc/contrail; mkdir -p /etc/contrail; "+
					"cp /etc/contrailconfigmaps/config.global.js.${POD_IP} /etc/contrail/config.global.js; "+
					"cp /etc/contrailconfigmaps/contrail-webui-userauth.js.${POD_IP} /etc/contrail/contrail-webui-userauth.js; "+
					"exec /usr/bin/node /usr/src/contrail/contrail-web-core/jobServerStart.js",
				map[string]string{
					"config.global.js.${POD_IP}":           "",
					"contrail-webui-userauth.js.${POD_IP}": "",
				}),
			}

			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
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
		if container.Name == "redis" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"exec redis-server --lua-time-limit 15000 --dbfilename '' --bind 127.0.0.1 --port 6380",
				}
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
			(&statefulSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList
			(&statefulSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image
			readinessProbe := corev1.Probe{
				FailureThreshold: 3,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h 127.0.0.1 -p 6380 ping"},
					},
				},
			}
			startUpProbe := corev1.Probe{
				FailureThreshold: 30,
				PeriodSeconds:    3,
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"sh", "-c", "redis-cli -h 127.0.0.1 -p 6380 ping"},
					},
				},
			}
			(&statefulSet.Spec.Template.Spec.Containers[idx]).ReadinessProbe = &readinessProbe
			(&statefulSet.Spec.Template.Spec.Containers[idx]).StartupProbe = &startUpProbe
		}
	}

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec)
	v1alpha1.DefaultSecurityContext(&statefulSet.Spec.Template.Spec)

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
		log.Error(err, "PodIPListAndIPMapFromInstance failed")
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
		// TODO: Services can be run on masters only, ensure that pods number is
		if nodes, err := v1alpha1.GetControllerNodes(r.Client); err != nil || len(podIPList) < len(nodes) {
			// to avoid redundand sts-es reloading configure only as STS pods are ready
			reqLogger.Error(err, "Not enough pods are ready to generate configs %v < %v", len(podIPList), len(nodes))
			return requeueReconcile, err
		}

		if err = instance.InstanceConfiguration(request, podIPList, r.Client); err != nil {
			log.Error(err, "InstanceConfiguration failed")
			return reconcile.Result{}, err
		}

		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			log.Error(err, "ensureCertificatesExist failed")
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

	if err = r.updateStatus(instance, statefulSet, webuiService.ClusterIP()); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			return requeueReconcile, nil
		}
		log.Error(err, "Failed to update status.")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileWebui) updateStatus(cr *v1alpha1.Webui, sts *appsv1.StatefulSet, cip string) error {
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace},
		sts); err != nil {
		return err
	}
	active := sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	degraded := sts.Status.ReadyReplicas < *sts.Spec.Replicas
	cr.Status.Active = &active
	cr.Status.Degraded = &degraded
	r.updatePorts(cr)
	if err := r.updateServiceStatus(cr); err != nil {
		return err
	}
	cr.Status.Endpoint = cip
	return r.Client.Status().Update(context.Background(), cr)
}

func (r *ReconcileWebui) updatePorts(cr *v1alpha1.Webui) {
	cr.Status.Ports.WebUIHttpPort = v1alpha1.WebuiHttpListenPort
	cr.Status.Ports.WebUIHttpsPort = v1alpha1.WebuiHttpsListenPort
}

func (r *ReconcileWebui) updateServiceStatus(cr *v1alpha1.Webui) error {
	pods, err := r.listWebUIPods(cr.Name)
	if err != nil {
		return err
	}
	serviceStatuses := map[string]v1alpha1.WebUIServiceStatusMap{}
	for _, pod := range pods {
		podStatus := v1alpha1.WebUIServiceStatusMap{}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			status := "Non-Functional"
			if containerStatus.Ready {
				status = "Functional"
			}
			podStatus[strings.Title(containerStatus.Name)] = v1alpha1.WebUIServiceStatus{ModuleName: containerStatus.Name, ModuleState: status}
		}
		serviceStatuses[pod.Spec.NodeName] = podStatus
	}
	cr.Status.ServiceStatus = serviceStatuses
	return nil
}

func (r *ReconcileWebui) listWebUIPods(webUIName string) ([]corev1.Pod, error) {
	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"tf_manager": "webui", "webui": webUIName})
	listOpts := client.ListOptions{LabelSelector: labelSelector}
	if err := r.Client.List(context.TODO(), pods, &listOpts); err != nil {
		log.Error(err, "listWebUIPods failed")
		return nil, err
	}
	res := []corev1.Pod{}
	for _, pod := range pods.Items {
		if pod.Status.PodIP == "" || pod.Status.Phase != "Running" {
			continue
		}
		res = append(res, pod)
	}
	return res, nil
}

func (r *ReconcileWebui) ensureCertificatesExist(instance *v1alpha1.Webui, pods []corev1.Pod, instanceType string) error {
	domain, err := v1alpha1.ClusterDNSDomain(r.Client)
	if err != nil {
		return err
	}
	subjects := instance.PodsCertSubjects(domain, pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, instance, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
