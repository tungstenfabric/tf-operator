package v1alpha1

import (
	"bytes"
	"context"
	"reflect"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QueryEngine is the Schema for the analyticsdb query engine.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=queryengine,scope=Namespaced
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Active",type=boolean,JSONPath=`.status.active`
type QueryEngine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueryEngineSpec   `json:"spec,omitempty"`
	Status QueryEngineStatus `json:"status,omitempty"`
}

// QueryEngineSpec is the Spec for the AnalyticsDB query engine.
// +k8s:openapi-gen=true
type QueryEngineSpec struct {
	CommonConfiguration  PodConfiguration         `json:"commonConfiguration,omitempty"`
	ServiceConfiguration QueryEngineConfiguration `json:"serviceConfiguration"`
}

// QueryEngineConfiguration is the Spec for the AnalyticsDB query engine.
// +k8s:openapi-gen=true
type QueryEngineConfiguration struct {
	AnalyticsdbPort           *int         `json:"analyticsdbPort,omitempty"`
	AnalyticsdbIntrospectPort *int         `json:"analyticsdbIntrospectPort,omitempty"`
	Containers                []*Container `json:"containers,omitempty"`
	LogLevel                  string       `json:"logLevel,omitempty"`
}

// QueryEngineStatus status of QueryEngine
// +k8s:openapi-gen=true
type QueryEngineStatus struct {
	CommonStatus `json:",inline"`
}

// QueryEngineList contains a list of QueryEngine.
// +k8s:openapi-gen=true
type QueryEngineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueryEngine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QueryEngine{}, &QueryEngineList{})
}

// InstanceConfiguration configures and updates configmaps
func (c *QueryEngine) InstanceConfiguration(podList []corev1.Pod, client client.Client,
) (data map[string]string, err error) {
	data, err = make(map[string]string), nil

	analyticsCassandraInstance, err := GetAnalyticsCassandraInstance(client)
	if err != nil {
		return
	}

	cassandraNodesInformation, err := NewCassandraClusterConfiguration(
		analyticsCassandraInstance, c.Namespace, client)
	if err != nil {
		return
	}

	redisNodesInformation, err := NewRedisClusterConfiguration(RedisInstance,
		c.Namespace, client)
	if err != nil {
		return
	}

	analyticsNodesInformation, err := NewAnalyticsClusterConfiguration(AnalyticsInstance, c.Namespace, client)
	if err != nil {
		return
	}
	configNodesInformation, err := NewConfigClusterConfiguration(ConfigInstance, c.Namespace, client)
	if err != nil {
		return
	}

	queryengineConfig := c.ConfigurationParameters()
	var podIPList []string
	for _, pod := range podList {
		podIPList = append(podIPList, pod.Status.PodIP)
	}
	sort.SliceStable(podList, func(i, j int) bool { return podList[i].Status.PodIP < podList[j].Status.PodIP })
	sort.SliceStable(podIPList, func(i, j int) bool { return podIPList[i] < podIPList[j] })

	configApiList := make([]string, len(configNodesInformation.APIServerIPList))
	copy(configApiList, configNodesInformation.APIServerIPList)
	sort.Strings(configApiList)
	configApiIPCommaSeparated := configtemplates.JoinListWithSeparator(configApiList, ",")

	cassandraCQLEndpointList := configtemplates.EndpointList(cassandraNodesInformation.ServerIPList, cassandraNodesInformation.CQLPort)
	cassandraCQLEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(cassandraCQLEndpointList, " ")
	collectorEndpointList := configtemplates.EndpointList(analyticsNodesInformation.CollectorServerIPList, analyticsNodesInformation.CollectorPort)
	collectorEndpointListSpaceSeparated := configtemplates.JoinListWithSeparator(collectorEndpointList, " ")

	redisEndpointList := configtemplates.EndpointList(redisNodesInformation.ServerIPList, redisNodesInformation.ServerPort)
	redisEndpointListSpaceSpearated := configtemplates.JoinListWithSeparator(redisEndpointList, " ")

	for _, pod := range podList {
		hostname := pod.Annotations["hostname"]
		podIP := pod.Status.PodIP
		instrospectListenAddress := c.Spec.CommonConfiguration.IntrospectionListenAddress(podIP)

		var queryEngineBuffer bytes.Buffer
		err = configtemplates.QueryEngineConfig.Execute(&queryEngineBuffer, struct {
			Hostname                 string
			PodIP                    string
			ListenAddress            string
			InstrospectListenAddress string
			CassandraServerList      string
			CollectorServerList      string
			RedisServerList          string
			CAFilePath               string
			AnalyticsDataTTL         string
			LogLevel                 string
		}{
			Hostname:                 hostname,
			PodIP:                    podIP,
			ListenAddress:            podIP,
			InstrospectListenAddress: instrospectListenAddress,
			CassandraServerList:      cassandraCQLEndpointListSpaceSeparated,
			CollectorServerList:      collectorEndpointListSpaceSeparated,
			RedisServerList:          redisEndpointListSpaceSpearated,
			CAFilePath:               certificates.SignerCAFilepath,
			AnalyticsDataTTL:         strconv.Itoa(analyticsNodesInformation.AnalyticsDataTTL),
			LogLevel:                 queryengineConfig.LogLevel,
		})
		if err != nil {
			panic(err)
		}
		data["queryengine."+podIP] = queryEngineBuffer.String()

		// TODO: commonize for all services
		var vncApiBuffer bytes.Buffer
		err = configtemplates.ConfigAPIVNC.Execute(&vncApiBuffer, struct {
			APIServerList          string
			APIServerPort          string
			CAFilePath             string
			AuthMode               AuthenticationMode
			KeystoneAuthParameters KeystoneAuthParameters
			PodIP                  string
		}{
			APIServerList:          configApiIPCommaSeparated,
			APIServerPort:          strconv.Itoa(configNodesInformation.APIServerPort),
			CAFilePath:             certificates.SignerCAFilepath,
			AuthMode:               c.Spec.CommonConfiguration.AuthParameters.AuthMode,
			KeystoneAuthParameters: c.Spec.CommonConfiguration.AuthParameters.KeystoneAuthParameters,
			PodIP:                  podIP,
		})
		if err != nil {
			panic(err)
		}
		data["vnc_api_lib.ini."+podIP] = vncApiBuffer.String()
	}

	return
}

// CreateConfigMap makes default empty ConfigMap
func (c *QueryEngine) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"queryengine",
		c)
}

// CreateSecret creates a secret.
func (c *QueryEngine) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"queryengine",
		c)
}

// PrepareSTS prepares the intented statefulset for the queryengine object
func (c *QueryEngine) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "queryengine", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the queryengine statefulset
func (c *QueryEngine) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *QueryEngine) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// SetInstanceActive sets the QueryEngine instance to active
func (c *QueryEngine) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	if err := client.Get(context.TODO(), types.NamespacedName{Name: sts.Name, Namespace: request.Namespace}, sts); err != nil {
		return err
	}
	*activeStatus = sts.Status.ReadyReplicas >= *sts.Spec.Replicas/2+1
	*degradedStatus = sts.Status.ReadyReplicas < *sts.Spec.Replicas
	if err := client.Status().Update(context.TODO(), c); err != nil {
		return err
	}
	return nil
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *QueryEngine) PodIPListAndIPMapFromInstance(request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance("queryengine", request, reconcileClient, "")
}

//PodsCertSubjects gets list of QueryEngine pods certificate subjets which can be passed to the certificate API
func (c *QueryEngine) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, altIPs)
}

// ManageNodeStatus updates nodes in status
func (c *QueryEngine) ManageNodeStatus(podNameIPMap map[string]string,
	client client.Client) (updated bool, err error) {
	updated = false
	err = nil

	if reflect.DeepEqual(c.Status.Nodes, podNameIPMap) {
		return
	}

	c.Status.Nodes = podNameIPMap
	if err = client.Status().Update(context.TODO(), c); err != nil {
		return
	}

	updated = true
	return
}

// IsActive returns true if instance is active
func (c *QueryEngine) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// ConfigurationParameters create queryengine struct
func (c *QueryEngine) ConfigurationParameters() QueryEngineConfiguration {
	queryengineConfiguration := QueryEngineConfiguration{}
	var analyticsdbPort int
	var logLevel string

	if c.Spec.ServiceConfiguration.AnalyticsdbPort != nil {
		analyticsdbPort = *c.Spec.ServiceConfiguration.AnalyticsdbPort
	} else {
		analyticsdbPort = AnalyticsdbPort
	}
	queryengineConfiguration.AnalyticsdbPort = &analyticsdbPort

	if c.Spec.ServiceConfiguration.LogLevel != "" {
		logLevel = c.Spec.ServiceConfiguration.LogLevel
	} else {
		logLevel = LogLevel
	}
	queryengineConfiguration.LogLevel = logLevel
	return queryengineConfiguration

}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *QueryEngine) CommonStartupScript(command string, configs map[string]string) string {
	return CommonStartupScript(command, configs)
}
