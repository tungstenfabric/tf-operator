package zookeeper

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1"
	"github.com/tungstenfabric/tf-operator/pkg/controller/utils"
	"github.com/tungstenfabric/tf-operator/pkg/label"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("controller_zookeeper")
var restartTime, _ = time.ParseDuration("1s")
var requeueReconcile = reconcile.Result{Requeue: true, RequeueAfter: restartTime}

func resourceHandler(myclient client.Client) handler.Funcs {
	appHandler := handler.Funcs{
		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
			listOps := &client.ListOptions{Namespace: e.Meta.GetNamespace()}
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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
			list := &v1alpha1.ZookeeperList{}
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

func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileZookeeper{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler.
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller

	c, err := controller.New("zookeeper-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Zookeeper{}},
		&handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	serviceMap := map[string]string{"contrail_manager": "zookeeper"}
	srcPod := &source.Kind{Type: &corev1.Pod{}}
	podHandler := resourceHandler(mgr.GetClient())
	predInitStatus := utils.PodInitStatusChange(serviceMap)
	predPodIPChange := utils.PodIPChange(serviceMap)
	predInitRunning := utils.PodInitRunning(serviceMap)

	if err = c.Watch(srcPod, podHandler, predPodIPChange); err != nil {
		return err
	}
	if err = c.Watch(srcPod, podHandler, predInitStatus); err != nil {
		return err
	}
	if err = c.Watch(srcPod, podHandler, predInitRunning); err != nil {
		return err
	}

	srcSTS := &source.Kind{Type: &appsv1.StatefulSet{}}
	stsHandler := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Zookeeper{},
	}
	stsPred := utils.STSStatusChange(utils.ZookeeperGroupKind())
	if err = c.Watch(srcSTS, stsHandler, stsPred); err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileZookeeper implements reconcile.Reconciler.
var _ reconcile.Reconciler = &ReconcileZookeeper{}

// ReconcileZookeeper reconciles a Zookeeper object.
type ReconcileZookeeper struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver.
	Client client.Client
	Scheme *runtime.Scheme
}

// Reconcile reconciles zookeeper.
func (r *ReconcileZookeeper) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithName("Reconcile").WithName(request.Name)
	reqLogger.Info("Reconciling Zookeeper")
	instanceType := "zookeeper"
	instance := &v1alpha1.Zookeeper{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !instance.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	configMapName := instance.Name + "-zookeeper-conf"
	if _, err := instance.CreateConfigMap(configMapName, r.Client, r.Scheme, request); err != nil {
		return reconcile.Result{}, err
	}

	statefulSet := GetSTS()
	if err := instance.PrepareSTS(statefulSet, &instance.Spec.CommonConfiguration, request, r.Scheme); err != nil {
		return reconcile.Result{}, err
	}
	if err = v1alpha1.EnsureServiceAccount(&statefulSet.Spec.Template.Spec, instanceType, r.Client, request, r.Scheme, instance); err != nil {
		return reconcile.Result{}, err
	}

	instance.AddVolumesToIntendedSTS(statefulSet, map[string]string{configMapName: configMapName})

	zookeeperDefaultConfiguration := instance.ConfigurationParameters()

	for idx := range statefulSet.Spec.Template.Spec.Containers {

		container := &statefulSet.Spec.Template.Spec.Containers[idx]
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		container.Image = instanceContainer.Image

		if container.Name == "zookeeper" {
			if instanceContainer.Command == nil {
				command := []string{"bash", "-c",
					configurationInitCommand +
						"zkServer.sh --config /var/lib/zookeeper start-foreground",
				}
				container.Command = command
			} else {
				container.Command = instanceContainer.Command
			}
			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{
					Name:      configMapName,
					MountPath: "/zookeeper-conf",
				})
		}

	}
	statefulSet.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement

	// Configure InitContainers.
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
	for idx, container := range statefulSet.Spec.Template.Spec.InitContainers {
		instanceContainer := utils.GetContainerFromList(container.Name, instance.Spec.ServiceConfiguration.Containers)
		(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Image = instanceContainer.Image
		if instanceContainer.Command != nil {
			(&statefulSet.Spec.Template.Spec.InitContainers[idx]).Command = instanceContainer.Command
		}
		if container.Name == "init" {
			// nothing to do
		}
	}

	v1alpha1.AddCommonVolumes(&statefulSet.Spec.Template.Spec)

	if created, err := instance.CreateSTS(statefulSet, instanceType, request, r.Client); err != nil || created {
		if err != nil {
			return reconcile.Result{}, err
		}
		return requeueReconcile, err
	}

	if updated, err := instance.UpdateSTS(statefulSet, instanceType, request, r.Client); err != nil || updated {
		if err != nil {
			return reconcile.Result{}, err
		}
		return requeueReconcile, nil
	}

	podIPList, podIPMap, err := instance.PodIPListAndIPMapFromInstance(instanceType, request, r.Client)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(podIPList) > 0 {
		if err = instance.InstanceConfiguration(request, configMapName, podIPList, r.Client); err != nil {
			return reconcile.Result{}, err
		}
		if err = instance.SetPodsToReady(podIPList, r.Client); err != nil {
			return reconcile.Result{}, err
		}

		pods := make([]corev1.Pod, len(podIPList))
		copy(pods, podIPList)
		sort.SliceStable(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

		var found *corev1.Pod
		for _, pod := range pods {
			ip, ok := instance.Status.Nodes[pod.Name]
			if !ok || ip != pod.Status.PodIP {
				found = &pod
			}
			if found != nil {
				break
			}
		}

		if found != nil && len(pods) > 1 {
			myidString := found.Name[len(found.Name)-1:]
			myidInt, err := strconv.Atoi(myidString)
			if err != nil {
				return reconcile.Result{}, err
			}

			serverDef := fmt.Sprintf("server.%d=%s:%s;%s:2181",
				myidInt+1, found.Status.PodIP,
				strconv.Itoa(*zookeeperDefaultConfiguration.ElectionPort)+":"+strconv.Itoa(*zookeeperDefaultConfiguration.ServerPort),
				found.Status.PodIP)
			runScript := fmt.Sprintf("zkCli.sh -server %s reconfig -add \"%s\"", found.Status.PodIP, serverDef)
			command := []string{"bash", "-c", runScript, serverDef}
			if sout, serr, err := v1alpha1.ExecCmdInContainer(found, "zookeeper", command); err != nil {
				reqLogger.Error(err, "Zookeeper reconfig failed", "out", sout, "err", serr)
				return requeueReconcile, err
			}
		}

		if err = instance.ManageNodeStatus(podIPMap, r.Client); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err = r.ensurePodDisruptionBudgetExists(instance); err != nil {
		return reconcile.Result{}, err
	}

	if instance.Status.Active == nil {
		active := false
		instance.Status.Active = &active
	}
	if err = instance.SetInstanceActive(r.Client, instance.Status.Active, statefulSet, request); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
func (r *ReconcileZookeeper) ensurePodDisruptionBudgetExists(zookeeper *v1alpha1.Zookeeper) error {
	pdb := &policy.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zookeeper.Name + "-zookeeper",
			Namespace: zookeeper.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, pdb, func() error {
		oneVal := intstr.FromInt(1)
		pdb.ObjectMeta.Labels = label.New("zookeeper", zookeeper.Name)
		pdb.Spec.MaxUnavailable = &oneVal
		pdb.Spec.Selector = metav1.SetAsLabelSelector(label.New("zookeeper", zookeeper.Name))
		return controllerutil.SetControllerReference(zookeeper, pdb, r.Scheme)
	})

	return err
}

const configurationInitCommand = `#!/bin/sh
function link_file() {
  local src=/zookeeper-conf/$1
  local dst=/var/lib/zookeeper/${2:-${1}}
  echo "INFO: $(date): wait for $src"
  while [ ! -e $src ] ; do sleep 1; done
  echo "INFO: $(date): link $src => $dst"
  rm -f $dst
  ln -sf $src $dst
}

link_file log4j.properties
link_file configuration.xsl
link_file zoo.cfg
link_file zoo.cfg.dynamic.$POD_IP zoo.cfg.dynamic
link_file myid.$POD_IP myid

`
