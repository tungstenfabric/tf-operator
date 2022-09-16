package manager

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	_ "github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-logr/logr"
)

func getUnstructuredV1(kind string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    kind,
		Version: "v1",
	})
	return u
}

func getUnstructuredObj(ns, kind, name string, clnt client.Client) (*unstructured.Unstructured, error) {
	mgr := getUnstructured(kind)
	err := clnt.Get(context.Background(), getObjectKey(ns, name), mgr)
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	return mgr, err
}

func getUnstructuredObjV1(ns, kind, name string, clnt client.Client) (*unstructured.Unstructured, error) {
	mgr := getUnstructuredV1(kind)
	err := clnt.Get(context.Background(), getObjectKey(ns, name), mgr)
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	return mgr, err
}

func resetStatusNodes(ns, kind, name string, clnt client.Client) (*unstructured.Unstructured, error) {
	ll := log.WithName("resetStatusNodes")
	obj, err := getUnstructuredObj(ns, kind, name, clnt)
	if obj != nil {
		status := GetChildObject("status", obj.UnstructuredContent())
		ll.Info(kind, "name", name, "status", status)
		delete(status, "nodes")
		delete(status, "agents")
		var active bool = true
		status["active"] = active
		ll.Info(kind, "name", name, "status with reset nodes", status)
	}
	return obj, err
}

func EnableZiu2011(namespace string, clnt client.Client, scheme *runtime.Scheme, log logr.Logger) error {
	ll := log.WithName("EnableZiu2011Prep")

	kinds := append(v1alpha1.ZiuKindsAll, "Vrouter")
	for _, k := range kinds {
		if err := enableZiu2011ForCR(k, namespace, clnt, scheme, log); err != nil {
			return err
		}
	}

	ll.Info("Get manager")
	mgr_obj, err := v1alpha1.GetManagerObject(clnt)
	if err != nil {
		ll.Error(err, "Failed to get Manager")
	}

	var active bool = true

	// hack redis - 2011 misses redis service, that blocks ziu process for analytics step
	ll.Info("Ensure Redis")
	redises, err := ProcessRedis(mgr_obj, clnt, scheme)
	if err != nil {
		ll.Error(err, "ProcessRedis failed")
		return err
	}
	for _, redis := range redises {
		if redis.Status.Active == nil {
			redis.Status.Active = &active
			if err = clnt.Status().Update(context.TODO(), redis); err != nil {
				ll.WithName(redis.Name).Error(err, "Failed to update")
				return err
			}
		}
	}

	// hack analyticsdb - 2011 misses analyticsdb service, that blocks ziu process for analytics step
	ll.Info("Ensure Cassandra")
	dbs, err := ProcessCassandras(mgr_obj, clnt, scheme)
	if err != nil {
		ll.Error(err, "ProcessCassandras failed")
		return err
	}
	for _, db := range dbs {
		if db.Status.Active == nil {
			db.Status.Active = &active
			if err = clnt.Status().Update(context.TODO(), db); err != nil {
				ll.WithName(db.Name).Error(err, "Failed to update")
				return err
			}
		}
	}

	// hack queryengine - 2011 misses queryengine service, that blocks ziu process
	ll.Info("Ensure QueryEngine")
	queryengine, err := ProcessQueryEngine(mgr_obj, clnt, scheme)
	if err != nil {
		ll.Error(err, "ProcessQueryEngine failed")
		return err
	}
	if queryengine.Status.Active == nil {
		queryengine.Status.Active = &active
		if err = clnt.Status().Update(context.TODO(), queryengine); err != nil {
			ll.WithName(queryengine.Name).Error(err, "Failed to update")
			return err
		}
	}

	// hack analytics - 2011 misses analytics service, that blocks ziu process
	ll.Info("Ensure Analytics")
	analytics, err := ProcessAnalytics(mgr_obj, clnt, scheme)
	if err != nil {
		ll.Error(err, "ProcessAnalytics failed")
		return err
	}
	if analytics.Status.Active == nil {
		analytics.Status.Active = &active
		if err = clnt.Status().Update(context.TODO(), analytics); err != nil {
			ll.WithName(analytics.Name).Error(err, "Failed to update")
			return err
		}
	}

	// hack configmap - 2011 misses annotation that leads to panic in case of 2011 => 21.4
	const ca_config_map = "csr-signer-ca"
	csr, err := getUnstructuredObjV1(namespace, "ConfigMap", ca_config_map, clnt)
	if err != nil {
		ll.WithName(ca_config_map).Error(err, "Failed to get object")
		return err
	}
	if csr != nil {
		ll.WithName(ca_config_map).Info("Ensure")
		uc := csr.UnstructuredContent()
		metadata := GetChildObject("metadata", uc)
		if _, ok := metadata["annotations"]; !ok {
			ll.Info(fmt.Sprintf("CA config map %s has no annotations - create it", ca_config_map))
			metadata["annotations"] = map[string]string{"fake": "fake"}
			if err = clnt.Update(context.TODO(), csr); err != nil {
				ll.WithName(ca_config_map).Error(err, "Failed to update")
				return err
			}
		}
	}

	ll.Info("Done")
	return nil
}

func enableZiu2011ForCR(kind, namespace string, clnt client.Client, scheme *runtime.Scheme, log logr.Logger) error {
	ll := log.WithName("enableZiu2011ForCR").WithName(kind)
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
	services, ok := servicesMap[kind]
	if !ok {
		log.Info("Nothing to patch")
		return nil
	}
	for _, name := range services {
		ll.WithName(name).Info("Patch status")
		obj, err := resetStatusNodes(namespace, kind, name, clnt)
		if err != nil {
			ll.WithName(name).Error(err, "Failed to reset status")
			return err
		}
		if obj == nil {
			ll.WithName(name).Info("Not found - skip")
			continue
		}
		ll.WithName(name).Info(kind, name, "Update status")
		if err = clnt.Status().Update(context.TODO(), obj); err != nil {
			ll.WithName(name).Error(err, "Failed to update object")
			return err
		}
	}
	ll.Info("Done")
	return nil
}

func EnableZiu2011ForCR(kind, namespace string, clnt client.Client, scheme *runtime.Scheme, log logr.Logger) error {
	if err := enableZiu2011ForCR(kind, namespace, clnt, scheme, log); err != nil {
		return err
	}
	if kind == "QueryEngine" || kind == "Analytics" || kind == "AnalyticsAlarm" {
		// Kinds depend on redis, so patch it one more times to ensure it is active
		if err := enableZiu2011ForCR("Redis", namespace, clnt, scheme, log); err != nil {
			return err
		}
	}
	return nil
}
