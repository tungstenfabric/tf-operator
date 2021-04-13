package manager

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

var log = logf.Log.WithName("controller_manager")
var restartTime, _ = time.ParseDuration("3s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

var resourcesList = []runtime.Object{
	&v1alpha1.Analytics{},
	&v1alpha1.QueryEngine{},
	&v1alpha1.AnalyticsSnmp{},
	&v1alpha1.AnalyticsAlarm{},
	&v1alpha1.Cassandra{},
	&v1alpha1.Zookeeper{},
	&v1alpha1.Webui{},
	&v1alpha1.Config{},
	&v1alpha1.Control{},
	&v1alpha1.Rabbitmq{},
	&v1alpha1.Vrouter{},
	&v1alpha1.Kubemanager{},
	&corev1.ConfigMap{},
}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Manager Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	if err := apiextensionsv1beta1.AddToScheme(scheme.Scheme); err != nil {
		return err
	}
	var r reconcile.Reconciler
	reconcileManager := ReconcileManager{client: mgr.GetClient(),
		scheme:     mgr.GetScheme(),
		manager:    mgr,
		cache:      mgr.GetCache(),
		kubernetes: k8s.New(mgr.GetClient(), mgr.GetScheme()),
	}
	r = &reconcileManager
	//r := newReconciler(mgr)
	c, err := createController(mgr, r)
	if err != nil {
		return err
	}
	reconcileManager.controller = c
	return addManagerWatch(c, mgr)
}

func createController(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	c, err := controller.New("manager-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return c, err
	}
	return c, nil
}

func addResourcesToWatch(c controller.Controller, obj runtime.Object) error {
	return c.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Manager{},
	})
}

func addManagerWatch(c controller.Controller, mgr manager.Manager) error {
	err := c.Watch(&source.Kind{Type: &v1alpha1.Manager{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	for _, resource := range resourcesList {
		if err = addResourcesToWatch(c, resource); err != nil {
			return err
		}
	}
	return c.Watch(&source.Kind{Type: &corev1.Node{}}, nodeChangeHandler(mgr.GetClient()))
}

// blank assignment to verify that ReconcileManager implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileManager{}

// ReconcileManager reconciles a Manager object.
type ReconcileManager struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	client     client.Client
	scheme     *runtime.Scheme
	manager    manager.Manager
	controller controller.Controller
	cache      cache.Cache
	kubernetes *k8s.Kubernetes
}

// Reconcile reconciles the manager.
func (r *ReconcileManager) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Manager")
	instance := &v1alpha1.Manager{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	if err := r.processCSRSignerCaConfigMap(instance); err != nil {
		return reconcile.Result{}, err
	}

	nodes, err := r.getControllerNodes()
	if err != nil {
		return reconcile.Result{}, err
	}

	var replicas int32
	if instance.Spec.CommonConfiguration.Replicas != nil {
		replicas = *instance.Spec.CommonConfiguration.Replicas
	} else {
		replicas = r.getReplicas(nodes)
		if replicas == 0 {
			return reconcile.Result{}, nil
		}
	}

	nodesHostAliases := r.getNodesHostAliases(nodes)

	// set defaults if not set
	if instance.Spec.CommonConfiguration.AuthParameters == nil {
		instance.Spec.CommonConfiguration.AuthParameters = &v1alpha1.AuthParameters{}
		instance.Spec.CommonConfiguration.AuthParameters.AuthMode = v1alpha1.AuthenticationModeNoAuth
	}
	if err := instance.Spec.CommonConfiguration.AuthParameters.Prepare(request.Namespace, r.client); err != nil {
		return reconcile.Result{}, err
	}

	var requeueErr error = nil
	if err := r.processVRouters(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processVRouters, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processVRouters")
	}

	if err := r.processRabbitMQ(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processRabbitMQ, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processRabbitMQ")
	}

	if err := r.processCassandras(instance, replicas, nodesHostAliases); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processCassandras, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processCassandras")
	}

	if err := r.processZookeepers(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processZookeepers, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processZookeepers")
	}

	if err := r.processControls(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processControls, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processControls")
	}

	if err := r.processConfig(instance, replicas, nodesHostAliases); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processConfig, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processConfig")
	}

	if err := r.processWebui(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processWebui, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processWebui")
	}

	if err := r.processAnalytics(instance, replicas, nodesHostAliases); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalytics, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalytics")
	}

	if err := r.processQueryEngine(instance, replicas, nodesHostAliases); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processQueryEngine, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processQueryEngine")
	}

	if err := r.processAnalyticsSnmp(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalyticsSnmp, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalyticsSnmp")
	}

	if err := r.processAnalyticsAlarm(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalyticsAlarm, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalyticsAlarm")
	}

	if err := r.processKubemanagers(instance, replicas); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processKubemanagers, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processKubemanagers")
	}

	r.setConditions(instance)
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to update status, and reconcile is restarting.")
			return requeueReconcile, nil
		}
		return reconcile.Result{}, err
	}

	if requeueErr != nil {
		return requeueReconcile, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileManager) setConditions(manager *v1alpha1.Manager) {
	readyStatus := v1alpha1.ConditionFalse
	if manager.IsClusterReady() {
		readyStatus = v1alpha1.ConditionTrue
	}
	manager.Status.Conditions = []v1alpha1.ManagerCondition{{
		Type:   v1alpha1.ManagerReady,
		Status: readyStatus,
	}}
}

func (r *ReconcileManager) getControllerNodes() ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	labels := client.MatchingLabels{"node-role.kubernetes.io/master": ""}
	if err := r.client.List(context.Background(), nodeList, labels); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (r *ReconcileManager) getReplicas(nodes []corev1.Node) int32 {
	nodesNumber := len(nodes)
	if nodesNumber%2 == 0 && nodesNumber != 0 {
		return int32(nodesNumber - 1)
	}
	return int32(nodesNumber)
}

func (r *ReconcileManager) getNodesHostAliases(nodes []corev1.Node) []corev1.HostAlias {
	hostAliases := []corev1.HostAlias{}
	for _, n := range nodes {
		ip := ""
		hostname := ""
		for _, a := range n.Status.Addresses {
			if a.Type == corev1.NodeInternalIP {
				ip = a.Address
			}
			if a.Type == corev1.NodeHostName {
				hostname = a.Address
			}
		}
		if ip == "" || hostname == "" {
			continue
		}
		hostAliases = append(hostAliases, corev1.HostAlias{
			IP:        ip,
			Hostnames: []string{hostname},
		})
	}

	return hostAliases
}

func (r *ReconcileManager) processAnalytics(manager *v1alpha1.Manager, replicas int32, hostAliases []corev1.HostAlias) error {
	if manager.Spec.Services.Analytics == nil {
		if manager.Status.Analytics != nil {
			oldConfig := &v1alpha1.Analytics{}
			oldConfig.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Analytics.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldConfig)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Analytics = nil
		}
		return nil
	}

	analytics := &v1alpha1.Analytics{}
	analytics.ObjectMeta = manager.Spec.Services.Analytics.ObjectMeta
	analytics.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, analytics, func() error {
		analytics.Spec = manager.Spec.Services.Analytics.Spec
		analytics.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analytics.Spec.CommonConfiguration)
		if analytics.Spec.CommonConfiguration.Replicas == nil {
			analytics.Spec.CommonConfiguration.Replicas = &replicas
		}
		if len(analytics.Spec.CommonConfiguration.HostAliases) == 0 {
			analytics.Spec.CommonConfiguration.HostAliases = hostAliases
		}
		return controllerutil.SetControllerReference(manager, analytics, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &analytics.Name
	status.Active = analytics.Status.Active
	manager.Status.Analytics = status
	return nil
}

func (r *ReconcileManager) processQueryEngine(manager *v1alpha1.Manager, replicas int32, hostAliases []corev1.HostAlias) error {
	if manager.Spec.Services.QueryEngine == nil {
		if manager.Status.QueryEngine != nil {
			oldConfig := &v1alpha1.QueryEngine{}
			oldConfig.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.QueryEngine.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldConfig)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.QueryEngine = nil
		}
		return nil
	}

	queryengine := &v1alpha1.QueryEngine{}
	queryengine.ObjectMeta = manager.Spec.Services.QueryEngine.ObjectMeta
	queryengine.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, queryengine, func() error {
		queryengine.Spec = manager.Spec.Services.QueryEngine.Spec
		queryengine.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, queryengine.Spec.CommonConfiguration)
		if queryengine.Spec.CommonConfiguration.Replicas == nil {
			queryengine.Spec.CommonConfiguration.Replicas = &replicas
		}
		if len(queryengine.Spec.CommonConfiguration.HostAliases) == 0 {
			queryengine.Spec.CommonConfiguration.HostAliases = hostAliases
		}
		return controllerutil.SetControllerReference(manager, queryengine, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &queryengine.Name
	status.Active = queryengine.Status.Active
	manager.Status.QueryEngine = status
	return nil
}

func (r *ReconcileManager) processAnalyticsSnmp(manager *v1alpha1.Manager, replicas int32) error {
	if manager.Spec.Services.AnalyticsSnmp == nil {
		if manager.Status.AnalyticsSnmp != nil {
			oldAnalyticsSnmp := &v1alpha1.AnalyticsSnmp{}
			oldAnalyticsSnmp.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.AnalyticsSnmp.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldAnalyticsSnmp)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.AnalyticsSnmp = nil
		}
		return nil
	}

	analyticsSnmp := &v1alpha1.AnalyticsSnmp{}
	analyticsSnmp.ObjectMeta = manager.Spec.Services.AnalyticsSnmp.ObjectMeta
	analyticsSnmp.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, analyticsSnmp, func() error {
		analyticsSnmp.Spec = manager.Spec.Services.AnalyticsSnmp.Spec
		analyticsSnmp.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analyticsSnmp.Spec.CommonConfiguration)
		if analyticsSnmp.Spec.CommonConfiguration.Replicas == nil {
			analyticsSnmp.Spec.CommonConfiguration.Replicas = &replicas
		}
		return controllerutil.SetControllerReference(manager, analyticsSnmp, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &analyticsSnmp.Name
	status.Active = analyticsSnmp.Status.Active
	manager.Status.AnalyticsSnmp = status
	return err
}

func (r *ReconcileManager) processAnalyticsAlarm(manager *v1alpha1.Manager, replicas int32) error {
	if manager.Spec.Services.AnalyticsAlarm == nil {
		if manager.Status.AnalyticsAlarm != nil {
			oldAnalyticsAlarm := &v1alpha1.AnalyticsAlarm{}
			oldAnalyticsAlarm.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.AnalyticsAlarm.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldAnalyticsAlarm)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.AnalyticsAlarm = nil
		}
		return nil
	}

	analyticsAlarm := &v1alpha1.AnalyticsAlarm{}
	analyticsAlarm.ObjectMeta = manager.Spec.Services.AnalyticsAlarm.ObjectMeta
	analyticsAlarm.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, analyticsAlarm, func() error {
		analyticsAlarm.Spec = manager.Spec.Services.AnalyticsAlarm.Spec
		analyticsAlarm.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analyticsAlarm.Spec.CommonConfiguration)
		if analyticsAlarm.Spec.CommonConfiguration.Replicas == nil {
			analyticsAlarm.Spec.CommonConfiguration.Replicas = &replicas
		}
		return controllerutil.SetControllerReference(manager, analyticsAlarm, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &analyticsAlarm.Name
	status.Active = analyticsAlarm.Status.Active
	manager.Status.AnalyticsAlarm = status
	return err
}

func (r *ReconcileManager) processZookeepers(manager *v1alpha1.Manager, replicas int32) error {
	for _, existingZookeeper := range manager.Status.Zookeepers {
		found := false
		for _, intendedZookeeper := range manager.Spec.Services.Zookeepers {
			if *existingZookeeper.Name == intendedZookeeper.Name {
				found = true
				break
			}
		}
		if !found {
			oldZookeeper := &v1alpha1.Zookeeper{}
			oldZookeeper.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingZookeeper.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldZookeeper)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	var zookeeperServiceStatus []*v1alpha1.ServiceStatus
	for _, zookeeperService := range manager.Spec.Services.Zookeepers {
		zookeeper := &v1alpha1.Zookeeper{}
		zookeeper.ObjectMeta = zookeeperService.ObjectMeta
		zookeeper.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, zookeeper, func() error {
			zookeeper.Spec = zookeeperService.Spec
			zookeeper.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, zookeeper.Spec.CommonConfiguration)
			if zookeeper.Spec.CommonConfiguration.Replicas == nil {
				zookeeper.Spec.CommonConfiguration.Replicas = &replicas
			}
			return controllerutil.SetControllerReference(manager, zookeeper, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &zookeeper.Name
		status.Active = zookeeper.Status.Active
		zookeeperServiceStatus = append(zookeeperServiceStatus, status)
	}

	manager.Status.Zookeepers = zookeeperServiceStatus
	return nil
}

func (r *ReconcileManager) processCassandras(manager *v1alpha1.Manager, replicas int32, hostAliases []corev1.HostAlias) error {
	for _, existingCassandra := range manager.Status.Cassandras {
		found := false
		for _, intendedCassandra := range manager.Spec.Services.Cassandras {
			if *existingCassandra.Name == intendedCassandra.Name {
				found = true
				break
			}
		}
		if !found {
			oldCassandra := &v1alpha1.Cassandra{}
			oldCassandra.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingCassandra.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldCassandra)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	cassandraStatusList := []*v1alpha1.ServiceStatus{}
	for _, cassandraService := range manager.Spec.Services.Cassandras {
		cassandra := &v1alpha1.Cassandra{}
		cassandra.ObjectMeta = cassandraService.ObjectMeta
		cassandra.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, cassandra, func() error {
			cassandra.Spec = cassandraService.Spec
			if cassandra.Spec.ServiceConfiguration.ClusterName == "" {
				cassandra.Spec.ServiceConfiguration.ClusterName = manager.GetName()
			}
			cassandra.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, cassandra.Spec.CommonConfiguration)
			if cassandra.Spec.CommonConfiguration.Replicas == nil {
				cassandra.Spec.CommonConfiguration.Replicas = &replicas
			}
			if len(cassandra.Spec.CommonConfiguration.HostAliases) == 0 {
				cassandra.Spec.CommonConfiguration.HostAliases = hostAliases
			}
			return controllerutil.SetControllerReference(manager, cassandra, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &cassandra.Name
		status.Active = cassandra.Status.Active
		cassandraStatusList = append(cassandraStatusList, status)
	}
	manager.Status.Cassandras = cassandraStatusList
	return nil
}

func (r *ReconcileManager) processWebui(manager *v1alpha1.Manager, replicas int32) error {
	if manager.Spec.Services.Webui == nil {
		if manager.Status.Webui != nil {
			oldWebUI := &v1alpha1.Webui{}
			oldWebUI.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Webui.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldWebUI)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Webui = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	webui := &v1alpha1.Webui{}
	webui.ObjectMeta = manager.Spec.Services.Webui.ObjectMeta
	webui.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, webui, func() error {
		webui.Spec = manager.Spec.Services.Webui.Spec
		webui.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, webui.Spec.CommonConfiguration)
		if webui.Spec.CommonConfiguration.Replicas == nil {
			webui.Spec.CommonConfiguration.Replicas = &replicas
		}
		return controllerutil.SetControllerReference(manager, webui, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &webui.Name
	status.Active = &webui.Status.Active
	manager.Status.Webui = status
	return err
}

func (r *ReconcileManager) processConfig(manager *v1alpha1.Manager, replicas int32, hostAliases []corev1.HostAlias) error {
	if manager.Spec.Services.Config == nil {
		if manager.Status.Config != nil {
			oldConfig := &v1alpha1.Config{}
			oldConfig.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Config.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldConfig)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Config = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	config := &v1alpha1.Config{}
	config.ObjectMeta = manager.Spec.Services.Config.ObjectMeta
	config.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, config, func() error {
		config.Spec = manager.Spec.Services.Config.Spec
		config.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, config.Spec.CommonConfiguration)
		if config.Spec.CommonConfiguration.Replicas == nil {
			config.Spec.CommonConfiguration.Replicas = &replicas
		}
		if len(config.Spec.CommonConfiguration.HostAliases) == 0 {
			config.Spec.CommonConfiguration.HostAliases = hostAliases
		}
		return controllerutil.SetControllerReference(manager, config, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &config.Name
	status.Active = config.Status.Active
	manager.Status.Config = status
	return nil
}

func (r *ReconcileManager) processKubemanagers(manager *v1alpha1.Manager, replicas int32) error {
	for _, existingKubemanager := range manager.Status.Kubemanagers {
		found := false
		for _, intendedKubemanager := range manager.Spec.Services.Kubemanagers {
			if *existingKubemanager.Name == intendedKubemanager.Name {
				found = true
				break
			}
		}
		if !found {
			oldKubemanager := &v1alpha1.Kubemanager{}
			oldKubemanager.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingKubemanager.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldKubemanager)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	var kubemanagerServiceStatus []*v1alpha1.ServiceStatus

	for _, kubemanagerService := range manager.Spec.Services.Kubemanagers {
		kubemanager := &v1alpha1.Kubemanager{}
		kubemanager.ObjectMeta = kubemanagerService.ObjectMeta
		kubemanager.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, kubemanager, func() error {
			kubemanager.Spec.ServiceConfiguration = kubemanagerService.Spec.ServiceConfiguration
			kubemanager.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, kubemanagerService.Spec.CommonConfiguration)
			if kubemanager.Spec.CommonConfiguration.Replicas == nil {
				kubemanager.Spec.CommonConfiguration.Replicas = &replicas
			}
			return controllerutil.SetControllerReference(manager, kubemanager, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &kubemanager.Name
		status.Active = kubemanager.Status.Active
		kubemanagerServiceStatus = append(kubemanagerServiceStatus, status)
	}

	manager.Status.Kubemanagers = kubemanagerServiceStatus
	return nil
}

func (r *ReconcileManager) processControls(manager *v1alpha1.Manager, replicas int32) error {
	for _, existingControl := range manager.Status.Controls {
		found := false
		for _, intendedControl := range manager.Spec.Services.Controls {
			if *existingControl.Name == intendedControl.Name {
				found = true
				break
			}
		}
		if !found {
			oldControl := &v1alpha1.Control{}
			oldControl.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingControl.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldControl)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	var controlServiceStatus []*v1alpha1.ServiceStatus
	for _, controlService := range manager.Spec.Services.Controls {
		control := &v1alpha1.Control{}
		control.ObjectMeta = controlService.ObjectMeta
		control.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, control, func() error {
			control.Spec = controlService.Spec
			control.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, control.Spec.CommonConfiguration)
			if control.Spec.CommonConfiguration.Replicas == nil {
				control.Spec.CommonConfiguration.Replicas = &replicas
			}
			return controllerutil.SetControllerReference(manager, control, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &control.Name
		status.Active = control.Status.Active
		controlServiceStatus = append(controlServiceStatus, status)
	}

	manager.Status.Controls = controlServiceStatus
	return nil
}

func (r *ReconcileManager) processRabbitMQ(manager *v1alpha1.Manager, replicas int32) error {
	if manager.Spec.Services.Rabbitmq == nil {
		if manager.Status.Rabbitmq != nil {
			oldRabbitMQ := &v1alpha1.Rabbitmq{}
			oldRabbitMQ.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Rabbitmq.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldRabbitMQ)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Rabbitmq = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	rabbitMQ := &v1alpha1.Rabbitmq{}
	rabbitMQ.ObjectMeta = manager.Spec.Services.Rabbitmq.ObjectMeta
	rabbitMQ.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, rabbitMQ, func() error {
		rabbitMQ.Spec = manager.Spec.Services.Rabbitmq.Spec
		rabbitMQ.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, rabbitMQ.Spec.CommonConfiguration)
		if rabbitMQ.Spec.CommonConfiguration.Replicas == nil {
			rabbitMQ.Spec.CommonConfiguration.Replicas = &replicas
		}
		return controllerutil.SetControllerReference(manager, rabbitMQ, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &rabbitMQ.Name
	status.Active = rabbitMQ.Status.Active
	manager.Status.Rabbitmq = status
	return err
}

func (r *ReconcileManager) processVRouters(manager *v1alpha1.Manager, replicas int32) error {
	for _, existingVRouter := range manager.Status.Vrouters {
		found := false
		for _, intendedVRouter := range manager.Spec.Services.Vrouters {
			if *existingVRouter.Name == intendedVRouter.Name {
				found = true
				break
			}
		}
		if !found {
			oldVRouter := &v1alpha1.Vrouter{}
			oldVRouter.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingVRouter.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldVRouter)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	var vRouterServiceStatus []*v1alpha1.ServiceStatus
	for _, vRouterService := range manager.Spec.Services.Vrouters {

		vRouter := &v1alpha1.Vrouter{}
		vRouter.ObjectMeta = vRouterService.ObjectMeta
		vRouter.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, vRouter, func() error {
			vRouter.Spec.ServiceConfiguration = vRouterService.Spec.ServiceConfiguration.VrouterConfiguration
			vRouter.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, vRouterService.Spec.CommonConfiguration)
			if vRouter.Spec.CommonConfiguration.Replicas == nil {
				vRouter.Spec.CommonConfiguration.Replicas = &replicas
			}
			return controllerutil.SetControllerReference(manager, vRouter, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &vRouter.Name
		status.Active = vRouter.Status.Active
		vRouterServiceStatus = append(vRouterServiceStatus, status)
	}

	manager.Status.Vrouters = vRouterServiceStatus
	return nil
}

func (r *ReconcileManager) processCSRSignerCaConfigMap(manager *v1alpha1.Manager) error {
	caCertificate := certificates.NewCACertificate(r.client, r.scheme, manager, "manager")
	if err := caCertificate.EnsureExists(); err != nil {
		return err
	}

	csrSignerCaConfigMap := &corev1.ConfigMap{}
	csrSignerCaConfigMap.ObjectMeta.Name = certificates.SignerCAConfigMapName
	csrSignerCaConfigMap.ObjectMeta.Namespace = manager.Namespace

	_, err := controllerutil.CreateOrUpdate(context.Background(), r.client, csrSignerCaConfigMap, func() error {
		csrSignerCAValue, err := caCertificate.GetCaCert()
		if err != nil {
			return err
		}
		csrSignerCaConfigMap.Data = map[string]string{certificates.SignerCAFilename: string(csrSignerCAValue)}
		return controllerutil.SetControllerReference(manager, csrSignerCaConfigMap, r.scheme)
	})

	return err
}
