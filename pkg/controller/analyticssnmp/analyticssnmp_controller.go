package analyticssnmp

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	configtemplates "github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/Juniper/contrail-operator/pkg/certificates"
	"github.com/Juniper/contrail-operator/pkg/controller/utils"
	"github.com/Juniper/contrail-operator/pkg/k8s"
	"github.com/go-logr/logr"
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
)

// InstanceType is a string value for AnalyticsSnmp
var InstanceType = "analyticssnmp"

// Log is a default logger for AnalyticsSnmp
var Log = logf.Log.WithName("controller_" + InstanceType)

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.AnalyticsSnmpList{}
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
			list := &v1alpha1.AnalyticsSnmpList{}
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
			list := &v1alpha1.AnalyticsSnmpList{}
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
			list := &v1alpha1.AnalyticsSnmpList{}
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

// Add adds the AnalyticsSnmp controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAnalyticsSnmp{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Manager:    mgr,
		Kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
}
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New(InstanceType+"-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AnalyticsSnmp.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.AnalyticsSnmp{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	serviceMap := map[string]string{"contrail_manager": InstanceType}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predInitStatus := utils.PodInitStatusChange(serviceMap)
	predPodIPChange := utils.PodIPChange(serviceMap)
	predInitRunning := utils.PodInitRunning(serviceMap)

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AnalyticsSnmp{},
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

	cassandraServiceMap := map[string]string{"contrail_manager": "cassandra"}
	predCassandraPodIPChange := utils.PodIPChange(cassandraServiceMap)
	if err = c.Watch(srcPod, podHandler, predCassandraPodIPChange); err != nil {
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
		OwnerType:    &v1alpha1.AnalyticsSnmp{},
	}
	stsPred := utils.STSStatusChange(utils.ConfigGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileAnalyticsSnmp implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileAnalyticsSnmp{}

// ReconcileAnalyticsSnmp reconciles a AnalyticsSnmp object.
type ReconcileAnalyticsSnmp struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles AnalyticsSnmp.
func (r *ReconcileAnalyticsSnmp) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling AnalyticsSnmp")

	// Get instance
	instance := &v1alpha1.AnalyticsSnmp{}
	if err := r.Client.Get(context.TODO(), request.NamespacedName, instance); err != nil && errors.IsNotFound(err) {
		reqLogger.Error(err, "Instance not found.")
		return reconcile.Result{}, nil
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("Instance is deleting, skip reconcile.")
		return reconcile.Result{}, nil
	}

	// Wait until cassandra, zookeeper, rabbitmq and config be active
	cassandraInstance := v1alpha1.Cassandra{}
	zookeeperInstance := v1alpha1.Zookeeper{}
	rabbitmqInstance := v1alpha1.Rabbitmq{}
	configInstance := v1alpha1.Config{}
	cassandraActive := cassandraInstance.IsActive(instance.Spec.ServiceConfiguration.CassandraInstance,
		request.Namespace, r.Client)
	zookeeperActive := zookeeperInstance.IsActive(instance.Spec.ServiceConfiguration.ZookeeperInstance,
		request.Namespace, r.Client)
	rabbitmqActive := rabbitmqInstance.IsActive(instance.Labels["contrail_cluster"],
		request.Namespace, r.Client)
	configActive := configInstance.IsActive(instance.Labels["contrail_cluster"],
		request.Namespace, r.Client)
	if !cassandraActive || !zookeeperActive || !rabbitmqActive || !configActive {
		reqLogger.Info(fmt.Sprintf("%t %t %t %t", cassandraActive, zookeeperActive, rabbitmqActive, configActive))
		return reconcile.Result{}, nil
	}

	// Get or create configmaps
	configMap, isConfigMapCreated, err := r.GetOrCreateConfigMap(FullName("configmap", request), instance, request)
	if err != nil {
		reqLogger.Error(err, "ConfigMap not created.")
		return reconcile.Result{}, err
	}

	_, err = v1alpha1.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request, InstanceType, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Get Statefulset
	statefulSet, err := v1alpha1.QuerySTS(FullName("statefulset", request), request.Namespace, r.Client)
	if err != nil {
		reqLogger.Error(err, "StatefulSet not found.")
		return reconcile.Result{}, err
	}

	// Stateful set does not exist
	if statefulSet == nil {
		statefulSet, err = r.GetStatefulSet(request, instance, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}
		v1alpha1.CreateSTS(statefulSet, InstanceType, request, r.Client)

		return reconcile.Result{Requeue: true}, nil
	}

	// Get pods
	podIpList, podIpMap, err := v1alpha1.PodIPListAndIPMapFromInstance(InstanceType,
		&instance.Spec.CommonConfiguration,
		request,
		r.Client,
		true, false, false, false, false,
	)
	if err != nil {
		reqLogger.Error(err, "Pod list not found")
		return reconcile.Result{}, err
	}
	if len(podIpMap) <= 0 {
		reqLogger.Info("No pods.")
		return reconcile.Result{Requeue: true}, nil
	}

	// Fill config map or check if it changed to update statefulset
	if isConfigMapCreated {
		reqLogger.Info("Configmap just created")
		return reconcile.Result{Requeue: true}, nil
	}

	// Get needed data
	dataForConfigMap, err := instance.GetDataForConfigMap(podIpList, request, r.Client, reqLogger)
	if err != nil {
		reqLogger.Error(err, "Cannot get data for the ConfigMap.")
	}
	// Check data changed
	configChanged := false
	for file, content := range configMap.Data {
		neededContent, found := dataForConfigMap[file]
		if found && neededContent != content {
			configChanged = true
			break
		}
	}
	// Update satefulset if config was changed
	if configChanged {
		statefulSet, err = r.GetStatefulSet(request, instance, reqLogger)
		if err != nil {
			return reconcile.Result{}, nil
		}
		if err = r.Client.Update(context.TODO(), statefulSet); err != nil {
			reqLogger.Error(err, "Update statefulset failed")
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Ensure certificates exists
	certSubjects := v1alpha1.PodsCertSubjects(podIpList,
		instance.Spec.CommonConfiguration.HostNetwork,
		v1alpha1.PodAlternativeIPs{},
	)
	crt := certificates.NewCertificate(r.Client, r.Scheme, instance, certSubjects, InstanceType)
	if err := crt.EnsureExistsAndIsSigned(); err != nil {
		reqLogger.Error(err, "Certificates for pod not exist.")
		return reconcile.Result{Requeue: true}, nil
	}

	// Add new config files
	if !reflect.DeepEqual(configMap.Data, dataForConfigMap) {
		// Update configmap
		configMap.Data = dataForConfigMap
		if err := r.Client.Update(context.TODO(), configMap); err != nil {
			reqLogger.Error(err, "Cannot update the ConfigMap.")
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// Set pod `status` label to `ready`
	if err = v1alpha1.SetPodsToReady(podIpList, r.Client); err != nil {
		reqLogger.Error(err, "Failed to set pods to ready")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// FullName ...
func FullName(name string, request reconcile.Request) string {
	return request.Name + "-" + InstanceType + "-" + name
}

// GetOrCreateConfigMap ...
func (r *ReconcileAnalyticsSnmp) GetOrCreateConfigMap(name string,
	instance *v1alpha1.AnalyticsSnmp,
	request reconcile.Request,
) (configMap *corev1.ConfigMap, isCreated bool, err error) {

	configMap, isCreated, err = v1alpha1.GetOrCreateConfigMap(name,
		r.Client,
		r.Scheme,
		request,
		InstanceType,
		instance,
	)
	return
}

// GetStatefulSet ...
func (r *ReconcileAnalyticsSnmp) GetStatefulSet(request reconcile.Request, instance *v1alpha1.AnalyticsSnmp, reqLogger logr.Logger) (*appsv1.StatefulSet, error) {
	// Get basic stateful set
	statefulSet, err := GetStatefulsetFromYaml()
	if err != nil {
		reqLogger.Error(err, "Cant load the stateful set from yaml.")
		return nil, err
	}

	// Add common configuration to stateful set
	if err := v1alpha1.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, InstanceType, request, r.Scheme, instance, true); err != nil {
		reqLogger.Error(err, "Cant prepare the stateful set.")
		return nil, err
	}

	// Add volumes to stateful set
	v1alpha1.AddVolumesToIntendedSTS(statefulSet, map[string]string{
		FullName("configmap", request):     FullName("volume", request),
		certificates.SignerCAConfigMapName: request.Name + "-csr-signer-ca",
	})
	v1alpha1.AddSecretVolumesToIntendedSTS(statefulSet, map[string]string{
		request.Name + "-secret-certificates": request.Name + "-secret-certificates",
	})

	// Don't know what is it
	statefulSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{{
						Key:      InstanceType,
						Operator: "In",
						Values:   []string{request.Name},
					}},
				},
				TopologyKey: "kubernetes.io/hostname",
			}},
		},
	}

	// Manual settings for containers
	for idx := range statefulSet.Spec.Template.Spec.Containers {
		container := &statefulSet.Spec.Template.Spec.Containers[idx]
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		if instanceContainer == nil {
			reqLogger.Info(fmt.Sprintf("There is no %s container in the manifect", container.Name))
			continue
		}

		// Add image from manifest
		container.Image = instanceContainer.Image

		// Add volume mounts to container
		volumeMountList := []corev1.VolumeMount{}
		if len(container.VolumeMounts) > 0 {
			volumeMountList = container.VolumeMounts
		}
		volumeMount := corev1.VolumeMount{
			Name:      FullName("volume", request),
			MountPath: "/etc/contrailconfigmaps",
		}
		volumeMountList = append(volumeMountList, volumeMount)
		volumeMount = corev1.VolumeMount{
			Name:      request.Name + "-secret-certificates",
			MountPath: "/etc/certificates",
		}
		volumeMountList = append(volumeMountList, volumeMount)
		volumeMount = corev1.VolumeMount{
			Name:      request.Name + "-csr-signer-ca",
			MountPath: certificates.SignerCAMountPath,
		}
		volumeMountList = append(volumeMountList, volumeMount)
		container.VolumeMounts = volumeMountList

		if container.Name == "analytics-snmp-collector" {
			if instanceContainer.Command == nil {
				container.Command = []string{"bash", "-c", "ln -sf /etc/contrailconfigmaps/vncini.${POD_IP} /etc/contrail/vnc_api_lib.ini; /usr/bin/tf-snmp-collector -c /etc/contrailconfigmaps/tf-snmp-collector.${POD_IP} --device-config-file /etc/contrail/device.ini"}
			} else {
				container.Command = instanceContainer.Command
			}
		}

		if container.Name == "analytics-snmp-topology" {
			if instanceContainer.Command == nil {
				container.Command = []string{"bash", "-c", "ln -sf /etc/contrailconfigmaps/vncini.${POD_IP} /etc/contrail/vnc_api_lib.ini; /usr/bin/tf-topology -c /etc/contrailconfigmaps/tf-topology.${POD_IP}"}
			} else {
				container.Command = instanceContainer.Command
			}
		}

		if container.Name == "nodemanager" {
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					"ln -sf /etc/contrailconfigmaps/vncini.${POD_IP} /etc/contrail/vnc_api_lib.ini; " +
						"ln -sf /etc/contrailconfigmaps/nodemanager.${POD_IP} /etc/contrail/contrail-analytics-snmp-nodemgr.conf; " +
						"exec /usr/bin/contrail-nodemgr --nodetype=contrail-analytics-snmp",
				}
				container.Command = command
			} else {
				container.Command = instanceContainer.Command
			}
		}

		if container.Name == "provisioner" {
			if instanceContainer.Command != nil {
				container.Command = instanceContainer.Command
			}

			envList := []corev1.EnvVar{}
			if len(container.Env) > 0 {
				envList = container.Env
			}
			envList = append(envList, corev1.EnvVar{
				Name:  "SSL_ENABLE",
				Value: "True",
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_CA_CERTFILE",
				Value: certificates.SignerCAFilepath,
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_CERTFILE",
				Value: "/etc/certificates/server-$(POD_IP).crt",
			})
			envList = append(envList, corev1.EnvVar{
				Name:  "SERVER_KEYFILE",
				Value: "/etc/certificates/server-key-$(POD_IP).pem",
			})

			configNodesInformation, err := v1alpha1.NewConfigClusterConfiguration(instance.Labels["contrail_cluster"], request.Namespace, r.Client)
			if err != nil {
				reqLogger.Error(err, "Cant get configNodesInformation")
				return nil, err
			}
			configNodeList := configNodesInformation.APIServerIPList
			envList = append(envList, corev1.EnvVar{
				Name:  "CONFIG_NODES",
				Value: configtemplates.JoinListWithSeparator(configNodeList, ","),
			})
			container.Env = envList
		}
	}

	return statefulSet, nil
}
