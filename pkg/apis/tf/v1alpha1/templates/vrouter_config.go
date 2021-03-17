package templates

import (
	"strings"
	"text/template"

	htemplate "html/template"

	"github.com/Masterminds/sprig"
)

var vrouterTemplateFuncs = map[string]interface{}{
	"bool2string": bool2string,
	"isEnabled":   isEnabled,
}

func bool2string(i *bool) string {
	if i == nil {
		return ""
	}
	if *i {
		return "True"
	}
	return "False"
}

func isEnabled(param string) bool {
	lowerParam := strings.ToLower(param)
	if lowerParam == "true" || lowerParam == "yes" || lowerParam == "enabled" {
		return true
	}
	return false
}

// VRouterAgentParams using to pass Manifest params into vrouter-agent container for prepare config file
var VRouterAgentParams = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Funcs(vrouterTemplateFuncs).Parse(`#!/bin/bash
set -o allexport

# log dir be mount onto host /var/log/contrail/vrouter-agent
CONTAINER_LOG_DIR=/var/log/contrail

# TODO: not clear if it is needed in operator
VROUTER_ENCRYPTION=false

#TODO uncomment parameters to export after debug
CONTROL_NODES="{{ .ClusterParams.ControlNodes }}"
CONFIG_NODES="{{ .ClusterParams.ConfigNodes }}"
ANALYTICS_NODES="{{ .ClusterParams.AnalyticsNodes }}"

# Cloud Orchestration
CLOUD_ORCHESTRATOR="{{ .ServiceConfig.CloudOrchestrator }}"
HYPERVISOR_TYPE="{{ .ServiceConfig.HypervisorType }}"

# Collector
#STATS_COLLECTOR_DESTINATION_PATH="{{ .ServiceConfig.StatsCollectorDestinationPath }}"
#COLLECTOR_PORT="{{ .ServiceConfig.CollectorPort }}"

# Config
#CONFIG_API_PORT="{{ .ServiceConfig.ConfigApiPort }}"
CONFIG_API_SERVER_CA_CERTFILE="{{ .ServiceConfig.ConfigApiServerCaCertfile }}"
CONFIG_API_SSL_ENABLE="{{ .ServiceConfig.ConfigApiSslEnable }}"

# DNS
#DNS_SERVER_PORT="{{ .ServiceConfig.DnsServerPort }}"

# Host
#DPDK_UIO_DRIVER="{{ .ServiceConfig.DpdkUioDriver }}"
#SRIOV_PHYSICAL_INTERFACE="{{ .ServiceConfig.SriovPhysicalInterace }}"
#SRIOV_PHYSICAL_NETWORK="{{ .ServiceConfig.SriovPhysicalNetwork }}"
#SRIOV_VF="{{ .ServiceConfig.SriovVf }}"

# Introspect
#INTROSPECT_LISTEN_ALL="{{ .ServiceConfig.IntrospectListenAll }}"
INTROSPECT_SSL_ENABLE="{{ .ServiceConfig.IntrospectSslEnable }}"

# Keystone authentication
#KEYSTONE_AUTH_ADMIN_PORT="{{ .ServiceConfig.KeystoneAuthAdminPort }}"
#KEYSTONE_AUTH_CA_CERTFILE="{{ .ServiceConfig.KeystoneAuthCaCertfile }}"
#KEYSTONE_AUTH_CERTFILE="{{ .ServiceConfig.KeystoneAuthCertfile }}"
#KEYSTONE_AUTH_HOST="{{ .ServiceConfig.KeystoneAuthHost }}"
#KEYSTONE_AUTH_INSECURE="{{ .ServiceConfig.KeystoneAuthInsecure }}"
#KEYSTONE_AUTH_KEYFILE="{{ .ServiceConfig.KeystoneAuthKeyfile }}"
#KEYSTONE_AUTH_PROJECT_DOMAIN_NAME="{{ .ServiceConfig.KeystoneAuthProjectDomainName }}"
#KEYSTONE_AUTH_PROTO="{{ .ServiceConfig.KeystoneAuthProto }}"
#KEYSTONE_AUTH_REGION_NAME="{{ .ServiceConfig.KeystoneAuthRegionName }}"
#KEYSTONE_AUTH_URL_TOKENS="{{ .ServiceConfig.KeystoneAuthUrlTokens }}"
#KEYSTONE_AUTH_URL_VERSION="{{ .ServiceConfig.KeystoneAuthUrlVersion }}"
#KEYSTONE_AUTH_USER_DOMAIN_NAME="{{ .ServiceConfig.KeystoneAuthUserDomainName }}"
#KEYSTONE_AUTH_ADMIN_PASSWORD="{{ .ServiceConfig.KeystoneAuthAdminPassword }}"

# Kubernetes
#K8S_TOKEN="{{ .ServiceConfig.K8sToken }}"
#K8S_TOKEN_FILE="{{ .ServiceConfig.K8sTokenFile }}"
#KUBERNETES_API_PORT="{{ .ServiceConfig.KubernetesApiPort }}"
#KUBERNETES_API_SECURE_PORT="{{ .ServiceConfig.KubernetesApiSecurePort }}"
#KUBERNETES_POD_SUBNETS="{{ .ServiceConfig.KubernetesPodSubnet }}"

# Logging
#LOG_DIR="{{ .ServiceConfig.LogDir }}"
LOG_LEVEL="{{ .ServiceConfig.LogLevel }}"
LOG_LOCAL="{{ .ServiceConfig.LogLocal }}"

# Metadata
#METADATA_PROXY_SECRET="{{ .ServiceConfig.MetadataProxySecret }}"
#METADATA_SSL_CA_CERTFILE="{{ .ServiceConfig.MetadataSslCaCertfile }}"
#METADATA_SSL_CERTFILE="{{ .ServiceConfig.MetadataSslCertfile }}"
#METADATA_SSL_CERT_TYPE="{{ .ServiceConfig.MetadataSslCertType }}"
#METADATA_SSL_ENABLE="{{ .ServiceConfig.MetadataSslEnable }}"
#METADATA_SSL_KEYFILE="{{ .ServiceConfig.MetadataSslKeyfile }}"

# OpenStack
#BARBICAN_TENANT_NAME="{{ .ServiceConfig.BarbicanTenantName }}"
#BARBICAN_PASSWORD="{{ .ServiceConfig.BarbicanPassword }}"
#BARBICAN_USER="{{ .ServiceConfig.BarbicanUser }}"

# Sandesh
SANDESH_CA_CERTFILE="{{ .ServiceConfig.SandeshCaCertfile }}"
SANDESH_CERTFILE="{{ .ServiceConfig.SandeshCertfile }}"
SANDESH_KEYFILE="{{ .ServiceConfig.SandeshKeyfile }}"
SANDESH_SSL_ENABLE="{{ .ServiceConfig.SandeshSslEnable }}"

# Server SSL
SERVER_CA_CERTFILE="{{ .ServiceConfig.ServerCaCertfile }}"
SERVER_CERTFILE="{{ .ServiceConfig.ServerCertfile }}"
SERVER_KEYFILE="{{ .ServiceConfig.ServerKeyfile }}"
SSL_ENABLE="{{ .ServiceConfig.SslEnable }}"
#SSL_INSECURE="{{ .ServiceConfig.SslInsecure }}"

# TSN
#TSN_AGENT_MODE="{{ .ServiceConfig.TsnAgentMode }}"

# vRouter
#AGENT_MODE="{{ .ServiceConfig.AgentMode }}"
#FABRIC_SNAT_HASH_TABLE_SIZE="{{ .ServiceConfig.FabricSnatHashTableSize }}"
#PRIORITY_BANDWIDTH="{{ .ServiceConfig.PriorityBandwidth }}"
#PRIORITY_ID="{{ .ServiceConfig.PriorityId }}"
#PRIORITY_SCHEDULING="{{ .ServiceConfig.PriorityScheduling }}"
#PRIORITY_TAGGING="{{ .ServiceConfig.PriorityTagging }}"
#QOS_DEF_HW_QUEUE="{{ .ServiceConfig.QosDefHwQueue }}"
#QOS_LOGICAL_QUEUES="{{ .ServiceConfig.QosLogicalQueues }}"
#QOS_QUEUE_ID="{{ .ServiceConfig.QosQueueId }}"
#REQUIRED_KERNEL_VROUTER_ENCRYPTION="{{ .ServiceConfig.RequiredKernelVrouterEncryption }}"
#SAMPLE_DESTINATION="{{ .ServiceConfig.SampleDestination }}"
#SLO_DESTINATION="{{ .ServiceConfig.SloDestination }}"
#VROUTER_CRYPT_INTERFACE="{{ .ServiceConfig.VrouterCryptInterface }}"
#VROUTER_DECRYPT_INTERFACE="{{ .ServiceConfig.VrouterDecryptInterface }}"
#VROUTER_DECRYPT_KEY="{{ .ServiceConfig.VrouterDecyptKey }}"
#VROUTER_ENCRYPTION="{{ .ServiceConfig.VrouterEncryption }}"
#VROUTER_GATEWAY="{{ .ServiceConfig.VrouterGateway }}"

# XMPP
#SUBCLUSTER="{{ .ServiceConfig.Subclaster }}"
XMPP_SERVER_CA_CERTFILE="{{ .ServiceConfig.XmppServerCaCertfile }}"
XMPP_SERVER_CERTFILE="{{ .ServiceConfig.XmppServerCertfile }}"
XMPP_SERVER_KEYFILE="{{ .ServiceConfig.XmppServerKeyfile }}"
#XMPP_SERVER_PORT="{{ .ServiceConfig.XmppServerPort }}"
XMPP_SSL_ENABLE="{{ .ServiceConfig.XmppSslEnable }}"

# HugePages
HUGE_PAGES_2MB="{{ default 0 .ServiceConfig.HugePages2M }}"
HUGE_PAGES_1GB="{{ default 0 .ServiceConfig.HugePages1G }}"

set +o allexport
`))

var VRouterAgentConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Funcs(vrouterTemplateFuncs).Parse(`[CONTROL-NODE]
servers={{ .XMPP_SERVERS_LIST }}
{{ if .SUBCLUSTER }}subcluster_name={{ .SUBCLUSTER }}{{ end }}
[DEFAULT]
http_server_ip={{ .INTROSPECT_IP }}
collectors={{ .COLLECTOR_SERVERS }}
log_file={{ .CONTAINER_LOG_DIR }}/contrail-vrouter-agent.log
log_level={{ .LOG_LEVEL }}
log_local={{ .LOG_LOCAL }}
hostname={{ .Hostname }}
agent_name={{ .Hostname }}
xmpp_dns_auth_enable={{ .XMPP_SSL_ENABLE }}
xmpp_auth_enable={{ .XMPP_SSL_ENABLE }}
xmpp_server_cert={{ .XMPP_SERVER_CERTFILE }}
xmpp_server_key={{ .XMPP_SERVER_KEYFILE }}
xmpp_ca_cert={{ .XMPP_SERVER_CA_CERTFILE }}
{{ if eq .AGENT_MODE "dpdk" }}
platform={{ .AGENT_MODE }}
physical_interface_mac={{ .PHYS_INT_MAC }}
physical_interface_address={{ .PCI_ADDRESS }}
physical_uio_driver={{ .DPDK_UIO_DRIVER }}
{{ else }}
physical_interface_mac = {{ .PHYS_INT_MAC }}
{{ end }}
{{ if .TSN_AGENT_MODE }}agent_mode = {{ .TSN_AGENT_MODE }}{{ end }}

tsn_servers = {{ .TSN_NODES | replace "," " " }}

[SANDESH]
introspect_ssl_enable={{ .INTROSPECT_SSL_ENABLE }}
introspect_ssl_insecure=True
sandesh_ssl_enable={{ .SANDESH_SSL_ENABLE }}
sandesh_keyfile={{ .SANDESH_KEYFILE }}
sandesh_certfile={{ .SANDESH_CERTFILE }}
sandesh_ca_cert={{ .SANDESH_CA_CERTFILE }}

[NETWORKS]
control_network_ip={{ .CONTROL_NETWORK_IP }}

[DNS]
servers={{ .DNS_SERVERS_LIST }}

[METADATA]
metadata_proxy_secret={{ .METADATA_PROXY_SECRET }}

{{ if .METADATA_SSL_ENABLE }}
metadata_use_ssl={{ .METADATA_SSL_ENABLE }}
metadata_client_cert={{ .METADATA_SSL_CERTFILE }}
metadata_client_key={{ .METADATA_SSL_KEYFILE }}
metadata_ca_cert={{ .METADATA_SSL_CA_CERTFILE }}
{{ if .METADATA_SSL_CERT_TYPE }}
metadata_ssl_conf+={{ .METADATA_SSL_CERT_TYPE }}
{{ end }}
{{ end }}

[VIRTUAL-HOST-INTERFACE]
name=vhost0
ip={{ .VROUTER_CIDR }}
physical_interface={{ .PHYS_INT }}
compute_node_address={{ .COMPUTE_NODE_ADDRESS }}
{{ if .VROUTER_GATEWAY }}
gateway={{ .VROUTER_GATEWAY }}
{{ end }}

[SERVICE-INSTANCE]
netns_command=/usr/bin/opencontrail-vrouter-netns
docker_command=/usr/bin/opencontrail-vrouter-docker

[HYPERVISOR]
type = {{ .HYPERVISOR_TYPE }}
{{ if and (eq .CLOUD_ORCHESTRATOR "vcenter") (not .TSN_AGENT_MODE ) }}
vmware_physical_interface = {{ .VMWARE_PHYS_INT }}
vmware_mode = vcenter
{{ end }}

[FLOWS]
fabric_snat_hash_table_size = {{ .FABRIC_SNAT_HASH_TABLE_SIZE }}

{{ if and (and .PRIORITY_ID (ne .AGENT_MODE "dpdk" )) (ne .IS_VLAN_ENABLED "true") }}
{{ $priority_id_list := .PRIORITY_ID | splitList ","  }}
{{ $priority_bandwidth_list := .PRIORITY_BANDWIDTH | splitList ","  }}
{{ $priority_scheduling_list := .PRIORITY_SCHEDULING | splitList ","  }}
[QOS-NIANTIC]
{{ range $idx, $prioroty_id := $priority_id_list }}
[PG-{{ $prioroty_id }}]
scheduling={{ index $priority_scheduling_list $idx }}
bandwidth={{ index $priority_bandwidth_list $idx }}
{{ end }}
{{ end }}

{{ if and .QOS_QUEUE_ID (ne .AGENT_MODE "dpdk") }}
{{ $qos_logical_queue := .QOS_LOGICAL_QUEUES | splitList ";" }}
[QOS]
priority_tagging={{ .PRIORITY_TAGGING }}
{{ $qos_queue_id := .QOS_QUEUE_ID | splitList "," }}
{{ range $idx, $queue_id := $qos_queue_id }}
{{ if lt $idx (sub (len $qos_queue_id) 1)}}
[QUEUE-{{ $queue_id }}]
logical_queue={{ if lt $idx (len $qos_logical_queue) }}{{ index $qos_logical_queue $idx }}{{ end }}
{{ end }}
{{ end }}
{{ if ne (len $qos_logical_queue) (len $qos_queue_id) }}
[QUEUE-{{ index $qos_queue_id (sub (len $qos_queue_id) 1)}}]
logical_queue=[]{{ else }}
[QUEUE-{{ index $qos_queue_id (sub (len $qos_queue_id) 1) }}]
logical_queue={{ index $qos_logical_queue (sub (len $qos_logical_queue) 1) }}{{ end }}
{{ if isEnabled .QOS_DEF_HW_QUEUE }}default_hw_queue=true{{ end }}
{{ end }}

{{ if eq .IS_ENCRYPTION_SUPPORTED_FLAG "true"}}
[CRYPT]
crypt_interface={{ .VROUTER_CRYPT_INTERFACE }}
{{ end }}

[SESSION]
slo_destination = {{ .SLO_DESTINATION }}
sample_destination = {{ .SAMPLE_DESTINATION }}
{{ if .STATS_COLLECTOR_DESTINATION_PATH }}
[STATS]
stats_collector={{ .STATS_COLLECTOR_DESTINATION_PATH }}{{ end }}

{{ if and .HUGE_PAGES_1GB (lt 0 (atoi .HUGE_PAGES_1GB)) }}
[RESTART]
huge_page_1G={{ .HUGEPAGES_DIR }}/bridge {{ .HUGEPAGES_DIR }}/flow
{{ else if and .HUGE_PAGES_2MB (lt 0 (atoi .HUGE_PAGES_2MB)) }}
[RESTART]
huge_page_2M={{ .HUGEPAGES_DIR }}/bridge {{ .HUGEPAGES_DIR }}/flow
{{ end }}`))

var VRouterVncApiLibIni = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Funcs(vrouterTemplateFuncs).Parse(`
[global]
WEB_SERVER = {{ .CONFIG_NODES }}
WEB_PORT = {{ default "8082" .CONFIG_API_PORT }}
BASE_URL = /
use_ssl = {{ .CONFIG_API_SSL_ENABLE }}
{{ if isEnabled .CONFIG_API_SSL_ENABLE }}
cafile = {{ .CONFIG_API_SERVER_CA_CERTFILE }}
{{ end }}
{{ if eq .AUTH_MODE "keystone" }}

; Authentication settings (optional)
[auth]
AUTHN_TYPE = keystone
AUTHN_PROTOCOL = {{ .KEYSTONE_AUTH_PROTO }}
AUTHN_SERVER = {{ .KEYSTONE_AUTH_HOST }}
AUTHN_PORT = {{ .KEYSTONE_AUTH_ADMIN_PORT }}
AUTHN_URL = {{ .KEYSTONE_AUTH_URL_TOKENS }}
AUTHN_DOMAIN = {{ .KEYSTONE_AUTH_PROJECT_DOMAIN_NAME }}
;AUTHN_TOKEN_URL = http://127.0.0.1:35357/v2.0/tokens
{{ if eq .KEYSTONE_AUTH_PROTO "https" }}
insecure = {{ lower .KEYSTONE_AUTH_INSECURE }}
certfile = {{ .KEYSTONE_AUTH_CERTFILE }}
keyfile = {{ .KEYSTONE_AUTH_KEYFILE }}
cafile = {{ .KEYSTONE_AUTH_CA_CERTFILE }}
{{ end }}
{{ else }}
[auth]
AUTHN_TYPE = noauth
{{ end }}`))

var VRouterLbaasAuthConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Funcs(vrouterTemplateFuncs).Parse(`[BARBICAN]
admin_tenant_name = {{ .BARBICAN_TENANT_NAME }}
admin_user = {{ .BARBICAN_USER }}
admin_password = {{ .BARBICAN_PASSWORD }}
auth_url = {{ .KEYSTONE_AUTH_PROTO }}://{{ .KEYSTONE_AUTH_HOST }}:{{ .KEYSTONE_AUTH_ADMIN_PORT }}{{ .KEYSTONE_AUTH_URL_VERSION }}
region = {{ .KEYSTONE_AUTH_REGION_NAME }}
user_domain_name = {{ .KEYSTONE_AUTH_USER_DOMAIN_NAME }}
project_domain_name = {{ .KEYSTONE_AUTH_PROJECT_DOMAIN_NAME }}
region_name = {{ .KEYSTONE_AUTH_REGION_NAME }}
insecure = {{ .KEYSTONE_AUTH_INSECURE }}
certfile = {{ .KEYSTONE_AUTH_CERTFILE }}
keyfile = {{ .KEYSTONE_AUTH_KEYFILE }}
cafile = {{ .KEYSTONE_AUTH_CA_CERTFILE }}

[KUBERNETES]
kubernetes_token={{ .K8S_TOKEN }}
kubernetes_api_server={{ default .KUBERNETES_API_SERVER .DEFAULT_LOCAL_IP }}
kubernetes_api_port={{ default "8080" .KUBERNETES_API_PORT }}
kubernetes_api_secure_port={{ default "6443" .KUBERNETES_API_SECURE_PORT }}`))

//VrouterNodemanagerConfig is the template of the Vrouter Nodemanager service configuration
var VrouterNodemanagerConfig = template.Must(template.New("").Parse(`[DEFAULTS]
http_server_ip={{ .INTROSPECT_IP }}
log_file=/var/log/contrail/contrail-vrouter-nodemgr.log
log_level={{ .LOG_LEVEL }}
log_local={{ .LOG_LOCAL }}
hostname={{ .Hostname }}
hostip={{ .CONTROL_NETWORK_IP }}
#db_port={{ .CassandraPort }}
#db_jmx_port={{ .CassandraJmxPort }}
#db_use_ssl=True

[COLLECTOR]
server_list={{ .COLLECTOR_SERVERS }}

[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile={{ .SANDESH_KEYFILE }}
sandesh_certfile={{ .SANDESH_CERTFILE }}
sandesh_ca_cert={{ .SANDESH_CA_CERTFILE }}
`))

var ContrailCNIConfig = template.Must(template.New("").Parse(`{
  "cniVersion": "0.3.1",
  "contrail" : {
      "cluster-name"  : "{{ .KubernetesClusterName }}",
      "meta-plugin"   : "multus",
      "vrouter-ip"    : "127.0.0.1",
      "vrouter-port"  : 9091,
      "config-dir"    : "/var/lib/contrail/ports/vm",
      "poll-timeout"  : 5,
      "poll-retries"  : 15,
      "log-file"      : "/var/log/contrail/cni/opencontrail.log",
      "log-level"     : "4"
  },
  "name": "contrail-k8s-cni",
  "type": "contrail-k8s-cni"
}`))
