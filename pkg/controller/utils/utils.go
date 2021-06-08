package utils

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
)

var log = logf.Log.WithName("utils")
var reqLogger = log.WithValues()

// const defines the Group constants.
const (
	ANALYTICS      = "Analytics.tf.tungsten.io"
	QUERYENGINE    = "QueryEngine.tf.tungsten.io"
	ANALYTICSSNMP  = "AnalyticsSnmp.tf.tungsten.io"
	ANALYTICSALARM = "AnalyticsAlarm.tf.tungsten.io"
	CASSANDRA      = "Cassandra.tf.tungsten.io"
	ZOOKEEPER      = "Zookeeper.tf.tungsten.io"
	RABBITMQ       = "Rabbitmq.tf.tungsten.io"
	REDIS          = "Redis.tf.tungsten.io"
	CONFIG         = "Config.tf.tungsten.io"
	CONTROL        = "Control.tf.tungsten.io"
	WEBUI          = "Webui.tf.tungsten.io"
	VROUTER        = "Vrouter.tf.tungsten.io"
	KUBEMANAGER    = "Kubemanager.tf.tungsten.io"
	MANAGER        = "Manager.tf.tungsten.io"
	REPLICASET     = "ReplicaSet.apps"
	DEPLOYMENT     = "Deployment.apps"
)

func RemoveIndex(s []corev1.Container, index int) []corev1.Container {
	return append(s[:index], s[index+1:]...)
}

// AnalyticsGroupKind returns group kind.
func AnalyticsGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(ANALYTICS)
}

// QueryEngineGroupKind returns group kind.
func QueryEngineGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(QUERYENGINE)
}

// AnalyticsSnmpGroupKind returns group kind.
func AnalyticsSnmpGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(ANALYTICSSNMP)
}

// AnalyticsAlarmGroupKind returns group kind.
func AnalyticsAlarmGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(ANALYTICSALARM)
}

// WebuiGroupKind returns group kind.
func WebuiGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(WEBUI)
}

// VrouterGroupKind returns group kind.
func VrouterGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(VROUTER)
}

// ControlGroupKind returns group kind.
func ControlGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(CONTROL)
}

// ConfigGroupKind returns group kind.
func ConfigGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(CONFIG)
}

// KubemanagerGroupKind returns group kind.
func KubemanagerGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(KUBEMANAGER)
}

// CassandraGroupKind returns group kind.
func CassandraGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(CASSANDRA)
}

// ZookeeperGroupKind returns group kind.
func ZookeeperGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(ZOOKEEPER)
}

// RabbitmqGroupKind returns group kind.
func RabbitmqGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(RABBITMQ)
}

// RedisGroupKind returns group kind.
func RedisGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(REDIS)
}

// ReplicaSetGroupKind returns group kind.
func ReplicaSetGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(REPLICASET)
}

// ManagerGroupKind returns group kind.
func ManagerGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(MANAGER)
}

// DeploymentGroupKind returns group kind.
func DeploymentGroupKind() schema.GroupKind {
	return schema.ParseGroupKind(DEPLOYMENT)
}

// DeploymentStatusChange monitors per application size change.
func DeploymentStatusChange(appGroupKind schema.GroupKind) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldDeployment, ok := e.ObjectOld.(*appsv1.Deployment)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newDeployment, ok := e.ObjectNew.(*appsv1.Deployment)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			isOwner := false
			for _, owner := range newDeployment.ObjectMeta.OwnerReferences {
				if *owner.Controller {
					groupVersionKind := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
					if appGroupKind == groupVersionKind.GroupKind() {
						isOwner = true
					}
				}
			}
			if (oldDeployment.Status.ReadyReplicas != newDeployment.Status.ReadyReplicas) && isOwner {
				return true
			}
			return false
		},
	}
}

// STSStatusChange monitors per application size change.
func STSStatusChange(appGroupKind schema.GroupKind) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSTS, ok := e.ObjectOld.(*appsv1.StatefulSet)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newSTS, ok := e.ObjectNew.(*appsv1.StatefulSet)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			isOwner := false
			for _, owner := range newSTS.ObjectMeta.OwnerReferences {
				if *owner.Controller {
					groupVersionKind := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
					if appGroupKind == groupVersionKind.GroupKind() {
						isOwner = true
					}
				}
			}
			if (oldSTS.Status.ReadyReplicas != newSTS.Status.ReadyReplicas) && isOwner {
				return true
			}
			return false
		},
	}
}

// DSStatusChange monitors per application size change.
func DSStatusChange(appGroupKind schema.GroupKind) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldDS, ok := e.ObjectOld.(*appsv1.DaemonSet)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newDS, ok := e.ObjectNew.(*appsv1.DaemonSet)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			isOwner := false
			for _, owner := range newDS.ObjectMeta.OwnerReferences {
				if *owner.Controller {
					groupVersionKind := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
					if appGroupKind == groupVersionKind.GroupKind() {
						isOwner = true
					}
				}
			}
			if (oldDS.Status.NumberReady != newDS.Status.NumberReady) && isOwner {
				return true
			}
			return false
		},
	}
}

// PodIPChange returns predicate function based on group kind.
func PodIPChange(appLabel map[string]string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			for key, value := range e.MetaOld.GetLabels() {
				if appLabel[key] == value {
					oldPod, ok := e.ObjectOld.(*corev1.Pod)
					if !ok {
						reqLogger.Info("type conversion mismatch")
					}
					newPod, ok := e.ObjectNew.(*corev1.Pod)
					if !ok {
						reqLogger.Info("type conversion mismatch")
					}
					return oldPod.Status.PodIP != newPod.Status.PodIP
				}
			}
			return false
		},
	}
}

func statusChange(pold, pnew *bool) bool {
	newActive := false
	if pnew != nil {
		newActive = *pnew
	}
	oldActive := false
	if pold != nil {
		oldActive = *pold
	}
	return !oldActive && newActive
}

// CassandraActiveChange returns predicate function based on group kind.
func CassandraActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCassandra, ok := e.ObjectOld.(*v1alpha1.Cassandra)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newCassandra, ok := e.ObjectNew.(*v1alpha1.Cassandra)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldCassandra.Status.Active, newCassandra.Status.Active)
		},
	}
}

// ConfigActiveChange returns predicate function based on group kind.
func ConfigActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConfig, ok := e.ObjectOld.(*v1alpha1.Config)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newConfig, ok := e.ObjectNew.(*v1alpha1.Config)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldConfig.Status.Active, newConfig.Status.Active)
		},
	}
}

// AnalyticsActiveChange returns predicate function based on group kind.
func AnalyticsActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldAnalytics, ok := e.ObjectOld.(*v1alpha1.Analytics)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newAnalytics, ok := e.ObjectNew.(*v1alpha1.Analytics)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldAnalytics.Status.Active, newAnalytics.Status.Active)
		},
	}
}

// VrouterActiveChange returns predicate function based on group kind.
func VrouterActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldVrouter, ok := e.ObjectOld.(*v1alpha1.Vrouter)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newVrouter, ok := e.ObjectNew.(*v1alpha1.Vrouter)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldVrouter.Status.Active, newVrouter.Status.Active)
		},
	}
}

// ControlActiveChange returns predicate function based on group kind.
func ControlActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldControl, ok := e.ObjectOld.(*v1alpha1.Control)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newControl, ok := e.ObjectNew.(*v1alpha1.Control)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldControl.Status.Active, newControl.Status.Active)
		},
	}
}

// RabbitmqActiveChange returns predicate function based on group kind.
func RabbitmqActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRabbitmq, ok := e.ObjectOld.(*v1alpha1.Rabbitmq)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newRabbitmq, ok := e.ObjectNew.(*v1alpha1.Rabbitmq)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldRabbitmq.Status.Active, newRabbitmq.Status.Active)
		},
	}
}

// RedisActiveChange returns predicate function based on group kind.
func RedisActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRedis, ok := e.ObjectOld.(*v1alpha1.Redis)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newRedis, ok := e.ObjectNew.(*v1alpha1.Redis)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldRedis.Status.Active, newRedis.Status.Active)
		},
	}
}

// ZookeeperActiveChange returns predicate function based on group kind.
func ZookeeperActiveChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldZookeeper, ok := e.ObjectOld.(*v1alpha1.Zookeeper)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			newZookeeper, ok := e.ObjectNew.(*v1alpha1.Zookeeper)
			if !ok {
				reqLogger.Info("type conversion mismatch")
			}
			return statusChange(oldZookeeper.Status.Active, newZookeeper.Status.Active)
		},
	}
}

// Get two unstructured object: manager's CommonConfiguration and resource specific
// CommonConfiguration from manager manifest.
// Set or replace fields in CommonConfig by valyes from specific config and return result
// structure
func MergeUnstructuredCommonConfig(commonConfig map[string]interface{},
	resourceConfig map[string]interface{}) map[string]interface{} {
	for field, content := range resourceConfig {
		commonConfig[field] = content
	}
	return commonConfig
}

// MergeCommonConfiguration combines common configuration of manager and service.
func MergeCommonConfiguration(manager v1alpha1.ManagerConfiguration,
	instance v1alpha1.PodConfiguration) v1alpha1.PodConfiguration {
	if len(instance.NodeSelector) == 0 && len(manager.NodeSelector) > 0 {
		instance.NodeSelector = manager.NodeSelector
	}
	if len(instance.ImagePullSecrets) == 0 && len(manager.ImagePullSecrets) > 0 {
		instance.ImagePullSecrets = manager.ImagePullSecrets
	}
	if len(instance.Tolerations) == 0 && len(manager.Tolerations) > 0 {
		instance.Tolerations = manager.Tolerations
	}
	if instance.TuneSysctl == nil {
		if manager.TuneSysctl != nil {
			instance.TuneSysctl = manager.TuneSysctl
		} else {
			trueVal := true
			instance.TuneSysctl = &trueVal
		}
	}
	if instance.AuthParameters == nil && manager.AuthParameters != nil {
		instance.AuthParameters = manager.AuthParameters
	}
	if instance.LogLevel == "" && manager.LogLevel != "" {
		instance.LogLevel = manager.LogLevel
	}
	if instance.Distribution == nil && manager.Distribution != nil {
		instance.Distribution = manager.Distribution
	}
	return instance
}

// GetContainerFromList gets a container from a list of container
func GetContainerFromList(containerName string, containerList []*v1alpha1.Container) *v1alpha1.Container {
	for _, instanceContainer := range containerList {
		if containerName == instanceContainer.Name {
			return instanceContainer
		}
	}
	return nil
}

// PodPhaseChanges Check if some labeled pods switch to Running or from Running to another phase
func PodPhaseChanges(podLabels map[string]string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// TODO Select our pods using labels
			for key, value := range e.MetaOld.GetLabels() {
				if podLabels[key] == value {
					oldPod, ok := e.ObjectOld.(*corev1.Pod)
					if !ok {
						reqLogger.Info("type conversion mismatch")
					}
					newPod, ok := e.ObjectNew.(*corev1.Pod)
					if !ok {
						reqLogger.Info("type conversion mismatch")
					}
					if (newPod.Status.Phase == "Running" && oldPod.Status.Phase != "Running") ||
						(newPod.Status.Phase != "Running" && oldPod.Status.Phase == "Running") {
						return true
					}
				}
			}
			return false
		},
	}
}
