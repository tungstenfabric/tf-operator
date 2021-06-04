package v1alpha1

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configtemplates "github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1/templates"
	"github.com/tungstenfabric/tf-operator/pkg/certificates"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ZookeeperSpec is the Spec for the zookeepers API.
// +k8s:openapi-gen=true
type ZookeeperSpec struct {
	CommonConfiguration  PodConfiguration       `json:"commonConfiguration,omitempty"`
	ServiceConfiguration ZookeeperConfiguration `json:"serviceConfiguration"`
}

// ZookeeperConfiguration is the Spec for the zookeepers API.
// +k8s:openapi-gen=true
type ZookeeperConfiguration struct {
	Containers        []*Container `json:"containers,omitempty"`
	ClientPort        *int         `json:"clientPort,omitempty"`
	ElectionPort      *int         `json:"electionPort,omitempty"`
	ServerPort        *int         `json:"serverPort,omitempty"`
	AdminEnableServer *bool        `json:"adminEnabled,omitempty"`
	AdminPort         *int         `json:"adminPort,omitempty"`
}

// ZookeeperStatus defines the status of the zookeeper object.
// +k8s:openapi-gen=true
type ZookeeperStatus struct {
	CommonStatus `json:",inline"`
	Ports        ZookeeperStatusPorts `json:"ports,omitempty"`
}

// ZookeeperStatusPorts defines the status of the ports of the zookeeper object.
// +k8s:openapi-gen=true
type ZookeeperStatusPorts struct {
	ClientPort string `json:"clientPort,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Zookeeper is the Schema for the zookeepers API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Zookeeper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZookeeperSpec   `json:"spec,omitempty"`
	Status ZookeeperStatus `json:"status,omitempty"`
}

// ZookeeperList contains a list of Zookeeper.
// +k8s:openapi-gen=true
type ZookeeperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Zookeeper `json:"items"`
}

var zookeeperLog = logf.Log.WithName("controller_zookeeper")

func init() {
	SchemeBuilder.Register(&Zookeeper{}, &ZookeeperList{})
}

// InstanceConfiguration creates the zookeeper instance configuration.
func (c *Zookeeper) InstanceConfiguration(request reconcile.Request, confCMName string,
	podList []corev1.Pod,
	client client.Client) error {
	zookeeperConfig := c.ConfigurationParameters()
	pods := make([]corev1.Pod, len(podList))
	copy(pods, podList)
	sort.SliceStable(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })

	confCMData, err := configtemplates.DynamicZookeeperConfig(pods, strconv.Itoa(*zookeeperConfig.ElectionPort), strconv.Itoa(*zookeeperConfig.ServerPort), strconv.Itoa(*zookeeperConfig.ClientPort))
	if err != nil {
		return err
	}
	var zookeeperLogConfig bytes.Buffer
	err = configtemplates.ZookeeperLogConfig.Execute(&zookeeperLogConfig, struct {
		LogLevel string
	}{
		LogLevel: c.Spec.CommonConfiguration.LogLevel,
	})
	if err != nil {
		panic(err)
	}
	confCMData["log4j.properties"] = zookeeperLogConfig.String()
	confCMData["configuration.xsl"] = configtemplates.ZookeeperXslConfig
	var zookeeperConfigBuffer bytes.Buffer
	err = configtemplates.ZookeeperStaticConfig.Execute(&zookeeperConfigBuffer, struct {
		AdminEnableServer string
		AdminServerPort   string
	}{
		AdminEnableServer: strconv.FormatBool(*zookeeperConfig.AdminEnableServer),
		AdminServerPort:   strconv.Itoa(*zookeeperConfig.AdminPort),
	})
	if err != nil {
		panic(err)
	}
	zookeeperStaticConfigString := zookeeperConfigBuffer.String()
	confCMData["zoo.cfg"] = zookeeperStaticConfigString

	zookeeperConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      confCMName,
			Namespace: request.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(context.TODO(), client, zookeeperConfigMap, func() error {
		zookeeperConfigMap.Data = confCMData
		return nil
	})
	return err

}

// CreateConfigMap creates a configmap for zookeeper service.
func (c *Zookeeper) CreateConfigMap(configMapName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.ConfigMap, error) {
	return CreateConfigMap(configMapName,
		client,
		scheme,
		request,
		"zookeeper",
		c)
}

// IsActive returns true if instance is active.
func (c *Zookeeper) IsActive(name string, namespace string, client client.Client) bool {
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, c)
	if err != nil || c.Status.Active == nil {
		return false
	}
	return *c.Status.Active
}

// IsUpgrading returns true if instance is upgrading.
func (c *Zookeeper) IsUpgrading(name string, namespace string, client client.Client) bool {
	instance := &Zookeeper{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, instance)
	if err != nil {
		return false
	}
	sts := &appsv1.StatefulSet{}
	err = client.Get(context.TODO(), types.NamespacedName{Name: name + "-" + "zookeeper" + "-statefulset", Namespace: namespace}, sts)
	if err != nil {
		return false
	}
	if sts.Status.CurrentRevision != sts.Status.UpdateRevision {
		return true
	}
	return false
}

// CreateSecret creates a secret.
func (c *Zookeeper) CreateSecret(secretName string,
	client client.Client,
	scheme *runtime.Scheme,
	request reconcile.Request) (*corev1.Secret, error) {
	return CreateSecret(secretName,
		client,
		scheme,
		request,
		"zookeeper",
		c)
}

// PrepareSTS prepares the intended deployment for the Zookeeper object.
func (c *Zookeeper) PrepareSTS(sts *appsv1.StatefulSet, commonConfiguration *PodConfiguration, request reconcile.Request, scheme *runtime.Scheme) error {
	return PrepareSTS(sts, commonConfiguration, "zookeeper", request, scheme, c, true)
}

// AddVolumesToIntendedSTS adds volumes to the Zookeeper deployment.
func (c *Zookeeper) AddVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// AddSecretVolumesToIntendedSTS adds volumes to the Rabbitmq deployment.
func (c *Zookeeper) AddSecretVolumesToIntendedSTS(sts *appsv1.StatefulSet, volumeConfigMapMap map[string]string) {
	AddSecretVolumesToIntendedSTS(sts, volumeConfigMapMap)
}

// PodIPListAndIPMapFromInstance gets a list with POD IPs and a map of POD names and IPs.
func (c *Zookeeper) PodIPListAndIPMapFromInstance(instanceType string, request reconcile.Request, reconcileClient client.Client) ([]corev1.Pod, map[string]string, error) {
	return PodIPListAndIPMapFromInstance(instanceType, request, reconcileClient, "")
}

// SetInstanceActive sets the Zookeeper instance to active.
func (c *Zookeeper) SetInstanceActive(client client.Client, activeStatus *bool, degradedStatus *bool, sts *appsv1.StatefulSet, request reconcile.Request) error {
	return SetInstanceActive(client, activeStatus, degradedStatus, sts, request, c)
}

// ZookeeperPod is a pod with zookeper service. It is an inheritor of corev1.Pod.
type zookeeperPod struct {
	Pod *corev1.Pod
}

// ExecToZookeeperContainer execute command on zookeeper container.
func (zp *zookeeperPod) execToZookeeperContainer(command []string) (stdout, stderr string, err error) {
	stdout, stderr, err = ExecToContainer(zp.Pod, "zookeeper", command, nil)
	return
}

// getPodId returns number of the pod from name, where pod name is `<zookeeper_sts_name>-<number_of_pod>`.
func getPodId(pod *corev1.Pod) (id int, err error) {
	name := pod.ObjectMeta.Name

	re := regexp.MustCompile(`\d+$`)
	id, err = strconv.Atoi(re.FindString(name))
	return
}

// Registrate registrates pod in the cluster.
func (zp *zookeeperPod) registrate(zConfig ZookeeperConfiguration) error {
	ip := zp.Pod.Status.PodIP
	electionPort := *zConfig.ElectionPort
	serverPort := *zConfig.ServerPort
	id, err := getPodId(zp.Pod)
	if err != nil {
		zookeeperLog.Error(err, "Failed to get pod id for pod "+zp.Pod.ObjectMeta.Name)
		return err
	}

	command := fmt.Sprintf("zkCli.sh -server %s reconfig -add \"server.%d=%s:%d:%d;%s:2181\"",
		ip,
		id+1,
		ip,
		electionPort,
		serverPort,
		ip,
	)
	_, _, err = zp.execToZookeeperContainer([]string{"bash", "-c", command})
	return err
}

func (c *Zookeeper) ManageNodeStatus(podIPList []corev1.Pod,
	client client.Client,
) (requequeNeeded bool, err error) {
	requequeNeeded = false
	err = nil

	config := c.ConfigurationParameters()
	clientPort := strconv.Itoa(*config.ClientPort)

	nodes := make(map[string]string)
	for _, pod := range podIPList {
		name := pod.ObjectMeta.Name
		ip := pod.Status.PodIP
		zpod := zookeeperPod{&pod}

		if _ip, _ok := c.Status.Nodes[name]; _ok && _ip == ip {
			nodes[name] = ip
			continue
		}

		if _err := zpod.registrate(config); _err == nil {
			// Pod successfully registered
			nodes[name] = ip
		} else {
			// Pod not registered
			requequeNeeded = true
		}
	}

	if c.Status.Ports.ClientPort != clientPort || !reflect.DeepEqual(c.Status.Nodes, nodes) {
		c.Status.Ports.ClientPort = clientPort
		c.Status.Nodes = nodes

		if err = client.Status().Update(context.TODO(), c); err != nil {
			return
		}
		requequeNeeded = true
	}
	return
}

// ConfigurationParameters sets the default for the configuration parameters.
func (c *Zookeeper) ConfigurationParameters() ZookeeperConfiguration {
	zookeeperConfiguration := ZookeeperConfiguration{}
	var clientPort int
	var electionPort int
	var serverPort int
	var adminEnableServer bool
	var adminPort int

	if c.Spec.ServiceConfiguration.ClientPort != nil {
		clientPort = *c.Spec.ServiceConfiguration.ClientPort
	} else {
		clientPort = ZookeeperPort
	}
	if c.Spec.ServiceConfiguration.ElectionPort != nil {
		electionPort = *c.Spec.ServiceConfiguration.ElectionPort
	} else {
		electionPort = ZookeeperElectionPort
	}
	if c.Spec.ServiceConfiguration.ServerPort != nil {
		serverPort = *c.Spec.ServiceConfiguration.ServerPort
	} else {
		serverPort = ZookeeperServerPort
	}
	if c.Spec.ServiceConfiguration.AdminEnableServer != nil {
		adminEnableServer = *c.Spec.ServiceConfiguration.AdminEnableServer
	} else {
		adminEnableServer = ZookeeperAdminEnableServer
	}
	if c.Spec.ServiceConfiguration.AdminPort != nil {
		adminPort = *c.Spec.ServiceConfiguration.AdminPort
	} else {
		adminPort = ZookeeperAdminPort
	}
	zookeeperConfiguration.ClientPort = &clientPort
	zookeeperConfiguration.ElectionPort = &electionPort
	zookeeperConfiguration.ServerPort = &serverPort
	zookeeperConfiguration.AdminEnableServer = &adminEnableServer
	zookeeperConfiguration.AdminPort = &adminPort

	return zookeeperConfiguration
}

// PodsCertSubjects gets list of Zookeeper pods certificate subjets which can be passed to the certificate API
func (c *Zookeeper) PodsCertSubjects(domain string, podList []corev1.Pod) []certificates.CertificateSubject {
	var altIPs PodAlternativeIPs
	return PodsCertSubjects(domain, podList, c.Spec.CommonConfiguration.HostNetwork, altIPs)
}

// CommonStartupScript prepare common run service script
//  command - is a final command to run
//  configs - config files to be waited for and to be linked from configmap mount
//   to a destination config folder (if destination is empty no link be done, only wait), e.g.
//   { "api.${POD_IP}": "", "vnc_api.ini.${POD_IP}": "vnc_api.ini"}
func (c *Zookeeper) CommonStartupScript(command string, configs map[string]string) string {
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
		ConfigMapMount: "/zookeeper-conf",
		DstConfigPath:  "/var/lib/zookeeper",
		CAFilePath:     certificates.SignerCAFilepath,
	})
	if err != nil {
		panic(err)
	}
	return buf.String()
}
