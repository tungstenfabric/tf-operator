package vrouter

import (
	"context"
	"time"

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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
)

var log = logf.Log.WithName("controller_vrouter")
var restartTime, _ = time.ParseDuration("3s")
var reconcileRequeue = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.VrouterList{}
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
			list := &v1alpha1.VrouterList{}
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
			list := &v1alpha1.VrouterList{}
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
			list := &v1alpha1.VrouterList{}
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

// Add creates a new Vrouter Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler.
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return NewReconciler(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig())
}

// NewReconciler returns a new reconcile.Reconciler.
func NewReconciler(client client.Client, scheme *runtime.Scheme, cfg *rest.Config) reconcile.Reconciler {
	return &ReconcileVrouter{Client: client, Scheme: scheme,
		Config: cfg}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New("vrouter-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Vrouter.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Vrouter{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to PODs.
	serviceMap := map[string]string{"tf_manager": "vrouter"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())

	predPodIPChange := utils.PodIPChange(serviceMap)
	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}

	serviceMap = map[string]string{"tf_manager": "control"}
	predPhaseChanges := utils.PodPhaseChanges(serviceMap)
	if err = c.Watch(srcPod, podHandler, predPhaseChanges); err != nil {
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

	srcDS := &source.Kind{Type: &appsv1.DaemonSet{}}
	dsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Vrouter{},
	}
	dsPred := utils.DSStatusChange(utils.VrouterGroupKind())
	if err = c.Watch(srcDS, dsHandler, dsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileVrouter implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileVrouter{}

// ReconcileVrouter reconciles a Vrouter object.
type ReconcileVrouter struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// Reconcile reads that state of the cluster for a Vrouter object and makes changes based on the state read
// and what is in the Vrouter.Spec.
func (r *ReconcileVrouter) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Vrouter")
	instanceType := "vrouter"
	instance := &v1alpha1.Vrouter{}

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

	configMapEnv, err := instance.CreateEnvConfigMap(instanceType, r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	cniConfigMap, err := instance.CreateCNIConfigMap(r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMapAgent, err := instance.CreateConfigMap(request.Name+"-vrouter-agent-config", r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}
	if err = instance.DefaultAgentConfigMapData(configMapAgent, r.Client); err != nil {
		reqLogger.Error(err, "DefaultAgentConfigMapData failed")
		return reconcile.Result{}, err
	}

	secretCertificates, err := instance.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	vcp, err := instance.VrouterConfigurationParameters(r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	analyticsNodes := instance.GetAnalyticsNodes(r.Client)
	configNodes := instance.GetConfigNodes(r.Client)
	controlNodes, err := instance.GetControlNodes(r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}
	reqLogger.Info("Controller nodes", "configNodes", configNodes, "controlNodes", controlNodes, "analyticsNodes", analyticsNodes)

	kcc, err := v1alpha1.ClusterParameters(r.Client)
	if err != nil {
		reqLogger.Error(err, "ClusterParameters failed")
		return reconcile.Result{}, err
	}
	daemonSet := GetDaemonset(
		instance,
		&kcc.Networking.CNIConfig,
		instance.Spec.ServiceConfiguration.StatusImage,
		vcp.CloudOrchestrator)
	if err = instance.PrepareDaemonSet(daemonSet, &instance.Spec.CommonConfiguration, request, r.Scheme, r.Client); err != nil {
		return reconcile.Result{}, err
	}
	err = v1alpha1.EnsureServiceAccount(&daemonSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMapVolumeName := request.Name + "-agent-volume"
	secretVolumeName := secretCertificates.Name
	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	cniVolumeName := cniConfigMap.Name + "-cni-volume"
	instance.AddVolumesToIntendedDS(daemonSet, map[string]string{
		configMapAgent.Name:                configMapVolumeName,
		cniConfigMap.Name:                  cniVolumeName,
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})
	instance.AddSecretVolumesToIntendedDS(daemonSet, map[string]string{secretCertificates.Name: secretVolumeName})

	for idx := range daemonSet.Spec.Template.Spec.Containers {

		container := &daemonSet.Spec.Template.Spec.Containers[idx]
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer.Command != nil {
			container.Command = instanceContainer.Command
		}

		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      configMapVolumeName,
				MountPath: "/etc/contrailconfigmaps",
			},
			corev1.VolumeMount{
				Name:      secretVolumeName,
				MountPath: "/etc/certificates",
			},
			corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			})

		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapEnv.Name,
				},
			},
		})

		container.Image = instanceContainer.Image

		if container.Name == "vrouteragent" {
			if container.Command == nil {
				command := []string{"bash", "-c", v1alpha1.CommonStartupScript(
					"mkdir -p /var/log/contrail/vrouter-agent; "+
						"source /etc/contrailconfigmaps/params.env.${POD_IP}; "+
						"source /actions.sh; "+
						"prepare_agent; "+
						"link_file contrail-vrouter-agent.conf.${POD_IP} contrail-vrouter-agent.conf; "+
						"link_file contrail-lbaas.auth.conf.${POD_IP} contrail-lbaas.auth.conf; "+
						"link_file vnc_api_lib.ini.${POD_IP} vnc_api_lib.ini; "+
						// agent handles errors w/o -e (use of -e leads to un-recovered bind iface on cleanup)
						"set +e; "+
						"start_agent; ",
					map[string]string{"params.env.${POD_IP}": ""}),
				}
				container.Command = command
			}
		}

		if container.Name == "nodemanager" {
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/vrouter-nodemanager-runner.sh"}
				container.Command = command
			}
		}

		if container.Name == "provisioner" {
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/vrouter-provisioner.sh"}
				container.Command = command
			}
		}
	}

	ubuntu := v1alpha1.UBUNTU
	for idx := range daemonSet.Spec.Template.Spec.InitContainers {

		container := &daemonSet.Spec.Template.Spec.InitContainers[idx]
		if instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers); instanceContainer != nil {
			if instanceContainer.Command != nil {
				container.Command = instanceContainer.Command
			}
			container.Image = instanceContainer.Image
		}

		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      configMapVolumeName,
				MountPath: "/etc/contrailconfigmaps",
			},
			corev1.VolumeMount{
				Name:      secretVolumeName,
				MountPath: "/etc/certificates",
			},
			corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			})

		container.EnvFrom = append(container.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapEnv.Name,
				},
			},
		})

		if container.Name == "vrouterkernelinit" {
			if instance.Spec.ServiceConfiguration.Distribution != nil || instance.Spec.ServiceConfiguration.Distribution == &ubuntu {
				if ic := utils.GetContainerFromList("vrouterkernelbuildinit", instance.Spec.ServiceConfiguration.Containers); ic != nil {
					container.Image = ic.Image
				}
			}
		}

		if container.Name == "vroutercni" {
			if container.Command == nil {
				// vroutercni container command is based on the entrypoint.sh script in the contrail-kubernetes-cni-init container
				command := []string{"sh", "-c",
					"mkdir -p /host/etc_cni/net.d && " +
						"mkdir -p /var/lib/contrail/ports/vm && " +
						"cp -f /usr/bin/contrail-k8s-cni /host/opt_cni_bin && " +
						"chmod 0755 /host/opt_cni_bin/contrail-k8s-cni && " +
						"cp -f /etc/cniconfigmaps/10-tf-cni.conf /host/etc_cni/net.d/10-tf-cni.conf && " +
						"tar -C /host/opt_cni_bin -xzf /opt/cni-v0.3.0.tgz && " +
						"mkdir -p /var/run/multus/cni/net.d && " +
						"cp -f /etc/cniconfigmaps/10-tf-cni.conf /var/run/multus/cni/net.d/80-openshift-network.conf",
				}

				container.Command = command
			}
			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      cniVolumeName,
					MountPath: "/etc/cniconfigmaps",
				})
		}

		// nothing to do for nodeinit
		// if container.Name == "nodeinit" {}
	}

	if err = instance.CreateDS(daemonSet, &instance.Spec.CommonConfiguration, instanceType, request,
		r.Scheme, r.Client); err != nil {
		reqLogger.Error(err, "Failed to create the daemon set.")
		return reconcile.Result{}, err
	}

	if err = instance.UpdateDS(daemonSet, &instance.Spec.CommonConfiguration, instanceType, request, r.Scheme, r.Client); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			reqLogger.Info("Faile to update the daemonset, and reconcile is restarting.")
			return reconcileRequeue, nil
		}
		reqLogger.Error(err, "Failed to update the daemon set.")
		return reconcile.Result{}, err
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
		return reconcileRequeue, nil
	}

	if len(podIPMap) > 0 {
		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			reqLogger.Error(err, "Failed to ensure certificates exist.")
			return reconcile.Result{}, err
		}

		if updated, err := instance.ManageNodeStatus(podIPMap, r.Client); err != nil || updated {
			if err != nil && !v1alpha1.IsOKForRequeque(err) {
				reqLogger.Error(err, "Failed to manage node status")
				return reconcile.Result{}, err
			}
			return reconcileRequeue, nil
		}
	}

	nodes := instance.GetAgentNodes(daemonSet, r.Client)
	reconcileAgain := false
	for _, node := range nodes.Items {
		pod := instance.GetNodeDSPod(node.Name, daemonSet, r.Client)
		if pod == nil || pod.Status.PodIP == "" || pod.Status.Phase != "Running" {
			reqLogger.Info("pod is not run yet", "node.Name", node.Name)
			reconcileAgain = true
			continue
		}
		vrouterPod := &v1alpha1.VrouterPod{Pod: pod}
		agentStatus := instance.LookupAgentStatus(node.Name)
		if agentStatus == nil {
			agentStatus = &v1alpha1.AgentStatus{
				Name:            node.Name,
				Status:          "Starting",
				EncryptedParams: "",
			}
			instance.Status.Agents = append(instance.Status.Agents, agentStatus)
			reqLogger.Info("newAgentStatus", "node.Name", node.Name)
		}

		again, _ := instance.UpdateAgent(node.Name, agentStatus, vrouterPod, configMapAgent, r.Client)
		reconcileAgain = reconcileAgain || again
	}

	falseVal := false
	instance.Status.ActiveOnControllers = &falseVal
	isControllerActive, err := instance.IsActiveOnControllers(r.Client)
	if err != nil {
		reqLogger.Error(err, "Failed to know is controller active.")
		return reconcile.Result{}, err
	}
	instance.Status.ActiveOnControllers = &isControllerActive

	// check reconcile after the check IsActiveOnControllers to allow set it if masters are ready
	// but some workers are not
	if reconcileAgain {
		reqLogger.Info("Update Status")
		if err := r.Client.Status().Update(context.TODO(), instance); err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update status.")
			return reconcile.Result{}, err
		}
		return reconcileRequeue, nil
	}

	instance.Status.Active = &falseVal
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, daemonSet, request, instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			return reconcileRequeue, nil
		}
		reqLogger.Error(err, "Failed to set instance active")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVrouter) ensureCertificatesExist(vrouter *v1alpha1.Vrouter, pods []corev1.Pod, instanceType string) error {
	domain, err := v1alpha1.ClusterDNSDomain(r.Client)
	if err != nil {
		return err
	}
	subjects := vrouter.PodsCertSubjects(domain, pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, vrouter, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}
