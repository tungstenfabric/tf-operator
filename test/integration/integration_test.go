package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/stretchr/testify/require"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/controller/analytics"
	"github.com/tungstenfabric/tf-operator/pkg/controller/analyticsalarm"
	"github.com/tungstenfabric/tf-operator/pkg/controller/analyticssnmp"
	"github.com/tungstenfabric/tf-operator/pkg/controller/cassandra"
	"github.com/tungstenfabric/tf-operator/pkg/controller/config"
	"github.com/tungstenfabric/tf-operator/pkg/controller/control"
	"github.com/tungstenfabric/tf-operator/pkg/controller/kubemanager"
	"github.com/tungstenfabric/tf-operator/pkg/controller/manager"
	"github.com/tungstenfabric/tf-operator/pkg/controller/queryengine"
	"github.com/tungstenfabric/tf-operator/pkg/controller/rabbitmq"
	"github.com/tungstenfabric/tf-operator/pkg/controller/redis"
	"github.com/tungstenfabric/tf-operator/pkg/controller/vrouter"
	"github.com/tungstenfabric/tf-operator/pkg/controller/webui"
	"github.com/tungstenfabric/tf-operator/pkg/controller/zookeeper"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fakeclient "k8s.io/client-go/kubernetes/fake"
)

var ziuRequeDuration, _ = time.ParseDuration("15s")
var ziuReconcielResult reconcile.Result = reconcile.Result{Requeue: true, RequeueAfter: ziuRequeDuration}

func runtimeScheme(t *testing.T) *runtime.Scheme {
	scheme, err := v1alpha1.SchemeBuilder.Build()
	require.NoError(t, err, "Failed to build scheme")
	require.NoError(t, appsv1.AddToScheme(scheme), "Failed appsv1.AddToScheme")
	require.NoError(t, corev1.AddToScheme(scheme), "Failed corev1.AddToScheme")
	require.NoError(t, policy.AddToScheme(scheme), "Failed policy.AddToScheme")
	return scheme
}

func getObjectsList(data map[string]runtime.Object) []runtime.Object {
	var res []runtime.Object
	for _, v := range data {
		res = append(res, v)
	}
	return res
}

func requireZiuStage(t *testing.T, stage int, cl client.Client) {
	ziuStage, err := v1alpha1.GetZiuStage(cl)
	require.NoError(t, err)
	require.Equal(t, v1alpha1.ZIUStatus(stage), ziuStage)
}

func ziuReconcileObjects(m *manager.ReconcileManager) []reconcile.Reconciler {
	cl := m.Client
	scheme := m.Scheme
	mgr := m.Manager
	k8s := k8s.New(cl, scheme)

	// WARNING: fixed workflow.
	// Must be changed if v1alpha1.ZiuKinds is updated
	return []reconcile.Reconciler{
		&config.ReconcileConfig{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&analytics.ReconcileAnalytics{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&analyticsalarm.ReconcileAnalyticsAlarm{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&analyticssnmp.ReconcileAnalyticsSnmp{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&redis.ReconcileRedis{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&queryengine.ReconcileQueryEngine{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&cassandra.ReconcileCassandra{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&zookeeper.ReconcileZookeeper{Client: cl, Scheme: scheme},
		&rabbitmq.ReconcileRabbitmq{Client: cl, Scheme: scheme, Manager: mgr},
		&control.ReconcileControl{Client: cl, Scheme: scheme, Manager: mgr},
		&webui.ReconcileWebui{Client: cl, Scheme: scheme, Manager: mgr, Kubernetes: k8s},
		&kubemanager.ReconcileKubemanager{Client: cl, Scheme: scheme},
		&vrouter.ReconcileVrouter{Client: cl, Scheme: scheme},
	}
}

func ziuObjectNames(stage int) []string {
	// WARNING: fixed workflow.
	// Must be changed if v1alpha1.ZiuKinds is updated
	names := map[string][]string{
		"Config":         {"config1"},
		"Analytics":      {"analytics1"},
		"AnalyticsAlarm": {"analyticsalarm1"},
		"AnalyticsSnmp":  {"analyticssnmp1"},
		"Redis":          {"redis1"},
		"QueryEngine":    {"queryengine1"},
		"Cassandra":      {"configdb1", "analyticsdb1"},
		"Zookeeper":      {"zookeeper1"},
		"Rabbitmq":       {"rabbitmq1"},
		"Control":        {"control1"},
		"Webui":          {"webui1"},
		"Kubemanager":    {"kubemanager1"},
		"Vrouter":        {"vrouter1"},
	}
	return names[v1alpha1.ZiuKinds[stage]]
}

// TODO: analyticsdb1
func ziuObjectKind(stage int) string {
	// WARNING: fixed workflow.
	// Must be changed if v1alpha1.ZiuKinds is updated
	names := map[string]string{
		"Config":         "config",
		"Analytics":      "analytics",
		"AnalyticsAlarm": "analyticsalarm",
		"AnalyticsSnmp":  "analyticssnmp",
		"Redis":          "redis",
		"QueryEngine":    "queryengine",
		"Cassandra":      "cassandra",
		"Zookeeper":      "zookeeper",
		"Rabbitmq":       "rabbitmq",
		"Control":        "control",
		"Webui":          "webui",
		"Kubemanager":    "kubemanager",
		"Vrouter":        "vrouter",
	}
	return names[v1alpha1.ZiuKinds[stage]]
}

func runReconcileStage(t *testing.T, stage int, mgr *manager.ReconcileManager) (res reconcile.Result, err error) {
	err = nil
	ro := ziuReconcileObjects(mgr)[stage]
	for _, name := range ziuObjectNames(stage) {
		t.Logf("Reconsile %s", name)
		req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "tf", Name: name}}
		_, err = ro.Reconcile(req)
		require.NoError(t, err)
		res = reconcile.Result{Requeue: true}
		err = nil
		for tries := 10; res.Requeue && err == nil && tries > 0; tries-- {
			res, err = ro.Reconcile(req)
			if tries == 1 {
				err = fmt.Errorf("Reconcile %s stage %d is in infinity loop", name, stage)
			}
		}
		if err != nil {
			break
		}
	}
	return
}

func setUpdatedReplicas(stage, replicas int, mgr *manager.ReconcileManager) (err error) {
	err = nil
	sts := &appsv1.StatefulSet{}
	kind := ziuObjectKind(stage)
	for _, name := range ziuObjectNames(stage) {
		fullName := types.NamespacedName{Name: name + "-" + kind + "-statefulset", Namespace: "tf"}
		if err = mgr.Client.Get(context.TODO(), fullName, sts); err != nil {
			if !errors.IsNotFound(err) {
				break
			}
			err = nil
			continue
		}
		sts.Status.Replicas = 1
		// todo: remove this as replicas by nodeselector be merged
		sts.Spec.Replicas = &sts.Status.Replicas
		if replicas != -1 {
			sts.Status.UpdatedReplicas = int32(replicas)
		} else {
			sts.Status.ReadyReplicas = sts.Status.Replicas
			sts.Status.CurrentReplicas = sts.Status.Replicas
			sts.Status.UpdatedReplicas = sts.Status.Replicas
		}
		if err = mgr.Client.Status().Update(context.TODO(), sts); err != nil {
			break
		}
	}
	return
}

func updateReplicas(stage int, mgr *manager.ReconcileManager) error {
	setAsReplicasMark := -1
	return setUpdatedReplicas(stage, setAsReplicasMark, mgr)
}

func updateStatus(t *testing.T, stage int, mgr *manager.ReconcileManager) {
	kind := ziuObjectKind(stage)
	for _, name := range ziuObjectNames(stage) {
		t.Logf("Set status %s/%s", kind, name)
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "tf.tungsten.io", Kind: kind, Version: "v1alpha1"})
		k := client.ObjectKey{Namespace: "tf", Name: name}
		require.NoError(t, mgr.Client.Get(context.Background(), k, u))
		obj := u.UnstructuredContent()
		status := manager.GetChildObject("status", obj)
		t.Logf("Update status %s/%s (%+v)", kind, name, status)
		status["active"] = true
		status["degraded"] = false
		require.NoError(t, mgr.Client.Status().Update(context.Background(), u))
	}
}

func lookup(s string, list []string) bool {
	for _, item := range list {
		if strings.Contains(s, item) {
			return true
		}
	}
	return false
}

func requireServiceTag(t *testing.T, tag string, containers []corev1.Container) {
	for _, c := range containers {
		require.Contains(t, c.Image, tag)
	}
}

func requireServiceStsTag(t *testing.T, services []string, tag string, mgr *manager.ReconcileManager) {
	// Check STSes containers tag
	stsList := &appsv1.StatefulSetList{}
	require.NoError(t, mgr.Client.List(context.Background(), stsList, client.InNamespace("tf")))
	for _, item := range stsList.Items {
		if len(services) > 0 && !lookup(item.Name, services) {
			continue
		}
		requireServiceTag(t, tag, item.Spec.Template.Spec.Containers)
	}
}

func requireAllStsTag(t *testing.T, tag string, mgr *manager.ReconcileManager) {
	requireServiceStsTag(t, []string{}, tag, mgr)
}

// func requireAllPodsTag(t *testing.T, tag string, mgr *manager.ReconcileManager) {
// 	end := len(v1alpha1.ZiuKinds)
// 	for i := 0; i < end; i++ {
// 		kind := ziuObjectKind(i)
// 		for _, n := range ziuObjectNames(i) {
// 			pods, err := v1alpha1.SelectPods(n, kind, "tf", mgr.Client)
// 			require.NoError(t, err)
// 			require.Greater(t, len(pods.Items), 0)
// 			//
// 			for _, p := range pods.Items {
// 				t.Log("DBG", n, kind, p.Name)
// 				require.Equal(t, corev1.PodPhase("Running"), p.Status.Phase)
// 				require.Greater(t, len(p.Spec.Containers), 0)
// 				requireServiceTag(t, tag, p.Spec.Containers)
// 				requireServiceTag(t, tag, p.Spec.InitContainers)
// 			}
// 		}
// 	}
// }

func init() {
	os.Setenv(k8sutil.WatchNamespaceEnvVar, "tf")
	fakeClientSet := fakeclient.NewSimpleClientset()
	k8s.SetClientset(fakeClientSet.CoreV1(), nil)
	// fakeDiscovery, ok := fakeClientSet.Discovery().(*fakediscovery.FakeDiscovery)
	// require.Equal(t, true, ok)
	// fakeDiscovery.Fake.Resources = append(fakeDiscovery.Fake.Resources, &metav1.APIResourceList{
	// 	GroupVersion: "tf.tungsten.io/v1alpha1",
	// 	APIResources: []metav1.APIResource{
	// 		{
	// 			Kind: "Config",
	// 		},
	// 	},
	// })
}

func TestReconcileManager(t *testing.T) {
	runtimeScheme := runtimeScheme(t)
	clnt := fake.NewFakeClientWithScheme(
		runtimeScheme,
		GetTestData("master", "ManagerList"),
		GetTestData("master", "SecretList"),
	)
	var reconcileManager *manager.ReconcileManager = &manager.ReconcileManager{
		Client:  clnt,
		Scheme:  runtimeScheme,
		Manager: nil}
	var reconcileRequest reconcile.Request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "cluster1",
			Namespace: "tf"},
	}
	require.NoError(t, v1alpha1.SetZiuStage(-1, clnt))
	stage, err := v1alpha1.GetZiuStage(clnt)
	require.NoError(t, err)
	require.Equal(t, -1, int(stage))
	result, err := reconcileManager.Reconcile(reconcileRequest)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, result)

}

func TestReconcileManagerZIU(t *testing.T) {
	initialData := GetAllTestData("master")
	objects := getObjectsList(initialData)
	runtimeScheme := runtimeScheme(t)

	var reconcileRequest reconcile.Request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "cluster1",
			Namespace: "tf"},
	}
	clnt := fake.NewFakeClientWithScheme(runtimeScheme, objects...)
	var reconcileManager *manager.ReconcileManager = &manager.ReconcileManager{
		Client:  clnt,
		Scheme:  runtimeScheme,
		Manager: nil}

	require.NoError(t, v1alpha1.SetZiuStage(-1, clnt))
	stage, err := v1alpha1.GetZiuStage(clnt)
	require.NoError(t, err)
	require.Equal(t, -1, int(stage))

	result, err := reconcileManager.Reconcile(reconcileRequest)
	require.NoError(t, err)
	require.Equal(t, reconcile.Result{}, result)

	sts := &appsv1.StatefulSet{}
	nsName := types.NamespacedName{Name: "kubemanager1-kubemanager-statefulset", Namespace: "tf"}
	require.NoError(t, clnt.Get(context.Background(), nsName, sts))
	ss := strings.Split(sts.Spec.Template.Spec.Containers[0].Image, ":")
	ss[len(ss)-1] = "test-ziu"
	sts.Spec.Template.Spec.Containers[0].Image = strings.Join(ss[:], ":")
	require.NoError(t, clnt.Status().Update(context.TODO(), sts))
	result, err = reconcileManager.Reconcile(reconcileRequest)
	require.NoError(t, err)
	require.Equal(t, true, result.Requeue)
	ziuStage, err := v1alpha1.GetZiuStage(clnt)
	require.NoError(t, err)
	require.Equal(t, 0, int(ziuStage))
}

func TestZIU_Master(t *testing.T) {
	testZIUUpdate(t, "master", "new")
}

// TODO: not implemented 2011 > master (testdata needed)
// func TestZIU_2011(t *testing.T) {
// 	// TODO: replace with v1alpha1.InitZiu as it supports autodetect
// 	v1alpha1.ZiuKinds = v1alpha1.ZiuKindsMaster
// 	testZIUUpdate(t, "2011", "new")
// }

func testZIUUpdate(t *testing.T, initialVersion, targetVersion string) {

	initialData := GetAllTestData(initialVersion)
	objects := getObjectsList(initialData)
	runtimeScheme := runtimeScheme(t)

	var reconcileRequest reconcile.Request = reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "cluster1",
			Namespace: "tf"},
	}
	clnt := fake.NewFakeClientWithScheme(runtimeScheme, objects...)
	var reconcileManager *manager.ReconcileManager = &manager.ReconcileManager{
		Client:  clnt,
		Scheme:  runtimeScheme,
		Manager: nil}

	// Initial - blocked state 0
	requireZiuStage(t, 0, clnt)

	isZiuRequired, err := v1alpha1.IsZiuRequired(clnt)
	require.NoError(t, err)
	require.Equal(t, false, isZiuRequired)

	// Update Manager with new Container tag
	initialData["ManagerList"] = GetTestData(targetVersion, "ManagerList")
	clnt = fake.NewFakeClientWithScheme(reconcileManager.Scheme, getObjectsList(initialData)...)
	reconcileManager.Client = clnt

	// Check initial containers tag
	requireAllStsTag(t, initialVersion, reconcileManager)
	// TODO: not implemented change of pod
	// requireAllPodsTag(t, initialVersion, reconcileManager)

	// Check ZIU is required
	isZiuRequired, err = v1alpha1.IsZiuRequired(clnt)
	require.NoError(t, err)
	require.Equal(t, true, isZiuRequired)

	// Start ZIU
	require.NoError(t, v1alpha1.InitZiu(clnt))

	// Check analytics db name
	var adbName string
	adbName, err = v1alpha1.GetAnalyticsCassandraInstance(clnt)
	require.NoError(t, err)
	if initialVersion == "2011" {
		require.Equal(t, "configdb1", adbName)
	} else {
		require.Equal(t, "analyticsdb1", adbName)
	}

	// Go over ziu stages
	end := len(v1alpha1.ZiuKinds)
	noErrFlag := true
	for i := 0; noErrFlag && i < end; i++ {
		t.Log("ZIU stage", i, ziuObjectKind(i), ziuObjectNames(i))

		// set updated replicas to 0 to allow 2nd check of reconcile that verify replicas
		require.NoError(t, setUpdatedReplicas(i, 0, reconcileManager))

		// 1st run manager - check stage moved forward once
		t.Logf("Reconcile manager 1")
		result, err := reconcileManager.Reconcile(reconcileRequest)
		require.NoError(t, err)
		require.Equal(t, ziuReconcielResult, result)
		t.Logf("Check ziu stage changed to %v", i+1)
		requireZiuStage(t, i+1, clnt)

		// 2nd run manager - ziu stage should not be changed till STS-es updated byt controllers
		t.Logf("Reconcile manager 2")
		result, err = reconcileManager.Reconcile(reconcileRequest)
		require.NoError(t, err)
		require.Equal(t, ziuReconcielResult, result)
		t.Logf("Check ziu stage unchanged %v", i+1)
		requireZiuStage(t, i+1, clnt)

		// Check STSes unchanged before controllers reconcile
		var toCheck []string
		for j := i; j < end; j++ {
			toCheck = append(toCheck, ziuObjectNames(j)...)
		}
		t.Logf("Check unchanged version '%s' for %+v", initialVersion, toCheck)
		requireServiceStsTag(t, toCheck, initialVersion, reconcileManager)

		oo := &v1alpha1.Analytics{}
		require.NoError(t, reconcileManager.Client.Get(context.TODO(), types.NamespacedName{Name: "analytics1", Namespace: "tf"}, oo))
		t.Logf("DBG: %+v", *oo.Status.Active)

		// run controller
		_, err = runReconcileStage(t, i, reconcileManager)
		require.NoError(t, err)

		// Check updated STSes
		toCheck = []string{}
		for j := 0; j < i+1; j++ {
			toCheck = append(toCheck, ziuObjectNames(j)...)
		}
		t.Logf("Check updated version '%s' for %+v", targetVersion, toCheck)
		requireServiceStsTag(t, toCheck, targetVersion, reconcileManager)

		// 3rd run manager - ziu stage should not be changed till replicas updated
		t.Logf("Reconcile manager 3")
		result, err = reconcileManager.Reconcile(reconcileRequest)
		require.NoError(t, err)
		require.Equal(t, ziuReconcielResult, result)
		t.Logf("Check ziu stage unchanged %v", i+1)
		requireZiuStage(t, i+1, clnt)

		// Set updated replicas equal to replicas to move forward
		err = updateReplicas(i, reconcileManager)
		require.NoError(t, err)

		// Set IsActive in Status to move forward
		updateStatus(t, i, reconcileManager)
	}

	// Run Reconcile one more time to finish ZIU and check ZIU done
	t.Logf("DBG: check ZIU done")
	result, err := reconcileManager.Reconcile(reconcileRequest)
	require.NoError(t, err)
	require.Equal(t, ziuReconcielResult, result)
	requireZiuStage(t, -1, clnt)
	isZiuRequired, err = v1alpha1.IsZiuRequired(clnt)
	require.NoError(t, err)
	require.Equal(t, false, isZiuRequired)

	// Check STSes target containers tag
	requireAllStsTag(t, targetVersion, reconcileManager)
}
