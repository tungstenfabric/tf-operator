package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/operator-framework/operator-sdk/pkg/log/zap"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	_ "github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tungstenfabric/tf-operator/pkg/apis"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	controller_manager "github.com/tungstenfabric/tf-operator/pkg/controller/manager"
)

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func exitOnErr(err error) {
	if err != nil {
		log.Error(err, "Fatal error")
		os.Exit(1)
	}
}

func getObjectKey(ns, name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}
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

func getUnstructuredV1(kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		// Group:   "v1",
		Kind:    kind,
		Version: "v1",
	})
	return u
}

func getUnstructuredObj(ns, kind, name string, clnt client.Client) *unstructured.Unstructured {
	mgr := getUnstructured(kind)
	err := clnt.Get(context.Background(), getObjectKey(ns, name), mgr)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	exitOnErr(err)
	return mgr
}

func getUnstructuredObjV1(ns, kind, name string, clnt client.Client) *unstructured.Unstructured {
	mgr := getUnstructuredV1(kind)
	err := clnt.Get(context.Background(), getObjectKey(ns, name), mgr)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	exitOnErr(err)
	return mgr
}

func resetStatusNodes(ns, kind, name string, clnt client.Client) *unstructured.Unstructured {
	if obj := getUnstructuredObj(ns, kind, name, clnt); obj != nil {
		status := controller_manager.GetChildObject("status", obj.UnstructuredContent())
		log.Info(kind, "name", name, "status", status)
		delete(status, "nodes")
		log.Info(kind, "name", name, "status with reset nodes", status)
		return obj
	}
	return nil
}

func do() {
	//  create k8s client
	cfg := config.GetConfigOrDie()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		namespace = "tf"
	}

	mgr, err := manager.New(cfg, manager.Options{
		Namespace:               namespace,
		MetricsBindAddress:      "0",
		LeaderElection:          true,
		LeaderElectionID:        "tf-manager-lock",
		LeaderElectionNamespace: namespace,
	})
	exitOnErr(err)

	err = apis.AddToScheme(mgr.GetScheme())
	exitOnErr(err)

	clnt, err := client.New(cfg, client.Options{})
	exitOnErr(err)

	// hack status/nodes - 21.4 has differen structure that leads to unmarshall
	// error during ziu 2011 => 21.4
	servicesMap := map[string][]string{
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
	log.Info("Remove nodes from services status")
	for kind, services := range servicesMap {
		for _, name := range services {
			log.Info(kind, name, "read object")
			obj := resetStatusNodes(namespace, kind, name, clnt)
			if obj == nil {
				log.Info(kind, name, "not found - skip")
				continue
			}
			log.Info(kind, name, "update status")
			err = clnt.Status().Update(context.TODO(), obj)
			exitOnErr(err)
			log.Info(kind, name, "done")
		}
	}

	log.Info("Get manager object")
	mgr_obj, err := v1alpha1.GetManagerObject(clnt)
	exitOnErr(err)
	log.Info("DEBUG", "manager", mgr_obj)

	var active bool = true

	// hack redis - 2011 misses redis service, that blocks ziu process for analytics step
	log.Info("Create Redis objects")
	redises, err := controller_manager.ProcessRedis(mgr_obj, clnt, mgr.GetScheme())
	exitOnErr(err)
	for _, redis := range redises {
		if redis.Status.Active == nil {
			log.Info(fmt.Sprintf("Set initial status for Redis object %s", redis.Name))
			redis.Status.Active = &active
			err = clnt.Status().Update(context.TODO(), redis)
			exitOnErr(err)
		}
	}
	log.Info("Redis", "created objects", redises)

	// hack analyticsdb - 2011 misses analyticsdb service, that blocks ziu process for analytics step
	log.Info("Create Cassandra objects")
	dbs, err := controller_manager.ProcessCassandras(mgr_obj, clnt, mgr.GetScheme())
	exitOnErr(err)
	for _, db := range dbs {
		if db.Status.Active == nil {
			log.Info(fmt.Sprintf("Set initial status for DB object %s", db.Name))
			db.Status.Active = &active
			err = clnt.Status().Update(context.TODO(), db)
			exitOnErr(err)
		}
	}
	log.Info("Cassandra", "created objects", dbs)

	// hack queryengine - 2011 misses queryengine service, that blocks ziu process
	log.Info("QueryEngine create object")
	queryengine, err := controller_manager.ProcessQueryEngine(mgr_obj, clnt, mgr.GetScheme())
	exitOnErr(err)
	if queryengine.Status.Active == nil {
		log.Info("Set initial status for QueryEngine object")
		queryengine.Status.Active = &active
		err = clnt.Status().Update(context.TODO(), queryengine)
		exitOnErr(err)
	}
	log.Info("QueryEngine", "created object", queryengine)

	// hack analytics - 2011 misses analytics service, that blocks ziu process
	log.Info("Create Analytics object")
	analytics, err := controller_manager.ProcessAnalytics(mgr_obj, clnt, mgr.GetScheme())
	exitOnErr(err)
	if analytics.Status.Active == nil {
		log.Info("Set initial status for Analytics object")
		// inactive as it has no dependecies before in ziu list
		analytics.Status.Active = &active
		err = clnt.Status().Update(context.TODO(), analytics)
		exitOnErr(err)
	}
	log.Info("Analytics", "created object", analytics)

	// hack configmap - 2011 misses annotation that leads to panic in case of 2011 => 21.4
	const ca_config_map = "csr-signer-ca"
	if obj := getUnstructuredObjV1(namespace, "ConfigMap", ca_config_map, clnt); obj != nil {
		log.Info(fmt.Sprintf("CA config map %s found", ca_config_map))
		uc := obj.UnstructuredContent()
		metadata := controller_manager.GetChildObject("metadata", uc)
		if _, ok := metadata["annotations"]; !ok {
			log.Info(fmt.Sprintf("CA config map %s has no annotations - create it", ca_config_map))
			metadata["annotations"] = map[string]string{"fake": "fake"}
			err = clnt.Update(context.TODO(), obj)
			exitOnErr(err)
			log.Info(fmt.Sprintf("CA config map %s annotations created", ca_config_map))
		}
	}
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime).
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	var repeate int
	var delay int
	pflag.CommandLine.IntVar(&repeate, "repeate", 1, "number of repeats")
	pflag.CommandLine.IntVar(&delay, "delay", 5, "delay in seconds between repeats")

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	log.Info(fmt.Sprintf("repeate %d times", repeate))
	log.Info(fmt.Sprintf("deleay between repeats %d seconds", delay))

	printVersion()

	for i := 0; i < repeate; i++ {
		log.Info("Repeate", "i", i)
		do()
		log.Info("Sleep", "seconds", delay)
		time.Sleep(time.Duration(time.Second * time.Duration(delay)))
	}

	os.Exit(0)
}
