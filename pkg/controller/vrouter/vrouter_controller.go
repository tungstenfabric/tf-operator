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
	core "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/controller/utils"
)

var log = logf.Log.WithName("controller_vrouter")

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
	serviceMap := map[string]string{"contrail_manager": "vrouter"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())

	predInitStatus := utils.PodInitStatusChange(serviceMap)
	if err = c.Watch(srcPod, podHandler, predInitStatus); err != nil {
		return err
	}

	predPodIPChange := utils.PodIPChange(serviceMap)
	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}

	predInitRunning := utils.PodInitRunning(serviceMap)
	if err = c.Watch(srcPod, podHandler, predInitRunning); err != nil {
		return err
	}

	serviceMap = map[string]string{"contrail_manager": "control"}
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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Vrouter")
	instanceType := "vrouter"
	instance := &v1alpha1.Vrouter{}

	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		return reconcile.Result{}, nil
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

	configMapName := request.Name + "-vrouter-agent-config"
	configMapAgent, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	configMapName = request.Name + "-secret-certificates"
	secretCertificates, err := instance.CreateSecret(configMapName, r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	daemonSet := GetDaemonset()
	if err = instance.PrepareDaemonSet(daemonSet, &instance.Spec.CommonConfiguration, request, r.Scheme, r.Client); err != nil {
		return reconcile.Result{}, err
	}

	csrSignerCaVolumeName := request.Name + "-csr-signer-ca"
	instance.AddVolumesToIntendedDS(daemonSet, map[string]string{
		configMapAgent.Name:                request.Name + "-agent-volume",
		cniConfigMap.Name:                  cniConfigMap.Name + "-cni-volume",
		certificates.SignerCAConfigMapName: csrSignerCaVolumeName,
	})
	instance.AddSecretVolumesToIntendedDS(daemonSet, map[string]string{secretCertificates.Name: request.Name + "-secret-certificates"})

	for idx, container := range daemonSet.Spec.Template.Spec.Containers {
		if container.Name == "vrouteragent" {
			command := []string{"bash", "-c", `mkdir -p /var/log/contrail/vrouter-agent;
				ln -sf /etc/agentconfigmaps/contrail-vrouter-agent.conf.${POD_IP} /etc/contrail/contrail-vrouter-agent.conf;
				ln -sf /etc/agentconfigmaps/contrail-lbaas.auth.conf.${POD_IP} /etc/contrail/contrail-lbaas.auth.conf;
				ln -sf /etc/agentconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini;
				source /etc/agentconfigmaps/params.env;
				source /actions.sh;
				prepare_agent;
				start_agent;
				wait $(cat /var/run/vrouter-agent.pid)`}
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer == nil {
				instanceContainer = utils.GetContainerFromList(container.Name, v1alpha1.DefaultVrouter.Containers)
			}
			if instanceContainer.Command == nil {
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := []corev1.VolumeMount{}
			if len((&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts
			}
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      request.Name + "-agent-volume",
				MountPath: v1alpha1.VrouterAgentConfigMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList

			(&daemonSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

			envFromList := []corev1.EnvFromSource{}
			if len((&daemonSet.Spec.Template.Spec.Containers[idx]).EnvFrom) > 0 {
				envFromList = (&daemonSet.Spec.Template.Spec.Containers[idx]).EnvFrom
			}
			envFromList = append(envFromList, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEnv.Name,
					},
				},
			})
			(&daemonSet.Spec.Template.Spec.Containers[idx]).EnvFrom = envFromList
		}

		if container.Name == "nodemanager" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/agentconfigmaps/nodemanager.conf.${POD_IP} /etc/contrail/contrail-vrouter-nodemgr.conf ;" +
						"ln -sf /etc/agentconfigmaps/vnc_api_lib.ini.${POD_IP} /etc/contrail/vnc_api_lib.ini ; " +
						"exec /usr/bin/contrail-nodemgr --nodetype=contrail-vrouter",
				}
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Command = command
			} else {
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}
			volumeMountList := []corev1.VolumeMount{}
			if len((&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts
			}
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      request.Name + "-agent-volume",
				MountPath: v1alpha1.VrouterAgentConfigMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList

			(&daemonSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

			(&daemonSet.Spec.Template.Spec.Containers[idx]).EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEnv.Name,
					},
				},
			}}
		}

		if container.Name == "provisioner" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer.Command != nil {
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Command = instanceContainer.Command
			}

			volumeMountList := []corev1.VolumeMount{}
			if len((&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts
			}
			volumeMountList = append(volumeMountList, corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			})
			volumeMountList = append(volumeMountList, corev1.VolumeMount{
				Name:      csrSignerCaVolumeName,
				MountPath: certificates.SignerCAMountPath,
			})
			(&daemonSet.Spec.Template.Spec.Containers[idx]).VolumeMounts = volumeMountList

			(&daemonSet.Spec.Template.Spec.Containers[idx]).Image = instanceContainer.Image

			(&daemonSet.Spec.Template.Spec.Containers[idx]).EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEnv.Name,
					},
				},
			}}

			envList := []corev1.EnvVar{
				{
					Name:  "SSL_ENABLE",
					Value: "True",
				},
				{
					Name:  "SERVER_CA_CERTFILE",
					Value: certificates.SignerCAFilepath,
				},
				{
					Name:  "SERVER_CERTFILE",
					Value: "/etc/certificates/server-$(POD_IP).crt",
				},
				{
					Name:  "SERVER_KEYFILE",
					Value: "/etc/certificates/server-key-$(POD_IP).pem",
				},
				{
					Name:  "CONFIG_NODES",
					Value: instance.GetConfigNodes(r.Client),
				},
				{
					Name:  "CLOUD_ORCHESTRATOR",
					Value: instance.VrouterConfigurationParameters().CloudOrchestrator,
				},
			}
			(&daemonSet.Spec.Template.Spec.Containers[idx]).Env = append(
				(&daemonSet.Spec.Template.Spec.Containers[idx]).Env, envList...)
		}
	}

	ubuntu := v1alpha1.UBUNTU
	for idx, container := range daemonSet.Spec.Template.Spec.InitContainers {
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer == nil {
			instanceContainer = utils.GetContainerFromList(container.Name, v1alpha1.DefaultVrouter.Containers)
		}
		(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
		if instanceContainer.Command != nil {
			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Command = instanceContainer.Command
		}
		if container.Name == "vrouterkernelinit" {
			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEnv.Name,
					},
				},
			}}
			if instance.Spec.ServiceConfiguration.Distribution != nil || instance.Spec.ServiceConfiguration.Distribution == &ubuntu {
				(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
			}
		}

		if container.Name == "vroutercni" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer == nil {
				instanceContainer = utils.GetContainerFromList(container.Name, v1alpha1.DefaultVrouter.Containers)
			}
			if instanceContainer.Command == nil {
				// vroutercni container command is based on the entrypoint.sh script in the contrail-kubernetes-cni-init container
				command := []string{"sh", "-c",
					"mkdir -p /host/etc_cni/net.d && " +
						"mkdir -p /var/lib/contrail/ports/vm && " +
						"cp -f /usr/bin/contrail-k8s-cni /host/opt_cni_bin && " +
						"chmod 0755 /host/opt_cni_bin/contrail-k8s-cni && " +
						"cp -f /etc/cniconfigmaps/10-tf-cni.conf /host/etc_cni/net.d/10-tf-cni.conf && " +
						"tar -C /host/opt_cni_bin -xzf /opt/cni-v0.3.0.tgz"}
				(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Command = command
			} else {
				(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Command = instanceContainer.Command
			}

			volumeMountList := []corev1.VolumeMount{}
			if len((&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts
			}
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			volumeMount = corev1.VolumeMount{
				Name:      cniConfigMap.Name + "-cni-volume",
				MountPath: "/etc/cniconfigmaps",
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts = volumeMountList

			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image

			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).EnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapEnv.Name,
					},
				},
			}}
		}

		if container.Name == "multusconfig" {
			instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
			if instanceContainer == nil {
				instanceContainer = utils.GetContainerFromList(container.Name, v1alpha1.DefaultVrouter.Containers)
			}
			volumeMountList := []corev1.VolumeMount{}
			if len((&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts) > 0 {
				volumeMountList = (&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts
			}
			volumeMount := corev1.VolumeMount{
				Name:      request.Name + "-agent-volume",
				MountPath: v1alpha1.VrouterAgentConfigMountPath,
			}
			volumeMountList = append(volumeMountList, volumeMount)
			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).VolumeMounts = volumeMountList

			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
		}

		if container.Name == "nodeinit" && instance.Spec.ServiceConfiguration.ContrailStatusImage != "" {
			(&daemonSet.Spec.Template.Spec.InitContainers[idx]).Env = append((&daemonSet.Spec.Template.Spec.InitContainers[idx]).Env,
				core.EnvVar{
					Name:  "CONTRAIL_STATUS_IMAGE",
					Value: instance.Spec.ServiceConfiguration.ContrailStatusImage,
				},
			)
		}
	}

	instance.SetParamsToAgents(request, r.Client)

	if err = instance.CreateDS(daemonSet, &instance.Spec.CommonConfiguration, instanceType, request,
		r.Scheme, r.Client); err != nil {
		return reconcile.Result{}, err
	}

	if err = instance.UpdateDS(daemonSet, &instance.Spec.CommonConfiguration, instanceType, request, r.Scheme, r.Client); err != nil {
		return reconcile.Result{}, err
	}
	getPhysicalInterface := false
	if instance.Spec.ServiceConfiguration.PhysicalInterface == "" {
		getPhysicalInterface = true
	}
	getGateway := false
	if instance.Spec.ServiceConfiguration.Gateway == "" {
		getGateway = true
	}
	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client, getPhysicalInterface, true, true, getGateway)
	if err != nil {
		return reconcile.Result{}, err
	}
	if len(podIPMap) > 0 {
		if err := r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			return reconcile.Result{}, err
		}

		if err = instance.SetPodsToReady(podIPList, r.Client); err != nil {
			return reconcile.Result{}, err
		}

		if err = instance.ManageNodeStatus(podIPMap, r.Client); err != nil {
			return reconcile.Result{}, err
		}
	}

	nodes := instance.GetAgentNodes(daemonSet, r.Client)
	reconcileAgain := false
	for _, node := range nodes.Items {
		pod := instance.GetNodeDSPod(node.Name, daemonSet, r.Client)
		if pod == nil {
			continue
		}

		vrouterPod := &v1alpha1.VrouterPod{pod}

		agentStatus := instance.LookupAgentStatus(node.Name)
		if agentStatus == nil {
			agentStatus := &v1alpha1.AgentStatus{
				Name:            node.Name,
				Status:          "Starting",
				EncryptedParams: v1alpha1.EncryptString(instance.GetParamsEnv(r.Client)),
			}
			instance.Status.Agents = append(instance.Status.Agents, agentStatus)
			if err := instance.SaveClusterStatus(node.Name, r.Client); err != nil {
				return reconcile.Result{}, err
			}
		}

		agentContainerStatus, err := vrouterPod.GetAgentContainerStatus()
		if err != nil || agentContainerStatus.State.Running == nil {
			resetStatus(instance, node.Name)
			continue
		}

		if pod.Status.Phase != "Running" || !vrouterPod.IsAgentRunning() {
			resetStatus(instance, node.Name)
		}

		hostVars := make(map[string]string)
		if err := vrouterPod.GetAgentParameters(&hostVars); err != nil {
			reconcileAgain = true
			continue
		}

		if agentStatus.Status != "Updating" {
			if err := instance.UpdateAgentConfigMapForPod(vrouterPod, &hostVars, r.Client); err != nil {
				reconcileAgain = true
				continue
			}
		}

		if agentStatus.Status == "Starting" {
			if vrouterPod.IsAgentRunning() {
				agentStatus.Status = "Ready"
			} else {
				reconcileAgain = true
				continue
			}
		}

		if agentStatus.Status == "Ready" {
			if agentStatus.EncryptedParams != v1alpha1.EncryptString(instance.GetParamsEnv(r.Client)) ||
				instance.IsClusterChanged(node.Name, r.Client) {
				agentStatus.Status = "Updating"
			}
		}

		if agentStatus.Status == "Updating" {
			instance.UpdateAgent(node.Name, vrouterPod, r.Client, &reconcileAgain)
		}
	}

	if reconcileAgain == true {
		restartTime, _ := time.ParseDuration("3s")
		return reconcile.Result{Requeue: true, RequeueAfter: restartTime}, nil
	}

	if instance.Status.Active == nil {
		active := false
		instance.Status.Active = &active
	}

	isControllerActive, err := instance.IsActiveOnControllers(r.Client)
	instance.Status.ActiveOnControllers = &isControllerActive
	if err != nil {
		return reconcile.Result{}, err
	}

	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, daemonSet, request, instance); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileVrouter) ensureCertificatesExist(vrouter *v1alpha1.Vrouter, pods *corev1.PodList, instanceType string) error {
	subjects := vrouter.PodsCertSubjects(pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, vrouter, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}

func resetStatus(instance *v1alpha1.Vrouter, nodeName string) {
	agentStatus := instance.LookupAgentStatus(nodeName)
	if agentStatus.Status == "Ready" || agentStatus.Status == "Updating" {
		agentStatus.Status = "Starting"
	}
}
