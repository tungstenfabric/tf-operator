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
	"github.com/go-logr/logr"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
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
	reconcileManager := &ReconcileManager{Client: mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Manager: mgr,
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
	Client  client.Client
	Scheme  *runtime.Scheme
	Manager manager.Manager
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
	ss = strings.Split(manager.Spec.Services.Kubemanager.Spec.ServiceConfiguration.Containers[0].Image, ":")
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

	mngr, err := v1alpha1.GetManagerObject(clnt)
	if err != nil {
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
	if reflect.TypeOf(service.Interface()).Kind() == reflect.Slice {
		for i := 0; i < service.Len(); i++ {
			meta := getChildObjectByIface("ObjectMeta", service.Index(i).Interface())
			serviceInstanceName := getIfaceField("Name", meta).(string)
			if subRes, err = runFn(kind, serviceInstanceName, true, clnt, params); err != nil {
				return nil, err
			}
			res = append(res, subRes)
		}
	} else {
		if reflect.ValueOf(service.Interface()).Kind() == reflect.Ptr && !reflect.ValueOf(service.Interface()).Elem().IsValid() {
			panic(fmt.Errorf("Internatl error: invalide service iface kind %+v obj=%+v mgr=%+v", kind, service, mngr.Spec.Services))
		}
		meta := getChildObjectByIface("ObjectMeta", service.Interface())
		serviceInstanceName := getIfaceField("Name", meta).(string)
		if subRes, err = runFn(kind, serviceInstanceName, false, clnt, params); err != nil {
			return nil, err
		}
		res = append(res, subRes)
	}
	return res, nil
}

func getObjectKey(name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: "tf",
		Name:      name,
	}
}

func getClusterObjectKey() client.ObjectKey {
	return getObjectKey("cluster1")
}

func getUnstructured(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "tf.tungsten.io",
		Kind:    name,
		Version: "v1alpha1",
	})
	return u
}

func getManagerUnstructured(clnt client.Client) (mgr *unstructured.Unstructured, err error) {
	mgr = getUnstructured("Manager")
	err = clnt.Get(context.Background(), getClusterObjectKey(), mgr)
	return
}

func toMap(iface interface{}) map[string]interface{} {
	val := reflect.ValueOf(iface)
	kind := val.Kind()
	switch kind {
	case reflect.Map:
		return iface.(map[string]interface{})
	case reflect.Struct:
		relType := val.Type()
		res := map[string]interface{}{}
		for idx := 0; idx < relType.NumField(); idx++ {
			f := relType.Field(idx)
			res[f.Name] = val.Field(idx).Interface()
		}
		return res
	case reflect.Ptr:
		if !val.Elem().IsValid() {
			panic(fmt.Errorf("Internal error: kind %+v, invalid obj=%+v", kind, iface))
		}
		return toMap(val.Elem().Interface())
	default:
		panic(fmt.Errorf("Internal error: kind %+v cannot be convert to map, obj=%+v", kind, iface))
	}
}

func GetChildObject(xPath string, obj map[string]interface{}) map[string]interface{} {
	elements := strings.Split(xPath, "/")
	if len(elements) == 0 || elements[0] == "" {
		panic(fmt.Errorf("Internal error: xPath parameter cannot be empty %s", xPath))
	}
	if child, ok := obj[elements[0]]; ok {
		cm := toMap(child)
		if len(elements) == 1 {
			return cm
		}
		return GetChildObject(strings.Join(elements[1:], "/"), cm)
	}
	panic(fmt.Errorf("Internal error: No child %s in object %+v", elements[0], obj))
}

func getChildObjectByIface(xPath string, iface interface{}) map[string]interface{} {
	obj := toMap(iface)
	if obj == nil {
		panic(fmt.Errorf("Internal error: not Interface: %+v ", iface))
	}
	return GetChildObject(xPath, obj)
}

func getIfaceField(xPath string, iface interface{}) interface{} {
	elements := strings.Split(xPath, "/")
	if len(elements) != 1 || elements[0] == "" {
		panic(fmt.Errorf("Internal error: xPath parameter now is not supported %s", xPath))
	}
	return toMap(iface)[elements[0]]
}

// Get Kind based on ZIU Stage
// Get manager unstructured spec for kind
// For each instance of the service check if Instance is updated
func isServiceUpdated(ziuStage v1alpha1.ZIUStatus, clnt client.Client) (bool, error) {
	kind := v1alpha1.ZiuKinds[ziuStage]
	u, err := getManagerUnstructured(clnt)
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
	updatedFalse := map[string]interface{}{"Updated": false}
	updatedTrue := map[string]interface{}{"Updated": true}
	var err error

	// Got STS related to service instance
	stsName := serviceName + "-" + strings.ToLower(kind) + "-statefulset"
	sts := &appsv1.StatefulSet{}
	if err = clnt.Get(context.Background(), types.NamespacedName{Name: stsName, Namespace: "tf"}, sts); err != nil {
		if errors.IsNotFound(err) {
			// We have to wait when sts will bw set up
			return updatedFalse, nil
		}
		return updatedFalse, err
	}

	// Find service instance spec in the manager spec
	var u_service map[string]interface{}
	services := GetChildObject("Manager/spec/services", params)
	serviceNameByKind := getServiceNameByKind(kind, services)
	if isSlice {
		// Iterate over services of this kind and find service with proper name
		for _, serv := range services[serviceNameByKind].([]interface{}) {
			if getChildObjectByIface("metadata", serv)["name"].(string) == serviceName {
				u_service = serv.(map[string]interface{})
				break
			}
		}
	} else {
		u_service = GetChildObject(serviceNameByKind, services)
	}
	if len(u_service) == 0 {
		return updatedFalse, fmt.Errorf("Failed to find service %s/%s by %s in object %+v", kind, serviceName, serviceNameByKind, services)
	}

	// Get name and image from the first container from manager service spec
	managerContainerList := GetChildObject("spec/serviceConfiguration", u_service)["containers"].([]interface{})
	if len(managerContainerList) == 0 {
		return updatedFalse, fmt.Errorf("Empty %s/%s service containers list: %s", serviceNameByKind, serviceName, u_service)
	}

	managerContainerName := getIfaceField("name", managerContainerList[0]).(string)
	managerContainerImage := getIfaceField("image", managerContainerList[0]).(string)

	// Find a container with the same name in STS template
	if !checkContainersTag(managerContainerName, managerContainerImage, sts.Spec.Template.Spec.Containers) {
		return updatedFalse, nil
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
	for _, podItem := range pods.Items {
		if podItem.Status.Phase != corev1.PodPhase("Running") ||
			!cmpContainers(sts.Spec.Template.Spec.Containers, podItem.Spec.Containers) {
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

func cmpContainers(stsContainers, podContainers []corev1.Container) bool {
	for _, c := range stsContainers {
		if !checkContainersTag(c.Name, c.Image, podContainers) {
			return false
		}
	}
	return true
}

func checkContainersTag(name, image string, containers []corev1.Container) bool {
	for _, container := range containers {
		if name == container.Name {
			return image == container.Image
		}
	}
	return false
}

// Extract by name and kind part of Manager manifest (resource spec) as unstructured
func getUnstructuredSpec(kind string, name string, isSlice bool, mgr *unstructured.Unstructured) (map[string]interface{}, error) {
	services := GetChildObject("spec/services", mgr.UnstructuredContent())
	// find right service: it's key in lowercase have to starts with kind
	var serviceSpec map[string]interface{}
	serviceName := getServiceNameByKind(kind, services)
	if serviceName == "" {
		return nil, fmt.Errorf("No service %s in spec/services %s", kind, services)
	}
	if isSlice {
		for _, serv := range services[serviceName].([]interface{}) {
			if getChildObjectByIface("metadata", serv)["name"].(string) == name {
				serviceSpec = getChildObjectByIface("spec", serv)
				break
			}
		}
	} else {
		serviceSpec = GetChildObject(serviceName+"/spec", services)
	}
	if len(serviceSpec) == 0 {
		return nil, fmt.Errorf("Failed to find %v/%v in manager spec by name %v", kind, name, serviceName)
	}
	return serviceSpec, nil
}

func updateResource(kind string, serviceName string, isSlice bool, clnt client.Client) error {
	mgr, err := getManagerUnstructured(clnt)
	if err != nil {
		return err
	}
	spec, err := getUnstructuredSpec(kind, serviceName, isSlice, mgr)
	if err != nil {
		return err
	}
	res := getUnstructured(kind)
	if err = clnt.Get(context.Background(), getObjectKey(serviceName), res); err != nil && !errors.IsNotFound(err) {
		return err
	}
	createNew := errors.IsNotFound(err)

	managerCommonConf := GetChildObject("spec/commonConfiguration", mgr.UnstructuredContent())
	spec["commonConfiguration"] = utils.MergeUnstructuredCommonConfig(managerCommonConf, spec["commonConfiguration"])
	res.Object["spec"] = spec
	log.Info(fmt.Sprintf("Service %s/%s spec: %+v", kind, serviceName, spec))

	if createNew {
		// Create new
		if err = clnt.Create(context.Background(), res); !errors.IsAlreadyExists(err) {
			return err
		}
		return nil
	}
	return clnt.Update(context.Background(), res)
}

func updateZiuResource(kind string, serviceName string, isSlice bool, clnt client.Client, params map[string]interface{}) (map[string]interface{}, error) {
	fake := make(map[string]interface{})
	return fake, updateResource(kind, serviceName, isSlice, clnt)
}

func processZiuStage(ziuStage v1alpha1.ZIUStatus, clnt client.Client) error {
	if _, err := iterateOverKindInstances(v1alpha1.ZiuKinds[ziuStage], updateZiuResource, clnt, nil); err != nil {
		return err
	}
	return v1alpha1.SetZiuStage(int(ziuStage)+1, clnt)
}

func ReconcileZiu(log logr.Logger, clnt client.Client) (reconcile.Result, error) {
	reqLogger := log.WithName("ZIU")

	ziuStage, err := v1alpha1.GetZiuStage(clnt)
	if err != nil || ziuStage < 0 {
		if err != nil {
			reqLogger.Error(err, "Error in ZIU")
		}
		return reconcile.Result{}, err
	}

	restartTime, _ := time.ParseDuration("15s")
	requeueResult := reconcile.Result{Requeue: true, RequeueAfter: restartTime}

	// We have to wait previous stage updated and ready
	if ziuStage > 0 {
		if isUpdated, err := isServiceUpdated(ziuStage-1, clnt); err != nil || !isUpdated {
			reqLogger.Info("Wait for updating services", "ziuStage", ziuStage, "err", err)
			return requeueResult, err
		}
	}
	if len(v1alpha1.ZiuKinds) == int(ziuStage) {
		// ZIU have been finished - set stage to -1
		reqLogger.Info("ZIU done")
		return requeueResult, v1alpha1.SetZiuStage(-1, clnt)
	}
	reqLogger.Info("Process ZIU stage", "ziuStage", ziuStage)
	return requeueResult, processZiuStage(ziuStage, clnt)
}

// Reconcile reconciles the manager.
func (r *ReconcileManager) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Manager")

	instance, err := v1alpha1.GetManagerObject(r.Client)
	if err != nil {
		return requeueReconcile, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	if err := r.processCSRSignerCaConfigMap(instance); err != nil {
		return reconcile.Result{}, fmt.Errorf("Failed to prepare CA: err=%+v", err)
	}

	// Run ZIU Process if no error in status get
	if res, err := ReconcileZiu(reqLogger, r.Client); err != nil || res.Requeue {
		return res, err
	}

	// set defaults if not set
	if err := instance.Spec.CommonConfiguration.AuthParameters.Prepare(request.Namespace, r.Client); err != nil {
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

	if err := r.processKubemanager(instance); err != nil {
		if v1alpha1.IsOKForRequeque(err) {
			log.Info("Failed to processKubemanager, future rereconcile")
			requeueErr = err
		}
		log.Error(err, "processKubemanager")
	}

	r.setConditions(instance)
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
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
			err := r.Client.Delete(context.TODO(), oldConfig)
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
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, analytics, func() error {
		analytics.Spec = manager.Spec.Services.Analytics.Spec
		analytics.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analytics.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, analytics, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldConfig)
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
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, queryengine, func() error {
		queryengine.Spec = manager.Spec.Services.QueryEngine.Spec
		queryengine.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, queryengine.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, queryengine, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldAnalyticsSnmp)
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
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, analyticsSnmp, func() error {
		analyticsSnmp.Spec = manager.Spec.Services.AnalyticsSnmp.Spec
		analyticsSnmp.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analyticsSnmp.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, analyticsSnmp, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldAnalyticsAlarm)
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
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, analyticsAlarm, func() error {
		analyticsAlarm.Spec = manager.Spec.Services.AnalyticsAlarm.Spec
		analyticsAlarm.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, analyticsAlarm.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, analyticsAlarm, r.Scheme)
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
	if manager.Spec.Services.Zookeeper == nil {
		if manager.Status.Zookeeper != nil {
			old := &v1alpha1.Zookeeper{}
			old.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Zookeeper.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.Client.Delete(context.TODO(), old)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Zookeeper = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	zookeeper := &v1alpha1.Zookeeper{}
	zookeeper.ObjectMeta = manager.Spec.Services.Zookeeper.ObjectMeta
	zookeeper.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, zookeeper, func() error {
		zookeeper.Spec = manager.Spec.Services.Zookeeper.Spec
		zookeeper.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, zookeeper.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, zookeeper, r.Scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{Name: &zookeeper.Name, Active: zookeeper.Status.Active}
	manager.Status.Zookeeper = status
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
			err := r.Client.Delete(context.TODO(), oldCassandra)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	cassandraStatusList := []*v1alpha1.ServiceStatus{}
	for _, cassandraService := range manager.Spec.Services.Cassandras {
		cassandra := &v1alpha1.Cassandra{}
		cassandra.ObjectMeta = cassandraService.ObjectMeta
		cassandra.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, cassandra, func() error {
			cassandra.Spec = cassandraService.Spec
			cassandra.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, cassandra.Spec.CommonConfiguration)
			return controllerutil.SetControllerReference(manager, cassandra, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldRedis)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	redisStatusList := []*v1alpha1.ServiceStatus{}
	for _, redisService := range manager.Spec.Services.Redis {
		redis := &v1alpha1.Redis{}
		redis.ObjectMeta = redisService.ObjectMeta
		redis.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, redis, func() error {
			redis.Spec = redisService.Spec
			if redis.Spec.ServiceConfiguration.ClusterName == "" {
				redis.Spec.ServiceConfiguration.ClusterName = manager.GetName()
			}
			redis.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, redis.Spec.CommonConfiguration)
			return controllerutil.SetControllerReference(manager, redis, r.Scheme)
		})
		if err != nil {
			return err
		}
		status := &v1alpha1.ServiceStatus{Name: &redis.Name, Active: redis.Status.Active}
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
			err := r.Client.Delete(context.TODO(), oldWebUI)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Webui = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	webui := &v1alpha1.Webui{}
	webui.ObjectMeta = manager.Spec.Services.Webui.ObjectMeta
	webui.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, webui, func() error {
		webui.Spec = manager.Spec.Services.Webui.Spec
		webui.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, webui.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, webui, r.Scheme)
	})
	if err != nil {
		return err
	}
	status := &v1alpha1.ServiceStatus{Name: &webui.Name, Active: webui.Status.Active}
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
			err := r.Client.Delete(context.TODO(), oldConfig)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Config = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	config := &v1alpha1.Config{}
	config.ObjectMeta = manager.Spec.Services.Config.ObjectMeta
	config.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, config, func() error {
		config.Spec = manager.Spec.Services.Config.Spec
		config.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, config.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, config, r.Scheme)
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

func (r *ReconcileManager) processKubemanager(manager *v1alpha1.Manager) error {
	if manager.Spec.Services.Kubemanager == nil {
		if manager.Status.Kubemanager != nil {
			old := &v1alpha1.Kubemanager{}
			old.ObjectMeta = v1.ObjectMeta{
				Namespace: manager.Namespace,
				Name:      *manager.Status.Kubemanager.Name,
				Labels: map[string]string{
					"tf_cluster": manager.Name,
				},
			}
			err := r.Client.Delete(context.TODO(), old)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Kubemanager = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	kubemanager := &v1alpha1.Kubemanager{}
	kubemanager.ObjectMeta = manager.Spec.Services.Kubemanager.ObjectMeta
	kubemanager.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, kubemanager, func() error {
		kubemanager.Spec = manager.Spec.Services.Kubemanager.Spec
		kubemanager.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, kubemanager.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, kubemanager, r.Scheme)
	})
	if err != nil {
		return err
	}
	manager.Status.Kubemanager = &v1alpha1.ServiceStatus{Name: &kubemanager.Name, Active: kubemanager.Status.Active}
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
			err := r.Client.Delete(context.TODO(), oldControl)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	var controlServiceStatus []*v1alpha1.ServiceStatus
	for _, controlService := range manager.Spec.Services.Controls {
		control := &v1alpha1.Control{}
		control.ObjectMeta = controlService.ObjectMeta
		control.ObjectMeta.Namespace = manager.Namespace
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, control, func() error {
			control.Spec = controlService.Spec
			control.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, control.Spec.CommonConfiguration)
			return controllerutil.SetControllerReference(manager, control, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldRabbitMQ)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			manager.Status.Rabbitmq = nil
		}
		return nil
	}

	if !manager.IsVrouterActiveOnControllers(r.Client) {
		return nil
	}

	rabbitMQ := &v1alpha1.Rabbitmq{}
	rabbitMQ.ObjectMeta = manager.Spec.Services.Rabbitmq.ObjectMeta
	rabbitMQ.ObjectMeta.Namespace = manager.Namespace
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, rabbitMQ, func() error {
		rabbitMQ.Spec = manager.Spec.Services.Rabbitmq.Spec
		rabbitMQ.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, rabbitMQ.Spec.CommonConfiguration)
		return controllerutil.SetControllerReference(manager, rabbitMQ, r.Scheme)
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
			err := r.Client.Delete(context.TODO(), oldVRouter)
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
		_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vRouter, func() error {
			vRouter.Spec.ServiceConfiguration = vRouterService.Spec.ServiceConfiguration
			vRouter.Spec.CommonConfiguration = utils.MergeCommonConfiguration(manager.Spec.CommonConfiguration, vRouterService.Spec.CommonConfiguration)
			return controllerutil.SetControllerReference(manager, vRouter, r.Scheme)
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
	return v1alpha1.InitCA(r.Client, r.Scheme, manager, "manager")
}
