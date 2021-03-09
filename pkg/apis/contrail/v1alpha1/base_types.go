package v1alpha1

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
	"github.com/tungstenfabric/tf-operator/pkg/k8s"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Container defines name, image and command.
// +k8s:openapi-gen=true
type Container struct {
	Name    string   `json:"name,omitempty"`
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
}

// ServiceStatus provides information on the current status of the service.
// +k8s:openapi-gen=true
type ServiceStatus struct {
	Name    *string `json:"name,omitempty"`
	Active  *bool   `json:"active,omitempty"`
	Created *bool   `json:"created,omitempty"`
}

// PodConfiguration is the common services struct.
// +k8s:openapi-gen=true
type PodConfiguration struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	// Host networking requested for this pod. Use the host's network namespace.
	// If this option is set, the ports that will be used must be specified.
	// Default to false.
	// +k8s:conversion-gen=false
	// +optional
	HostNetwork *bool `json:"hostNetwork,omitempty" protobuf:"varint,11,opt,name=hostNetwork"`
	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified.
	// +optional
	// +patchMergeKey=ip
	// +patchStrategy=merge
	HostAliases []corev1.HostAlias `json:"hostAliases,omitempty" patchStrategy:"merge" patchMergeKey:"ip" protobuf:"bytes,23,rep,name=hostAliases"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	// Use 0.0.0.0 for isntrospection ports
	// +optional
	IntrospectListenAll *bool `json:"introspectListenAll,omitempty"`
	// AuthParameters auth parameters
	// +optional
	AuthParameters *AuthParameters `json:"authParameters,omitempty"`
	// Kubernetes Cluster Configuration
	// +kubebuilder:validation:Enum=info;debug;warning;error;critical;none
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
}

//GetReplicas is used to get number of desired pods.
func (cc *PodConfiguration) GetReplicas() int32 {
	if cc.Replicas != nil {
		return *cc.Replicas
	}
	return int32(1)
}

// IntrospectionListenAddress returns listen address for instrospection
func (cc *PodConfiguration) IntrospectionListenAddress(addr string) string {
	f := IntrospectListenAll
	if cc != nil && cc.IntrospectListenAll != nil {
		f = *cc.IntrospectListenAll
	}
	if f {
		return "0.0.0.0"
	}
	return addr
}

func (ss *ServiceStatus) ready() bool {
	if ss == nil {
		return false
	}
	if ss.Active == nil {
		return false
	}

	return *ss.Active

}

// ensureSecret creates Secret for a service account
func ensureSecret(serviceAccountName, secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	owner v1.Object) error {

	namespace := owner.GetNamespace()
	existingSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, existingSecret)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": serviceAccountName,
			},
		},
		Type: corev1.SecretType("kubernetes.io/service-account-token"),
	}
	err = controllerutil.SetControllerReference(owner, secret, scheme)
	if err != nil {
		return err
	}
	return client.Create(context.TODO(), secret)
}

// ensureClusterRole creates ClusterRole
func ensureClusterRole(clusterRoleName string,
	client client.Client,
	scheme *runtime.Scheme,
	owner v1.Object) error {

	existingClusterRole := &rbacv1.ClusterRole{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, existingClusterRole)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	namespace := owner.GetNamespace()
	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs: []string{
				"*",
			},
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
		}},
	}
	return client.Create(context.TODO(), clusterRole)
}

// ensureClusterRoleBinding creates ClusterRole binding
func ensureClusterRoleBinding(
	serviceAccountName, clusterRoleName, clusterRoleBindingName string,
	client client.Client,
	scheme *runtime.Scheme,
	owner v1.Object) error {

	namespace := owner.GetNamespace()
	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleBindingName}, existingClusterRoleBinding)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRoleBindingName,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}
	return client.Create(context.TODO(), clusterRoleBinding)
}

// ensureServiceAccount creates ServiceAccoung, Secret, ClusterRole and ClusterRoleBinding objects
func ensureServiceAccount(
	serviceAccountName string,
	clusterRoleName string,
	clusterRoleBindingName string,
	secretName string,
	imagePullSecret []string,
	client client.Client,
	scheme *runtime.Scheme,
	owner v1.Object) error {

	if err := ensureSecret(serviceAccountName, secretName, client, scheme, owner); err != nil {
		return nil
	}

	namespace := owner.GetNamespace()
	existingServiceAccount := &corev1.ServiceAccount{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: serviceAccountName, Namespace: namespace}, existingServiceAccount)
	if err != nil && k8serrors.IsNotFound(err) {
		serviceAccount := &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets,
			corev1.ObjectReference{
				Kind:      "Secret",
				Namespace: namespace,
				Name:      secretName})
		for _, name := range imagePullSecret {
			serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets,
				corev1.LocalObjectReference{Name: name})
		}

		err = controllerutil.SetControllerReference(owner, serviceAccount, scheme)
		if err != nil {
			return err
		}
		if err = client.Create(context.TODO(), serviceAccount); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	if err := ensureClusterRole(clusterRoleName, client, scheme, owner); err != nil {
		return nil
	}
	if err := ensureClusterRoleBinding(serviceAccountName, clusterRoleName, clusterRoleName, client, scheme, owner); err != nil {
		return nil
	}
	return nil
}

// EnsureServiceAccount prepares the intended podList.
func EnsureServiceAccount(spec *corev1.PodSpec,
	instanceType string,
	imagePullSecret []string,
	client client.Client,
	request reconcile.Request,
	scheme *runtime.Scheme,
	object v1.Object) error {

	baseName := request.Name + "-" + instanceType + "-"
	serviceAccountName := baseName + "service-account"
	err := ensureServiceAccount(
		serviceAccountName,
		baseName+"role",
		baseName+"role-binding",
		baseName+"secret",
		imagePullSecret,
		client, scheme, object)
	if err != nil {
		log.Error(err, "EnsureServiceAccount failed")
		return err
	}
	spec.ServiceAccountName = serviceAccountName
	return nil
}

// SetPodsToReady sets the status label of a POD to ready.
func SetPodsToReady(podList []corev1.Pod, client client.Client) error {
	for _, pod := range podList {
		labels := pod.GetLabels()
		labels["status"] = "ready"
		pod.SetLabels(labels)
		if err := client.Update(context.TODO(), &pod); err != nil {
			return err
		}
	}
	return nil
}

// +k8s:deepcopy-gen=false
type podAltIPsRetriver func(pod corev1.Pod) []string

// PodAlternativeIPs alternative IPs list for cert alt names subject
// +k8s:deepcopy-gen=false
type PodAlternativeIPs struct {
	// Function which operate over pod object
	// to retrieve additional IP addresses used
	// by this pod.
	Retriever podAltIPsRetriver
	// ServiceIP through which pod can be reached.
	ServiceIP string
}

// PodsCertSubjects iterates over passed list of pods and for every pod prepares certificate subject
// which can be later used for generating certificate for given pod.
func PodsCertSubjects(domain string, podList []corev1.Pod, hostNetwork *bool, podAltIPs PodAlternativeIPs) []certificates.CertificateSubject {
	var pods []certificates.CertificateSubject
	useNodeName := true
	if hostNetwork != nil {
		useNodeName = *hostNetwork
	}
	for _, pod := range podList {
		var hostname string
		if useNodeName {
			hostname = pod.Spec.NodeName
		} else {
			hostname = pod.Spec.Hostname
		}
		var alternativeIPs []string
		if podAltIPs.ServiceIP != "" {
			alternativeIPs = append(alternativeIPs, podAltIPs.ServiceIP)
		}
		if podAltIPs.Retriever != nil {
			if altIPs := podAltIPs.Retriever(pod); len(altIPs) > 0 {
				alternativeIPs = append(alternativeIPs, altIPs...)
			}
		}
		podInfo := certificates.NewSubject(pod.Name, domain, hostname, pod.Status.PodIP, alternativeIPs)
		pods = append(pods, podInfo)
	}
	return pods
}

// CreateConfigMap creates a config map based on the instance type.
func CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request,
	instanceType string,
	object v1.Object) (*corev1.ConfigMap, error) {

	configMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: request.Namespace}, configMap)
	if err == nil {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		return configMap, err
	}
	if !k8serrors.IsNotFound(err) {
		return nil, err
	}
	// TODO: Bug. If config map exists without labels and references, they won't be updated
	configMap.SetName(configMapName)
	configMap.SetNamespace(request.Namespace)
	configMap.SetLabels(map[string]string{"contrail_manager": instanceType,
		instanceType: request.Name})
	configMap.Data = make(map[string]string)
	if err = controllerutil.SetControllerReference(object, configMap, scheme); err != nil {
		return nil, err
	}
	if err = client.Create(context.TODO(), configMap); err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	return configMap, nil
}

// CreateSecret creates a secret based on the instance type.
func CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request,
	instanceType string,
	object v1.Object) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: request.Namespace}, secret)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			secret.SetName(secretName)
			secret.SetNamespace(request.Namespace)
			secret.SetLabels(map[string]string{"contrail_manager": instanceType,
				instanceType: request.Name})
			var data = make(map[string][]byte)
			secret.Data = data
			if err = controllerutil.SetControllerReference(object, secret, scheme); err != nil {
				return nil, err
			}
			if err = client.Create(context.TODO(), secret); err != nil && !k8serrors.IsAlreadyExists(err) {
				return nil, err
			}
		}
	}
	return secret, nil
}

// PrepareSTS prepares the intended podList.
func PrepareSTS(sts *appsv1.StatefulSet,
	commonConfiguration *PodConfiguration,
	instanceType string,
	request reconcile.Request,
	scheme *runtime.Scheme,
	object v1.Object,
	usePralallePodManagementPolicy bool) error {

	SetSTSCommonConfiguration(sts, commonConfiguration)
	if usePralallePodManagementPolicy {
		sts.Spec.PodManagementPolicy = appsv1.PodManagementPolicyType("Parallel")
	} else {
		sts.Spec.PodManagementPolicy = appsv1.PodManagementPolicyType("OrderedReady")
	}
	baseName := request.Name + "-" + instanceType
	name := baseName + "-statefulset"
	sts.SetName(name)
	sts.SetNamespace(request.Namespace)
	labels := map[string]string{"contrail_manager": instanceType, instanceType: request.Name}
	sts.SetLabels(labels)
	sts.Spec.Selector.MatchLabels = labels
	sts.Spec.Template.SetLabels(labels)

	if err := controllerutil.SetControllerReference(object, sts, scheme); err != nil {
		return err
	}
	return nil
}

// SetDeploymentCommonConfiguration takes common configuration parameters
// and applies it to the deployment.
func SetDeploymentCommonConfiguration(deployment *appsv1.Deployment,
	commonConfiguration *PodConfiguration) *appsv1.Deployment {
	var replicas = int32(1)
	if commonConfiguration.Replicas != nil {
		replicas = *commonConfiguration.Replicas
	}
	deployment.Spec.Replicas = &replicas
	if len(commonConfiguration.Tolerations) > 0 {
		deployment.Spec.Template.Spec.Tolerations = commonConfiguration.Tolerations
	}
	if len(commonConfiguration.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = commonConfiguration.NodeSelector
	}
	if commonConfiguration.HostNetwork != nil {
		deployment.Spec.Template.Spec.HostNetwork = *commonConfiguration.HostNetwork
	} else {
		deployment.Spec.Template.Spec.HostNetwork = false
	}

	if len(commonConfiguration.HostAliases) > 0 {
		deployment.Spec.Template.Spec.HostAliases = commonConfiguration.HostAliases
	}

	if len(commonConfiguration.ImagePullSecrets) > 0 {
		imagePullSecretList := []corev1.LocalObjectReference{}
		for _, imagePullSecretName := range commonConfiguration.ImagePullSecrets {
			imagePullSecret := corev1.LocalObjectReference{
				Name: imagePullSecretName,
			}
			imagePullSecretList = append(imagePullSecretList, imagePullSecret)
		}
		deployment.Spec.Template.Spec.ImagePullSecrets = imagePullSecretList
	}
	return deployment
}

// SetSTSCommonConfiguration takes common configuration parameters
// and applies it to the pod.
func SetSTSCommonConfiguration(sts *appsv1.StatefulSet,
	commonConfiguration *PodConfiguration) {
	var replicas = int32(1)
	if commonConfiguration.Replicas != nil {
		replicas = *commonConfiguration.Replicas
	}
	sts.Spec.Replicas = &replicas
	if len(commonConfiguration.Tolerations) > 0 {
		sts.Spec.Template.Spec.Tolerations = commonConfiguration.Tolerations
	}
	if len(commonConfiguration.NodeSelector) > 0 {
		sts.Spec.Template.Spec.NodeSelector = commonConfiguration.NodeSelector
	}
	if commonConfiguration.HostNetwork != nil {
		sts.Spec.Template.Spec.HostNetwork = *commonConfiguration.HostNetwork
	} else {
		sts.Spec.Template.Spec.HostNetwork = false
	}

	if len(commonConfiguration.HostAliases) > 0 {
		sts.Spec.Template.Spec.HostAliases = commonConfiguration.HostAliases
	}

	if len(commonConfiguration.ImagePullSecrets) > 0 {
		imagePullSecretList := []corev1.LocalObjectReference{}
		for _, imagePullSecretName := range commonConfiguration.ImagePullSecrets {
			imagePullSecret := corev1.LocalObjectReference{
				Name: imagePullSecretName,
			}
			imagePullSecretList = append(imagePullSecretList, imagePullSecret)
		}
		sts.Spec.Template.Spec.ImagePullSecrets = imagePullSecretList
	}
}

// AddVolumesToIntendedSTS adds volumes to a deployment.
func AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	volumeList := sts.Spec.Template.Spec.Volumes
	for configMapName, volumeName := range volumeConfigMapMap {
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
				},
			},
		}
		volumeList = append(volumeList, volume)
	}
	sts.Spec.Template.Spec.Volumes = volumeList
}

// AddSecretVolumesToIntendedSTS adds volumes to a deployment.
func AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeSecretMap map[string]string) {
	volumeList := sts.Spec.Template.Spec.Volumes
	for secretName, volumeName := range volumeSecretMap {
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumeList = append(volumeList, volume)
	}
	sts.Spec.Template.Spec.Volumes = volumeList
}

// AddSecretVolumesToIntendedDS adds volumes to a deployment.
func AddSecretVolumesToIntendedDS(ds *appsv1.DaemonSet, volumeSecretMap map[string]string) {
	volumeList := ds.Spec.Template.Spec.Volumes
	for secretName, volumeName := range volumeSecretMap {
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}
		volumeList = append(volumeList, volume)
	}
	ds.Spec.Template.Spec.Volumes = volumeList
}

// QuerySTS queries the STS
func QuerySTS(name string, namespace string, reconcileClient client.Client) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	err := reconcileClient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, sts)
	if err != nil {
		return nil, err
	}
	return sts, nil
}

// CreateSTS creates the STS.
func CreateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client) (bool, error) {
	_, err := QuerySTS(request.Name+"-"+instanceType+"-statefulset", request.Namespace, reconcileClient)
	if err == nil {
		return false, nil
	}
	if !k8serrors.IsNotFound(err) {
		return false, err
	}
	sts.Spec.Template.ObjectMeta.Labels["version"] = "1"
	return true, reconcileClient.Create(context.TODO(), sts)
}

// UpdateSTS updates the STS.
func UpdateSTS(sts *appsv1.StatefulSet, instanceType string, request reconcile.Request, reconcileClient client.Client, strategy string) (bool, error) {
	stsName := request.Name + "-" + instanceType + "-statefulset"
	stsLog := log.WithName("UpdateSTS").WithName(stsName)
	currentSTS, err := QuerySTS(stsName, request.Namespace, reconcileClient)
	if currentSTS == nil {
		return false, err
	}
	hasChanges := false
	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = int32(*sts.Spec.Replicas)
	}
	if replicas != *currentSTS.Spec.Replicas {
		stsLog.Info("Replicas changed", "current", *currentSTS.Spec.Replicas, "new", replicas)
		hasChanges = true
	}
	if !hasChanges {
		for _, intendedContainer := range sts.Spec.Template.Spec.Containers {
			for _, currentContainer := range currentSTS.Spec.Template.Spec.Containers {
				if intendedContainer.Name == currentContainer.Name {
					if intendedContainer.Image != currentContainer.Image {
						hasChanges = true
						stsLog.Info("Image changed",
							"container", currentContainer.Name,
							"currentContainer.Image", currentContainer.Image,
							"intendedContainer.Image", intendedContainer.Image,
						)
						break
					}
					if !cmp.Equal(intendedContainer.Env, currentContainer.Env,
						cmpopts.IgnoreFields(corev1.ObjectFieldSelector{}, "APIVersion"),
					) {
						hasChanges = true
						stsLog.Info("Env changed",
							"container", currentContainer.Name,
							"currentContainer.Env", currentContainer.Env,
							"intendedContainer.Env", intendedContainer.Env,
						)
						break
					}
				}
			}
		}
	}
	if !hasChanges {
		return false, nil
	}

	stsLog.Info("Update", "strategy", strategy)

	if strategy == "deleteFirst" {
		versionInt, _ := strconv.Atoi(currentSTS.Spec.Template.ObjectMeta.Labels["version"])
		newVersion := versionInt + 1
		sts.Spec.Template.ObjectMeta.Labels["version"] = strconv.Itoa(newVersion)
	} else {
		sts.Spec.Template.ObjectMeta.Labels["version"] = currentSTS.Spec.Template.ObjectMeta.Labels["version"]
	}

	return true, reconcileClient.Update(context.TODO(), sts)
}

// SetInstanceActive sets the instance to active.
func SetInstanceActive(client client.Client, activeStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request, object runtime.Object) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace},
		sts); err != nil {
		return err
	}
	active := false
	if sts.Status.ReadyReplicas == *sts.Spec.Replicas {
		active = true
	}

	*activeStatus = active
	if err := client.Status().Update(context.TODO(), object); err != nil {
		return err
	}
	return nil
}

func getPodsHostname(c client.Client, pod *corev1.Pod) (string, error) {
	if !pod.Spec.HostNetwork {
		return pod.Spec.Hostname, nil
	}
	n := corev1.Node{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: pod.Spec.NodeName}, &n); err != nil {
		return "", err
	}

	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeHostName {
			// TODO: until moved to latest operator framework FQDN for pod is not available
			// so, artificially use FQDN based on host domain
			// TODO: commonize things between pods
			dnsDomain, err := ClusterDNSDomain(c)
			if err != nil || dnsDomain == "" || strings.HasSuffix(a.Address, dnsDomain) {
				return a.Address, nil
			}
			return a.Address + "." + dnsDomain, nil
		}
	}

	return "", errors.New("couldn't get pods hostname")
}

func updateAnnotations(pod *corev1.Pod, client client.Client) error {
	annotationMap := pod.GetAnnotations()
	if annotationMap == nil {
		annotationMap = make(map[string]string)
	}
	hostname, err := getPodsHostname(client, pod)
	if err != nil {
		return err
	}
	annotationMap["hostname"] = hostname
	pod.SetAnnotations(annotationMap)
	if err = client.Update(context.TODO(), pod); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func PodIPListAndIPMapFromInstance(instanceType string,
	commonConfiguration *PodConfiguration,
	request reconcile.Request,
	clnt client.Client) ([]corev1.Pod, map[string]string, error) {

	labelSelector := labels.SelectorFromSet(map[string]string{"contrail_manager": instanceType,
		instanceType: request.Name})
	listOps := &client.ListOptions{Namespace: request.Namespace, LabelSelector: labelSelector}
	allPods := &corev1.PodList{}
	err := clnt.List(context.TODO(), allPods, listOps)
	if err != nil || len(allPods.Items) == 0 {
		return nil, nil, err
	}

	var podNameIPMap = make(map[string]string)
	var podList = []corev1.Pod{}
	for idx := range allPods.Items {
		pod := &allPods.Items[idx]
		if pod.Status.PodIP == "" || (pod.Status.Phase != "Running" && pod.Status.Phase != "Pending") {
			continue
		}
		if err = updateAnnotations(pod, clnt); err != nil {
			return nil, nil, err
		}
		podNameIPMap[pod.Name] = pod.Status.PodIP
		podList = append(podList, *pod)
	}
	return podList, podNameIPMap, nil
}

// NewCassandraClusterConfiguration gets a struct containing various representations of Cassandra nodes string.
func NewCassandraClusterConfiguration(name string, namespace string, client client.Client) (CassandraClusterConfiguration, error) {
	instance := &Cassandra{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return CassandraClusterConfiguration{}, err
	}
	nodes := []string{}
	if instance.Status.Nodes != nil {
		for _, ip := range instance.Status.Nodes {
			nodes = append(nodes, ip)
		}
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	}
	config := instance.ConfigurationParameters()
	endpoint := instance.Status.ClusterIP + ":" + strconv.Itoa(*config.Port)
	clusterConfig := CassandraClusterConfiguration{
		Port:         *config.Port,
		CQLPort:      *config.CqlPort,
		JMXPort:      *config.JmxLocalPort,
		ServerIPList: nodes,
		Endpoint:     endpoint,
	}
	return clusterConfig, nil
}

// NewControlClusterConfiguration gets a struct containing various representations of Control nodes string.
func NewControlClusterConfiguration(name string, namespace string, myclient client.Client) (ControlClusterConfiguration, error) {
	instance := &Control{}
	err := myclient.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return ControlClusterConfiguration{}, err
	}
	nodes := []string{}
	if instance.Status.Nodes != nil {
		for _, ip := range instance.Status.Nodes {
			nodes = append(nodes, ip)
		}
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	}
	config := instance.ConfigurationParameters()
	clusterConfig := ControlClusterConfiguration{
		XMPPPort:            *config.XMPPPort,
		BGPPort:             *config.BGPPort,
		DNSPort:             *config.DNSPort,
		DNSIntrospectPort:   *config.DNSIntrospectPort,
		ControlServerIPList: nodes,
	}

	return clusterConfig, nil
}

// NewZookeeperClusterConfiguration gets a struct containing various representations of Zookeeper nodes string.
func NewZookeeperClusterConfiguration(name, namespace string, client client.Client) (ZookeeperClusterConfiguration, error) {
	instance := &Zookeeper{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return ZookeeperClusterConfiguration{}, err
	}
	nodes := []string{}
	if instance.Status.Nodes != nil {
		for _, ip := range instance.Status.Nodes {
			nodes = append(nodes, ip)
		}
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	}
	config := instance.ConfigurationParameters()
	clusterConfig := ZookeeperClusterConfiguration{
		ClientPort:   *config.ClientPort,
		ServerIPList: nodes,
	}
	return clusterConfig, nil
}

// NewRabbitmqClusterConfiguration gets a struct containing various representations of Rabbitmq nodes string.
func NewRabbitmqClusterConfiguration(name, namespace string, client client.Client) (RabbitmqClusterConfiguration, error) {
	instance := &Rabbitmq{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return RabbitmqClusterConfiguration{}, err
	}
	nodes := []string{}
	if instance.Status.Nodes != nil {
		for _, ip := range instance.Status.Nodes {
			nodes = append(nodes, ip)
		}
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	}
	instance.ConfigurationParameters()
	clusterConfig := RabbitmqClusterConfiguration{
		Port:         *instance.Spec.ServiceConfiguration.Port,
		ServerIPList: nodes,
		Secret:       instance.Status.Secret,
	}
	return clusterConfig, nil
}

// NewConfigClusterConfiguration gets a struct containing various representations of Config nodes string.
func NewConfigClusterConfiguration(name, namespace string, client client.Client) (ConfigClusterConfiguration, error) {
	instance := &Config{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return ConfigClusterConfiguration{}, err
	}
	nodes := []string{}
	if instance.Status.Nodes != nil {
		for _, ip := range instance.Status.Nodes {
			nodes = append(nodes, ip)
		}
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	}
	config := instance.ConfigurationParameters()
	clusterConfig := ConfigClusterConfiguration{
		APIServerPort:   *config.APIPort,
		APIServerIPList: nodes,
		// TODO: till not splited
		AnalyticsServerIPList: nodes,
		AnalyticsServerPort:   *config.AnalyticsPort,
		CollectorServerIPList: nodes,
		CollectorPort:         *config.CollectorPort,
	}
	return clusterConfig, nil
}

// ConfigClusterConfiguration  stores all information about service's endpoints
// under the Contrail Config
type ConfigClusterConfiguration struct {
	APIServerPort         int      `json:"apiServerPort,omitempty"`
	APIServerIPList       []string `json:"apiServerIPList,omitempty"`
	AnalyticsServerPort   int      `json:"analyticsServerPort,omitempty"`
	AnalyticsServerIPList []string `json:"analyticsServerIPList,omitempty"`
	CollectorPort         int      `json:"collectorPort,omitempty"`
	CollectorServerIPList []string `json:"collectorServerIPList,omitempty"`
}

// FillWithDefaultValues sets the default port values if they are set to the
// zero value
func (c *ConfigClusterConfiguration) FillWithDefaultValues() {
	if c.APIServerPort == 0 {
		c.APIServerPort = ConfigApiPort
	}
	if c.AnalyticsServerPort == 0 {
		c.AnalyticsServerPort = AnalyticsApiPort
	}
	if c.CollectorPort == 0 {
		c.CollectorPort = CollectorPort
	}
}

// ControlClusterConfiguration stores all information about services' endpoints
// under the Contrail Control
type ControlClusterConfiguration struct {
	XMPPPort            int      `json:"xmppPort,omitempty"`
	BGPPort             int      `json:"bgpPort,omitempty"`
	DNSPort             int      `json:"dnsPort,omitempty"`
	DNSIntrospectPort   int      `json:"dnsIntrospectPort,omitempty"`
	ControlServerIPList []string `json:"controlServerIPList,omitempty"`
}

// FillWithDefaultValues sets the default port values if they are set to the
// zero value
func (c *ControlClusterConfiguration) FillWithDefaultValues() {
	if c.XMPPPort == 0 {
		c.XMPPPort = XmppServerPort
	}
	if c.BGPPort == 0 {
		c.BGPPort = BgpPort
	}
	if c.DNSPort == 0 {
		c.DNSPort = DnsServerPort
	}
	if c.DNSIntrospectPort == 0 {
		c.DNSIntrospectPort = DnsIntrospectPort
	}
}

// ZookeeperClusterConfiguration stores all information about Zookeeper's endpoints.
type ZookeeperClusterConfiguration struct {
	ClientPort   int      `json:"clientPort,omitempty"`
	ServerPort   int      `json:"serverPort,omitempty"`
	ElectionPort int      `json:"electionPort,omitempty"`
	ServerIPList []string `json:"serverIPList,omitempty"`
}

// FillWithDefaultValues fills Zookeeper config with default values
func (c *ZookeeperClusterConfiguration) FillWithDefaultValues() {
	if c.ClientPort == 0 {
		c.ClientPort = ZookeeperPort
	}
	if c.ElectionPort == 0 {
		c.ElectionPort = ZookeeperElectionPort
	}
	if c.ServerPort == 0 {
		c.ServerPort = ZookeeperServerPort
	}
}

// RabbitmqClusterConfiguration stores all information about Rabbitmq's endpoints.
type RabbitmqClusterConfiguration struct {
	Port         int      `json:"port,omitempty"`
	ServerIPList []string `json:"serverIPList,omitempty"`
	Secret       string   `json:"secret,omitempty"`
}

// FillWithDefaultValues fills Rabbitmq config with default values
func (c *RabbitmqClusterConfiguration) FillWithDefaultValues() {
	if c.Port == 0 {
		c.Port = RabbitmqNodePort
	}
}

// CassandraClusterConfiguration stores all information about Cassandra's endpoints.
type CassandraClusterConfiguration struct {
	Port         int      `json:"port,omitempty"`
	CQLPort      int      `json:"cqlPort,omitempty"`
	JMXPort      int      `json:"jmxPort,omitempty"`
	ServerIPList []string `json:"serverIPList,omitempty"`
	Endpoint     string   `json:"endpoint,omitempty"`
}

// FillWithDefaultValues fills Cassandra config with default values
func (c *CassandraClusterConfiguration) FillWithDefaultValues() {
	if c.CQLPort == 0 {
		c.CQLPort = CassandraCqlPort
	}
	if c.JMXPort == 0 {
		c.JMXPort = CassandraJmxLocalPort
	}
	if c.Port == 0 {
		c.Port = CassandraPort
	}
}

// ProvisionerEnvData returns provisioner env data
func ProvisionerEnvData(configAPINodes string) string {
	var bufEnv bytes.Buffer
	err := templates.ProvisionerConfig.Execute(&bufEnv, struct {
		ConfigAPINodes   string
		SignerCAFilepath string
		Retries          string
		Delay            string
	}{
		ConfigAPINodes:   configAPINodes,
		SignerCAFilepath: certificates.SignerCAFilepath,
	})
	if err != nil {
		panic(err)
	}
	return bufEnv.String()
}

// UpdateProvisionerRunner adds provisioner runner data
func UpdateProvisionerRunner(configMapName string, configMap *corev1.ConfigMap) {
	var bufRun bytes.Buffer
	err := templates.ProvisionerRunner.Execute(&bufRun, struct {
		ConfigName string
	}{
		ConfigName: configMapName + ".env",
	})
	if err != nil {
		panic(err)
	}
	configMap.Data[configMapName+".sh"] = bufRun.String()
}

// UpdateProvisionerConfigMapData update provisioner data in config map
func UpdateProvisionerConfigMapData(configMapName string, configAPINodes string, configMap *corev1.ConfigMap) {
	UpdateProvisionerRunner(configMapName, configMap)
	configMap.Data[configMapName+".env"] = ProvisionerEnvData(configAPINodes)
}

// GetNodemanagerRunner returns nodemanagaer runner script
func GetNodemanagerRunner() string {
	var bufRun bytes.Buffer
	if err := templates.NodemanagerRunner.Execute(&bufRun, struct{}{}); err != nil {
		panic(err)
	}
	return bufRun.String()
}

// ExecCmdInContainer runs command inside a container
func ExecCmdInContainer(pod *corev1.Pod, containerName string, command []string) (stdout, stderr string, err error) {
	stdout, stderr, err = k8s.ExecToPodThroughAPI(command,
		containerName,
		pod.ObjectMeta.Name,
		pod.ObjectMeta.Namespace,
		nil,
	)
	return
}

// SendSignal signal to main container process with pid 1
func SendSignal(pod *corev1.Pod, containerName, signal string) (stdout, stderr string, err error) {
	return ExecCmdInContainer(
		pod,
		containerName,
		[]string{"/usr/bin/bash", "-c", "kill -" + signal + " $(cat /service.pid.reload) || kill -" + signal + " 1"},
	)
}

// CombinedError provides a combined errors object for comfort logging
type CombinedError struct {
	errors []error
}

func (e *CombinedError) Error() string {
	var res string
	if e != nil {
		res = "CombinedError:\n"
		for _, s := range e.errors {
			res = res + fmt.Sprintf("%s\n", s)
		}
	}
	return res
}

func contains(arr []string, v *corev1.ContainerStatus) bool {
	for _, i := range arr {
		if i == v.Name {
			return true
		}
	}
	return false
}

// ReloadContainers sends sighup to all runnig conainers in a pod
func ReloadContainers(pod *corev1.Pod, containers []string, signal string, clnt client.Client, log logr.Logger) error {
	l := log.WithName(pod.Name).WithName("ReloadContainers(" + signal + ")")
	var errors []error
	for _, cs := range pod.Status.ContainerStatuses {
		if !contains(containers, &cs) {
			continue
		}
		ll := l.WithName(cs.Name)
		if cs.State.Terminated != nil && cs.State.Waiting != nil {
			ll.Info("Skip", "state", cs.State)
			continue
		}
		if stdout, stderr, err := SendSignal(pod, cs.Name, signal); err != nil {
			ll.Error(err, "Failed", "stdout", stdout, "stderr", stderr)
			errors = append(errors, err)
			continue
		}
		ll.Info("Reloaded")
	}
	if len(errors) > 0 {
		return &CombinedError{errors: errors}
	}
	return nil
}

// ReloadServices sends sighup to given runnig conainers in a pod
func ReloadServices(srvList map[*corev1.Pod][]string, clnt client.Client, log logr.Logger) error {
	var errors []error
	for pod, containers := range srvList {
		if err := ReloadContainers(pod, containers, "HUP", clnt, log); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return &CombinedError{errors: errors}
	}
	return nil
}

// RestartServices sends sigterm to given runnig conainers in a pod
func RestartServices(srvList map[*corev1.Pod][]string, clnt client.Client, log logr.Logger) error {
	var errors []error
	for pod, containers := range srvList {
		if err := ReloadContainers(pod, containers, "TERM", clnt, log); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return &CombinedError{errors: errors}
	}
	return nil
}

// EncryptString returns sha
func EncryptString(str string) string {
	h := sha1.New()
	io.WriteString(h, str)
	key := hex.EncodeToString(h.Sum(nil))
	return string(key)
}

// ExecToContainer uninterractively exec to the vrouteragent container.
func ExecToContainer(pod *corev1.Pod, container string, command []string, stdin io.Reader) (string, string, error) {
	stdout, stderr, err := k8s.ExecToPodThroughAPI(command,
		container,
		pod.ObjectMeta.Name,
		pod.ObjectMeta.Namespace,
		stdin,
	)
	return stdout, stderr, err
}

// ContainerFileSha gets sha of file from a container
func ContainerFileSha(pod *corev1.Pod, container string, path string) (string, error) {
	command := []string{"bash", "-c", fmt.Sprintf("[ ! -e %s ] || /usr/bin/sha1sum %s", path, path)}
	stdout, _, err := ExecToContainer(pod, container, command, nil)
	shakey := strings.Split(stdout, " ")[0]
	return shakey, err
}

// ContainerFileChanged checks file content
func ContainerFileChanged(pod *corev1.Pod, container string, path string, content string) (bool, error) {
	shakey1, err := ContainerFileSha(pod, container, path)
	if err != nil {
		return false, err
	}
	shakey2 := EncryptString(content)
	return shakey1 == shakey2, nil
}

// AddCommonVolumes append common volumes and mounts
func AddCommonVolumes(podSpec *corev1.PodSpec) {
	commonVolumes := []corev1.Volume{
		{
			Name: "etc-hosts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/hosts",
				},
			},
		},
		{
			Name: "etc-resolv",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/resolv.conf",
				},
			},
		},
		{
			Name: "etc-timezone",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/timezone",
				},
			},
		},
		{
			Name: "etc-localtime",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/localtime",
				},
			},
		},
		// each sts / ds needs to provide such volume with own specific path
		// as they use own entrypoint instead of contrail-entrypoint.sh from containers
		// {
		// 	Name: "contrail-logs",
		// 	VolumeSource: core.VolumeSource{
		// 		HostPath: &core.HostPathVolumeSource{
		// 			Path: "/var/log/contrail",
		// 		},
		// 	},
		// },
		{
			Name: "var-crashes",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/crashes",
				},
			},
		},
	}
	commonMounts := []corev1.VolumeMount{
		{
			Name:      "etc-hosts",
			MountPath: "/etc/hosts",
			ReadOnly:  true,
		},
		{
			Name:      "etc-resolv",
			MountPath: "/etc/resolv.conf",
			ReadOnly:  true,
		},
		{
			Name:      "etc-timezone",
			MountPath: "/etc/timezone",
			ReadOnly:  true,
		},
		{
			Name:      "etc-localtime",
			MountPath: "/etc/localtime",
			ReadOnly:  true,
		},
		{
			Name:      "var-crashes",
			MountPath: "/var/crashes",
		},
	}

	podSpec.Volumes = append(podSpec.Volumes, commonVolumes...)
	for _, v := range podSpec.Volumes {
		if v.Name == "contrail-logs" {
			commonMounts = append(commonMounts,
				corev1.VolumeMount{
					Name:      "contrail-logs",
					MountPath: "/var/log/contrail",
				})
		}
	}

	for idx := range podSpec.Containers {
		c := &podSpec.Containers[idx]
		c.VolumeMounts = append(c.VolumeMounts, commonMounts...)
	}
	for idx := range podSpec.InitContainers {
		c := &podSpec.InitContainers[idx]
		c.VolumeMounts = append(c.VolumeMounts, commonMounts...)
	}

	AddNodemanagerVolumes(podSpec)
}

// AddNodemanagerVolumes append common volumes and mounts
// - /var/run:/var/run:z
// - /run/runc:/run/runc:z
// - /sys/fs/cgroup:/sys/fs/cgroup:ro
// - /sys/fs/selinux:/sys/fs/selinux
// - /var/lib/containers:/var/lib/containers:shared
func AddNodemanagerVolumes(podSpec *corev1.PodSpec) {
	nodemgrVolumes := []corev1.Volume{
		{
			Name: "var-run",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run",
				},
			},
		},
		{
			Name: "run-runc",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/run/runc",
				},
			},
		},
		{
			Name: "sys-fs-cgroups",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys/fs/cgroup",
				},
			},
		},
		{
			Name: "sys-fs-selinux",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys/fs/selinux",
				},
			},
		},
		{
			Name: "var-lib-containers",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/lib/containers",
				},
			},
		},
	}

	var sharedMode corev1.MountPropagationMode = "Bidirectional"
	nodemgrMounts := []corev1.VolumeMount{
		{
			Name:      "var-run",
			MountPath: "/var/run",
		},
		{
			Name:      "run-runc",
			MountPath: "/run/runc",
		},
		{
			Name:      "sys-fs-cgroups",
			MountPath: "/sys/fs/cgroup",
			ReadOnly:  true,
		},
		{
			Name:      "sys-fs-selinux",
			MountPath: "/sys/fs/selinux",
		},
		{
			Name:             "var-lib-containers",
			MountPath:        "/var/lib/containers",
			MountPropagation: &sharedMode,
		},
	}

	hasNodemgr := false
	for idx := range podSpec.Containers {
		if strings.HasPrefix(podSpec.Containers[idx].Name, "nodemanager") {
			hasNodemgr = true
			c := &podSpec.Containers[idx]
			c.VolumeMounts = append(c.VolumeMounts, nodemgrMounts...)
		}
	}
	if hasNodemgr {
		podSpec.Volumes = append(podSpec.Volumes, nodemgrVolumes...)
	}
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func CommonStartupScript(command string, configs map[string]string) string {
	var buf bytes.Buffer
	err := configtemplates.CommonRunConfig.Execute(&buf, struct {
		Command        string
		Configs        map[string]string
		ConfigMapMount string
		DstConfigPath  string
		CAFilePath     string
	}{
		Command:        command,
		Configs:        configs,
		ConfigMapMount: "/etc/contrailconfigmaps",
		DstConfigPath:  "/etc/contrail",
		CAFilePath:     certificates.SignerCAFilepath,
	})
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func addGroup(ng int64, a []int64) []int64 {
	for _, g := range a {
		if g == ng {
			return a
		}
	}
	return append(a, ng)
}

// DefaultSecurityContext sets security context if not set yet
// (it is to be set explicetely as on openshift default is restricted
// after bootstrap completed)
func DefaultSecurityContext(podSpec *corev1.PodSpec) {
	if podSpec.SecurityContext == nil {
		podSpec.SecurityContext = &corev1.PodSecurityContext{}
	}
	var rootid int64 = 0
	var uid int64 = 1999
	if podSpec.SecurityContext.FSGroup == nil {
		podSpec.SecurityContext.FSGroup = &uid
	}
	podSpec.SecurityContext.SupplementalGroups = addGroup(uid, podSpec.SecurityContext.SupplementalGroups)
	falseVal := false
	for idx := range podSpec.Containers {
		c := &podSpec.Containers[idx]
		if c.SecurityContext == nil {
			c.SecurityContext = &corev1.SecurityContext{}
		}
		if c.SecurityContext.Privileged == nil {
			c.SecurityContext.Privileged = &falseVal
		}
		if c.SecurityContext.RunAsUser == nil {
			// for now all containers expect to be run under root, they do switch user
			// by themselves
			c.SecurityContext.RunAsUser = &rootid
		}
		if c.SecurityContext.RunAsGroup == nil {
			c.SecurityContext.RunAsGroup = &rootid
		}
	}
}
