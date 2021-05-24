package analyticsalarm

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"text/template"
	"time"

	"github.com/go-logr/logr"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
	"github.com/tungstenfabric/tf-operator/pkg/randomstring"
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

// InstanceType is a string value for AnalyticsAlarm
var instanceType = "analyticsalarm"

// Log is a default logger for AnalyticsAlarm
var log = logf.Log.WithName("controller_" + instanceType)
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.AnalyticsAlarmList{}
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
			list := &v1alpha1.AnalyticsAlarmList{}
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
			list := &v1alpha1.AnalyticsAlarmList{}
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
			list := &v1alpha1.AnalyticsAlarmList{}
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

// Add adds the AnalyticsAlarm controller to the manager.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAnalyticsAlarm{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		Manager:    mgr,
		Kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
}
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.
	c, err := controller.New(instanceType+"-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource AnalyticsAlarm.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.AnalyticsAlarm{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}
	serviceMap := map[string]string{"tf_manager": instanceType}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predPodIPChange := utils.PodIPChange(serviceMap)

	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AnalyticsAlarm{},
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

	srcRedis := &source.Kind{Type: &v1alpha1.Redis{}}
	redisHandler := resourceHandler(mgr.GetClient())
	predRedisSizeChange := utils.RedisActiveChange()
	if err = c.Watch(srcRedis, redisHandler, predRedisSizeChange); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.AnalyticsAlarm{},
	}
	stsPred := utils.STSStatusChange(utils.ConfigGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileAnalyticsAlarm implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileAnalyticsAlarm{}

// ReconcileAnalyticsAlarm reconciles a AnalyticsAlarm object.
type ReconcileAnalyticsAlarm struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client     client.Client
	Scheme     *runtime.Scheme
	Manager    manager.Manager
	Kubernetes *k8s.Kubernetes
}

// Reconcile reconciles AnalyticsAlarm.
func (r *ReconcileAnalyticsAlarm) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling AnalyticsAlarm")

	// Get instance
	instance := &v1alpha1.AnalyticsAlarm{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		reqLogger.Info("Instance is deleting, skip reconcile.")
		return reconcile.Result{}, nil
	}

	// Wait until cassandra, zookeeper, rabbitmq, redis and config and analytics be active
	cassandraInstance := v1alpha1.Cassandra{}
	zookeeperInstance := v1alpha1.Zookeeper{}
	rabbitmqInstance := v1alpha1.Rabbitmq{}
	redisInstance := v1alpha1.Redis{}
	configInstance := v1alpha1.Config{}
	analyticsInstance := v1alpha1.Analytics{}
	cassandraActive := cassandraInstance.IsActive(instance.Spec.ServiceConfiguration.CassandraInstance, request.Namespace, r.Client)
	zookeeperActive := zookeeperInstance.IsActive(instance.Spec.ServiceConfiguration.ZookeeperInstance, request.Namespace, r.Client)
	rabbitmqActive := rabbitmqInstance.IsActive(instance.Spec.ServiceConfiguration.RabbitmqInstance, request.Namespace, r.Client)
	redisActive := redisInstance.IsActive(instance.Spec.ServiceConfiguration.RedisInstance, request.Namespace, r.Client)
	configActive := configInstance.IsActive(instance.Spec.ServiceConfiguration.ConfigInstance, request.Namespace, r.Client)
	analyticsActive := analyticsInstance.IsActive(instance.Spec.ServiceConfiguration.AnalyticsInstance, request.Namespace, r.Client)
	if !cassandraActive || !zookeeperActive || !rabbitmqActive || !redisActive || !configActive || !analyticsActive {
		reqLogger.Info("Dependencies not ready", "db", cassandraActive, "zk", zookeeperActive, "rmq", rabbitmqActive, "redis", redisActive, "api", configActive, "analytics", analyticsActive)
		return reconcile.Result{}, nil
	}

	// Get or create configmaps
	configMapName := request.Name + "-" + instanceType + "-configmap"
	configMap, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request)
	if err != nil {
		return reconcile.Result{}, err
	}

	_, err = v1alpha1.CreateSecret(request.Name+"-secret-certificates", r.Client, r.Scheme, request, instanceType, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	statefulSet, err := r.GetSTS(request, instance, reqLogger)
	if err != nil {
		return reconcile.Result{}, nil
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec,
		instanceType, instance.Spec.CommonConfiguration.ImagePullSecrets,
		r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec)
	v1alpha1.DefaultSecurityContext(&statefulSet.Spec.Template.Spec)

	if created, err := instance.CreateSTS(statefulSet, instanceType, request, r.Client); err != nil || created {
		if err != nil {
			reqLogger.Error(err, "Failed to create the stateful set.")
			return reconcile.Result{}, err
		}
		return requeueReconcile, err
	}

	if updated, err := instance.UpdateSTS(statefulSet, instanceType, request, r.Client); err != nil || updated {
		if err != nil && !v1alpha1.IsOKForRequeque(err) {
			reqLogger.Error(err, "Failed to update the stateful set")
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
	if err != nil {
		reqLogger.Error(err, "Pod list not found")
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

		if err = r.ensureCertificatesExist(instance, podIPList, instanceType); err != nil {
			reqLogger.Error(err, "Failed to ensure certificates exist.")
			return reconcile.Result{}, err
		}

		if err := instance.InstanceConfiguration(configMapName, podIPList, request, r.Client); err != nil {
			reqLogger.Error(err, "InstanceConfiguration failed")
			return reconcile.Result{}, err
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
	*instance.Status.ConfigChanged = !reflect.DeepEqual(configMap.Data, newConfigMap.Data)

	if *instance.Status.ConfigChanged {
		reqLogger.Info("Update StatefulSet: ConfigChanged")
		if err := r.Client.Update(context.TODO(), statefulSet); err != nil && !v1alpha1.IsOKForRequeque(err) {
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

	instance.Status.Active = &falseVal
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, statefulSet, request); err != nil && !v1alpha1.IsOKForRequeque(err) {
		if v1alpha1.IsOKForRequeque(err) {
			return requeueReconcile, nil
		}
		reqLogger.Error(err, "SetInstanceActive failed")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Done")
	return reconcile.Result{}, nil
}

func (r *ReconcileAnalyticsAlarm) ensureCertificatesExist(instance *v1alpha1.AnalyticsAlarm, pods []corev1.Pod, instanceType string) error {
	domain, err := v1alpha1.ClusterDNSDomain(r.Client)
	if err != nil {
		return err
	}
	subjects := instance.PodsCertSubjects(domain, pods)
	crt := certificates.NewCertificate(r.Client, r.Scheme, instance, subjects, instanceType)
	return crt.EnsureExistsAndIsSigned()
}

// FullName ...
func FullName(name string, request reconcile.Request) string {
	return request.Name + "-" + instanceType + "-" + name
}

// GetSTS prepare STS object for creation
func (r *ReconcileAnalyticsAlarm) GetSTS(request reconcile.Request, instance *v1alpha1.AnalyticsAlarm, reqLogger logr.Logger) (*appsv1.StatefulSet, error) {
	// Get basic stateful set
	statefulSet, err := GetStatefulsetFromYaml()
	if err != nil {
		reqLogger.Error(err, "Cant load the stateful set from yaml.")
		return nil, err
	}

	// Add common configuration to stateful set
	if err := v1alpha1.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, instanceType, request, r.Scheme, instance, true); err != nil {
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
						Key:      instanceType,
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

		if instanceContainer.Command != nil {
			container.Command = instanceContainer.Command
		}

		container.Image = instanceContainer.Image

		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      FullName("volume", request),
				MountPath: "/etc/contrailconfigmaps",
			},
			corev1.VolumeMount{
				Name:      request.Name + "-secret-certificates",
				MountPath: "/etc/certificates",
			},
			corev1.VolumeMount{
				Name:      request.Name + "-csr-signer-ca",
				MountPath: certificates.SignerCAMountPath,
			},
		)

		if container.Name == "analytics-alarm-gen" {
			if container.Command == nil {
				command := []string{"bash", "-c", instance.CommonStartupScript(
					"exec /usr/bin/contrail-alarm-gen -c /etc/contrailconfigmaps/tf-alarm-gen.${POD_IP}",
					map[string]string{
						"tf-alarm-gen.${POD_IP}":    "",
						"vnc_api_lib.ini.${POD_IP}": "vnc_api_lib.ini",
					}),
				}
				container.Command = command
			}
		}

		if container.Name == "kafka" {
			secret, err := instance.CreateSecret(request.Name+"-secret", r.Client, r.Scheme, request)
			if err != nil {
				reqLogger.Error(err, "Cannot create Secret")
				return nil, err
			}
			_, KPok := secret.Data["keystorePassword"]
			_, TPok := secret.Data["truststorePassword"]
			if !KPok || !TPok {
				secret.Data = map[string][]byte{
					"keystorePassword":   []byte(randomstring.RandString{Size: 10}.Generate()),
					"truststorePassword": []byte(randomstring.RandString{Size: 10}.Generate()),
				}
				if err = r.Client.Update(context.TODO(), secret); err != nil {
					reqLogger.Error(err, "Cannot update secret")
					return nil, err
				}
			}
			kafkaKeystorePassword := string(secret.Data["keystorePassword"])
			kafkaTruststorePassword := string(secret.Data["truststorePassword"])
			var kafkaInitKeystoreCommandBuffer bytes.Buffer
			err = kafkaInitKeystoreCommandTemplate.Execute(&kafkaInitKeystoreCommandBuffer, kafkaInitKeystoreCommandData{
				KeystorePassword:   kafkaKeystorePassword,
				TruststorePassword: kafkaTruststorePassword,
				CAFilePath:         certificates.SignerCAFilepath,
			})
			if err != nil {
				panic(err)
			}

			if container.Command == nil {
				command := []string{"bash", "-c", instance.CommonStartupScript(
					kafkaInitKeystoreCommandBuffer.String()+
						"bin/kafka-server-start.sh /etc/contrailconfigmaps/kafka.config.${POD_IP}",
					map[string]string{
						"kafka.config.${POD_IP}": "",
					}),
				}
				container.Command = command
			}
		}

		if container.Name == "nodemanager" {
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/analytics-alarm-nodemanager-runner.sh"}
				container.Command = command
			}
		}

		if container.Name == "provisioner" {
			if container.Command == nil {
				command := []string{"bash", "/etc/contrailconfigmaps/analytics-alarm-provisioner.sh"}
				container.Command = command
			}
		}
	}

	return statefulSet, nil
}

var kafkaInitKeystoreCommandTemplate = template.Must(template.New("").Parse(
	"keytool -keystore /opt/kafka/kafka.server.truststore.jks -keypass {{ .KeystorePassword }} -storepass {{ .TruststorePassword }} -noprompt  -alias CARoot -import -file {{ .CAFilePath }} && " +
		"openssl pkcs12 -export -in /etc/certificates/server-${POD_IP}.crt -inkey /etc/certificates/server-key-${POD_IP}.pem -chain -CAfile {{ .CAFilePath }} -password pass:{{ .TruststorePassword }} -name localhost -out TmpFile && " +
		"keytool -importkeystore -deststorepass {{ .KeystorePassword }} -destkeypass {{ .KeystorePassword }} -destkeystore /opt/kafka/kafka.server.keystore.jks -srcstorepass {{ .TruststorePassword }} -srckeystore TmpFile -srcstoretype PKCS12 -alias localhost && " +
		"keytool -keystore /opt/kafka/kafka.server.keystore.jks -keypass {{ .KeystorePassword }} -storepass {{ .KeystorePassword }} -noprompt -alias CARoot -import -file {{ .CAFilePath }} ; "))

type kafkaInitKeystoreCommandData struct {
	KeystorePassword   string
	TruststorePassword string
	CAFilePath         string
}
