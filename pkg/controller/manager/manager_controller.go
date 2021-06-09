package manager

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fatih/structs"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
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
	&v1alpha1.Redis{},
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
	reconcileManager := &ReconcileManager{client: mgr.GetClient(),
		scheme:  mgr.GetScheme(),
		manager: mgr,
	}
	c, err := controller.New("manager-controller", mgr, controller.Options{Reconciler: reconcileManager})
	if err != nil {
		return err
	}
	return addManagerWatch(c, mgr)
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
	client  client.Client
	scheme  *runtime.Scheme
	manager manager.Manager
}

// Get kubemanager STS. If kubemanager STS not found looks like tf start first time - return false
// Get kubemanager image tag from current setup (from kubemanager STS created)
// Get kubemanager image tag from current manager manifest
// If they are the same return false
// otherwise return true - tags are differ and we start ZIU procedure
func IsZiuRequired(clnt client.Client) (bool, error) {
	stsList := &appsv1.StatefulSetList{}

	// Check if required STS exists in the cluster
	err := clnt.List(context.Background(), stsList, client.InNamespace("tf"))
	if err != nil {
		return false, fmt.Errorf("Can't list stses %v", err)
	}
	stsFound := false
	for _, item := range stsList.Items {
		if item.Name == "kubemanager1-kubemanager-statefulset" {
			stsFound = true
		}
	}
	if !stsFound {
		// Looks like setup installed the first time
		return false, nil
	}

	// Get kubemanager container tag from sts
	kubemanagerSts := &appsv1.StatefulSet{}
	kubemanagerName := types.NamespacedName{Name: "kubemanager1-kubemanager-statefulset", Namespace: "tf"}
	err = clnt.Get(context.Background(), kubemanagerName, kubemanagerSts)
	if err != nil {
		return false, fmt.Errorf("Can't get kubemanager sts %v", err)
	}
	ss := strings.Split(kubemanagerSts.Spec.Template.Spec.Containers[0].Image, ":")
	kmStsTag := ss[len(ss)-1]
	// Get kubemanager container tag from manager manifest
	manager := &v1alpha1.Manager{}
	managerName := types.NamespacedName{Name: "cluster1", Namespace: "tf"}
	err = clnt.Get(context.Background(), managerName, manager)
	if err != nil {
		return false, fmt.Errorf("Can't get manager manifest %v", err)
	}
	ss = strings.Split(manager.Spec.Services.Kubemanagers[0].Spec.ServiceConfiguration.Containers[0].Image, ":")
	kmManagerTag := ss[len(ss)-1]
	if kmManagerTag == kmStsTag {
		return false, nil
	}
	return true, nil
}

// Got unstructured Services and Kind
// Iterate over Services map and find its key
// return service Name
func getServiceNameByKind(kind string, servicesStrict map[string]interface{}) string {
	serviceName := ""
	for name := range servicesStrict {
		if strings.EqualFold(name, kind) || strings.EqualFold(name, kind+"s") {
			serviceName = name
			break
		}
	}
	return serviceName
}

type srvInstanceFn func(
	kind string,
	srvName string,
	isSlice bool,
	clnt client.Client,
	params map[string]interface{}) (map[string]interface{}, error)

// Get instances of the kind from manager spec and call runFn function for each service
// Pass params if any to each call and  collect results in returned slice
func iterateOverKindInstances(kind string,
	runFn srvInstanceFn,
	clnt client.Client,
	params map[string]interface{}) ([]map[string]interface{}, error) {

	mngr := &v1alpha1.Manager{}
	mngrName := types.NamespacedName{Name: "cluster1", Namespace: "tf"}
	if err := clnt.Get(context.Background(), mngrName, mngr); err != nil {
		return nil, err
	}
	var service reflect.Value
	var _tried_services string
	for serviceName := range structs.Map(mngr.Spec.Services) {
		if strings.EqualFold(serviceName, kind) || strings.EqualFold(serviceName, kind+"s") {
			_srvs := reflect.Indirect(reflect.ValueOf(&mngr.Spec.Services))
			service = _srvs.FieldByName(serviceName)
			break
		}
		_tried_services = _tried_services + serviceName + " "
	}
	if !service.IsValid() {
		return nil, fmt.Errorf("Failed to find %s in services: %+v", kind, _tried_services)
	}

	var res []map[string]interface{}
	var subRes map[string]interface{}
	var err error
	if reflect.TypeOf(service.Interface()).Kind() == reflect.Slice {
		for i := 0; i < service.Len(); i++ {
			serviceInstanceName := structs.Map(service.Index(i).Interface())["ObjectMeta"].(map[string]interface{})["Name"].(string)
			if subRes, err = runFn(kind, serviceInstanceName, true, clnt, params); err != nil {
				return nil, err
			}
			res = append(res, subRes)
		}
	} else {
		serviceInstanceName := structs.Map(service.Interface())["ObjectMeta"].(map[string]interface{})["Name"].(string)
		if subRes, err = runFn(kind, serviceInstanceName, false, clnt, params); err != nil {
			return nil, err
		}
		res = append(res, subRes)
	}
	return res, nil
}

// Get Kind based on ZIU Stage
// Get manager unstructured spec for kind
// For each instance of the service check if Instance is updated
func isServiceUpdated(ziuStage int, clnt client.Client) (bool, error) {
	kind := v1alpha1.ZiuKinds[ziuStage]
	// Get manager as unstructured
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    "Manager",
		Version: "v1alpha1",
	})
	err := clnt.Get(context.Background(), client.ObjectKey{
		Namespace: "tf",
		Name:      "cluster1",
	}, u)
	if err != nil {
		return false, err
	}
	params := map[string]interface{}{
		"Manager": u.UnstructuredContent(),
	}
	resArr, err := iterateOverKindInstances(kind, isServiceInstanceUpdated, clnt, params)
	if err != nil {
		return false, err
	}
	if len(resArr) == 0 {
		return false, fmt.Errorf("iterateOverKindInstances needs to return non empty result")
	}
	var res bool = true
	for _, val := range resArr {
		res = res && val["Updated"].(bool)
	}
	return res, nil
}

// As params we got "Manager" spec as Unstructured
// And return boolean under "Updated" key in the map
//
// Find in the cluster STS related to the instance on service in manager spec
// Get name from the first container from manager service spec
// Find a container with the same name in STS template
// check if container image from manager manifest is the same with the image from deployed sts
// check if STS Updated Replicas is the same with Replicas
// if it is return true, otherwise, return false
func isServiceInstanceUpdated(kind string, serviceName string, isSlice bool, clnt client.Client, params map[string]interface{}) (map[string]interface{}, error) {
	// TODO Remove if it works without reflect
	// updatedFalse := map[string]interface{}{"Updated": reflect.ValueOf(false).Interface()}
	// updatedTrue := map[string]interface{}{"Updated": reflect.ValueOf(true).Interface()}
	updatedFalse := map[string]interface{}{"Updated": false}
	updatedTrue := map[string]interface{}{"Updated": true}

	// Got STS related to service instance
	stsName := serviceName + "-" + strings.ToLower(kind) + "-statefulset"
	sts := &appsv1.StatefulSet{}
	var err error
	if err = clnt.Get(context.Background(), types.NamespacedName{Name: stsName, Namespace: "tf"}, sts); err != nil {
		if errors.IsNotFound(err) {
			// We have to wait when sts will bw set up
			return updatedFalse, nil
		}
		return updatedFalse, err
	}

	// Find service instance spec in the manager spec
	var u_service map[string]interface{}
	services := params["Manager"].(map[string]interface{})["spec"].(map[string]interface{})["services"].(map[string]interface{})
	serviceNameByKind := getServiceNameByKind(kind, services)
	if isSlice {
		// Iterate over services of this kind and find service with proper name
		for _, servData := range services[serviceNameByKind].([]interface{}) {
			_srv := servData.(map[string]interface{})
			if _srv["metadata"].(map[string]interface{})["name"].(string) == serviceName {
				u_service = _srv
				break
			}
		}
	} else {
		u_service = services[serviceNameByKind].(map[string]interface{})
	}
	if len(u_service) == 0 {
		return updatedFalse, fmt.Errorf("Failed to find service %s/%s by %s in object %+v", kind, serviceName, serviceNameByKind, services)
	}

	// Get name and image from the first container from manager service spec
	managerContainer := u_service["spec"].(map[string]interface{})["serviceConfiguration"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})
	managerContainerName := managerContainer["name"].(string)
	managerContainerImage := managerContainer["image"].(string)

	// Find a container with the same name in STS template
	for _, container := range sts.Spec.Template.Spec.Containers {
		// check if container image from manager manifest is the same with the image from deployed sts
		if container.Name == managerContainerName && container.Image != managerContainerImage {
			return updatedFalse, nil
		}
	}
	// check if STS Updated Replicas is the same with Replicas
	if sts.Status.Replicas != sts.Status.UpdatedReplicas {
		return updatedFalse, nil
	}
	// Get service pods
	var pods *corev1.PodList
	if pods, err = v1alpha1.SelectPods(serviceName, strings.ToLower(kind), "tf", clnt); err != nil {
		return updatedFalse, err
	}
	// We have to see all pods required
	if int(sts.Status.Replicas) != len(pods.Items) {
		return updatedFalse, nil
	}
	podRes := true
	for _, podItem := range pods.Items {
		// We have to check contaier tag for all pods is the same with manager
		podRes = podRes && checkPodTag(managerContainerName, managerContainerImage, &podItem)
		// We have to see all pods are Running
		podRes = podRes && (podItem.Status.Phase == corev1.PodPhase("Running"))

		if !podRes {
			return updatedFalse, nil
		}
	}
	// Check if Service has Active status
	if !v1alpha1.IsUnstructuredActive(kind, serviceName, "tf", clnt) {
		return updatedFalse, nil
	}

	// All looks updated
	return updatedTrue, nil
}

func checkPodTag(managerContainerName string, managerContainerImage string, podItem *corev1.Pod) bool {
	resContainer := false
	for _, container := range podItem.Spec.Containers {
		if (managerContainerName == container.Name) && (managerContainerImage == container.Image) {
			resContainer = true
			break
		}
	}
	return resContainer
}

// Extract by name and kind part of Manager manifest (resource spec) as unstructured
func getUnstructuredSpec(kind string, name string, isSlice bool, clnt client.Client) (map[string]interface{}, error) {
	// Getmanager as unstructured
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    "Manager",
		Version: "v1alpha1",
	})
	err := clnt.Get(context.Background(), client.ObjectKey{
		Namespace: "tf",
		Name:      "cluster1",
	}, u)
	if err != nil {
		return u.UnstructuredContent(), err
	}
	services := u.UnstructuredContent()["spec"].(map[string]interface{})["services"].(map[string]interface{})
	// find right service: it's key in lowercase have to starts with kind
	var service map[string]interface{}
	serviceName := getServiceNameByKind(kind, services)
	if isSlice {
		for _, serv := range services[serviceName].([]interface{}) {
			if serv.(map[string]interface{})["metadata"].(map[string]interface{})["name"].(string) == name {
				service = serv.(map[string]interface{})["spec"].(map[string]interface{})
			}
		}
	} else {
		service = services[serviceName].(map[string]interface{})["spec"].(map[string]interface{})
	}
	if len(service) == 0 {
		return nil, fmt.Errorf("We can't find resource %v/%v by name %s in manager spec %+v", kind, name, serviceName, services)
	}
	return service, nil
}

func updateResource(kind string, serviceName string, isSlice bool, clnt client.Client) error {
	u_manager := &unstructured.Unstructured{}
	u_manager.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    "Manager",
		Version: "v1alpha1",
	})
	err := clnt.Get(context.Background(), client.ObjectKey{
		Namespace: "tf",
		Name:      "cluster1",
	}, u_manager)
	if err != nil {
		return err
	}
	managerCommonConf := u_manager.UnstructuredContent()["spec"].(map[string]interface{})["commonConfiguration"].(map[string]interface{})

	res := &unstructured.Unstructured{}
	res.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    kind,
		Version: "v1alpha1",
	})
	err = clnt.Get(context.Background(), client.ObjectKey{
		Namespace: "tf",
		Name:      serviceName,
	}, res)
	if err != nil {
		return err
	}
	res_spec, err := getUnstructuredSpec(kind, serviceName, isSlice, clnt)
	if err != nil {
		return err
	}
	res.Object["spec"] = res_spec
	mergedCommonConf := utils.MergeUnstructuredCommonConfig(managerCommonConf, res_spec["commonConfiguration"].(map[string]interface{}))
	res.Object["spec"].(map[string]interface{})["commonConfiguration"] = mergedCommonConf
	err = clnt.Update(context.Background(), res)
	return err
}

func updateZiuResource(kind string, serviceName string, isSlice bool, clnt client.Client, params map[string]interface{}) (map[string]interface{}, error) {
	fake := make(map[string]interface{})
	// Check if resource is absend on cluster - create it, increate ZIU Stage and return
	res := &unstructured.Unstructured{}
	res.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    kind,
		Version: "v1alpha1",
	})
	if err := clnt.Get(context.Background(), client.ObjectKey{
		Namespace: "tf",
		Name:      serviceName,
	}, res); err != nil {
		return fake, err
	}

	// We have resource in the cluster
	if _, err := v1alpha1.QuerySTS(res.GetName()+"-"+strings.ToLower(kind)+"-statefulset", res.GetNamespace(), clnt); err != nil {
		return fake, err
	}
	return nil, updateResource(kind, serviceName, isSlice, clnt)
}

func updateZiuStage(ziuStage v1alpha1.ZIUStatus, clnt client.Client) error {
	if _, err := iterateOverKindInstances(v1alpha1.ZiuKinds[ziuStage], updateZiuResource, clnt, nil); err != nil {
		return err
	}
	return v1alpha1.SetZiuStage(int(ziuStage)+1, clnt)

}

func ReconcileZiu(clnt client.Client) (reconcile.Result, error) {
	ziuStage, err := v1alpha1.GetZiuStage(clnt)
	if err != nil || ziuStage < 0 {
		return reconcile.Result{}, err
	}

	restartTime, _ := time.ParseDuration("15s")
	requeueResult := reconcile.Result{Requeue: true, RequeueAfter: restartTime}

	if ziuStage > 0 {
		// We have to wait previous stage updated and ready
		if isUpdated, err := isServiceUpdated(int(ziuStage)-1, clnt); err != nil || !isUpdated {
			return requeueResult, err
		}
	}

	if len(v1alpha1.ZiuKinds) == int(ziuStage) {
		// ZIU have been finished - set stage to -1
		return requeueResult, v1alpha1.SetZiuStage(-1, clnt)
	}

	return requeueResult, updateZiuStage(v1alpha1.ZIUStatus(ziuStage), clnt)
}

// Reconcile reconciles the manager.
func (r *ReconcileManager) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Manager")

	// Process ZIU if needed
	if res, err := ReconcileZiu(r.client); err != nil || res.Requeue {
		return res, err
	}

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

	// set defaults if not set
	if instance.Spec.CommonConfiguration.AuthParameters == nil {
		instance.Spec.CommonConfiguration.AuthParameters = &v1alpha1.AuthParameters{}
		instance.Spec.CommonConfiguration.AuthParameters.AuthMode = v1alpha1.AuthenticationModeNoAuth
	}
	if err := instance.Spec.CommonConfiguration.AuthParameters.Prepare(request.Namespace, r.client); err != nil {
		return reconcile.Result{}, err
	}

	var requeueErr error = nil
	if err := r.processVRouters(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processVRouters, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processVRouters")
	}

	if err := r.processRabbitMQ(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processRabbitMQ, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processRabbitMQ")
	}

	if err := r.processCassandras(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processCassandras, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processCassandras")
	}

	if err := r.processRedis(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processRedis, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processRedis")
	}

	if err := r.processZookeepers(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processZookeepers, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processZookeepers")
	}

	if err := r.processControls(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processControls, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processControls")
	}

	if err := r.processConfig(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processConfig, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processConfig")
	}

	if err := r.processWebui(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processWebui, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processWebui")
	}

	if err := r.processAnalytics(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalytics, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalytics")
	}

	if err := r.processQueryEngine(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processQueryEngine, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processQueryEngine")
	}

	if err := r.processAnalyticsSnmp(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalyticsSnmp, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalyticsSnmp")
	}

	if err := r.processAnalyticsAlarm(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processAnalyticsAlarm, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processAnalyticsAlarm")
	}

	if err := r.processKubemanagers(instance); err != nil {
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

func (r *ReconcileManager) processAnalytics(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processQueryEngine(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processAnalyticsSnmp(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processAnalyticsAlarm(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processZookeepers(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processCassandras(manager *v1alpha1.Manager) error {
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
			cassandra.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, cassandra.Spec.CommonConfiguration)
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

func (r *ReconcileManager) processRedis(manager *v1alpha1.Manager) error {
	for _, existingRedis := range manager.Status.Redis {
		found := false
		for _, intendedRedis := range manager.Spec.Services.Redis {
			if *existingRedis.Name == intendedRedis.Name {
				found = true
				break
			}
		}
		if !found {
			oldRedis := &v1alpha1.Redis{}
			oldRedis.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *existingRedis.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.client.Delete(context.TODO(), oldRedis)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.client) {
		return nil
	}

	redisStatusList := []*v1alpha1.ServiceStatus{}
	for _, redisService := range manager.Spec.Services.Redis {
		redis := &v1alpha1.Redis{}
		redis.ObjectMeta = redisService.ObjectMeta
		redis.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.client, redis, func() error {
			redis.Spec = redisService.Spec
			if redis.Spec.ServiceConfiguration.ClusterName == "" {
				redis.Spec.ServiceConfiguration.ClusterName = manager.GetName()
			}
			redis.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, redis.Spec.CommonConfiguration)
			return controllerutil.SetControllerReference(manager, redis, r.scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{}
		status.Name = &redis.Name
		status.Active = redis.Status.Active
		redisStatusList = append(redisStatusList, status)
	}
	manager.Status.Redis = redisStatusList
	return nil
}

func (r *ReconcileManager) processWebui(manager *v1alpha1.Manager) error {
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
		return controllerutil.SetControllerReference(manager, webui, r.scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{}
	status.Name = &webui.Name
	status.Active = webui.Status.Active
	manager.Status.Webui = status
	return err
}

func (r *ReconcileManager) processConfig(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processKubemanagers(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processControls(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processRabbitMQ(manager *v1alpha1.Manager) error {
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

func (r *ReconcileManager) processVRouters(manager *v1alpha1.Manager) error {
	if len(manager.Spec.Services.Vrouters) == 0 {
		return nil
	}
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
