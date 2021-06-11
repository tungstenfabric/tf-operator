package templates

import "text/template"

var ConfigAPIServerConfig = template.Must(template.New("").Parse(`encryption:
ca: {{ .CAFilePath }}
cert: /etc/certificates/server-{{ .PodIP }}.crt
key: /etc/certificates/server-key-{{ .PodIP }}.pem
insecure: false
apiServerList:
{{range .APIServerList}}
- {{ . }}
{{ end }}
apiPort: {{ .ListenPort }}
`))

// ConfigAPIConfig is the template of the Config API service configuration.
var ConfigAPIConfig = template.Must(template.New("").Parse(`[DEFAULTS]
listen_ip_addr={{ .ListenAddress }}
listen_port={{ .ListenPort }}
http_server_port={{ .ApiIntrospectPort}}
http_server_ip={{ .InstrospectListenAddress }}
log_file=/var/log/contrail/contrail-api.log
log_level={{ .LogLevel }}
log_local=1
list_optimization_enabled=True
auth={{ .AuthMode }}
aaa_mode={{ .AAAMode }}
cloud_admin_role=admin
global_read_only_role=
config_api_ssl_enable=True
config_api_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
config_api_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
config_api_ssl_ca_cert={{ .CAFilePath }}
cassandra_server_list={{ .CassandraServerList }}
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
zk_server_ip={{ .ZookeeperServerList }}
rabbit_server={{ .RabbitmqServerList }}
rabbit_vhost={{ .RabbitmqVhost }}
rabbit_user={{ .RabbitmqUser }}
rabbit_password={{ .RabbitmqPassword }}
rabbit_use_ssl=True
kombu_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kombu_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kombu_ssl_ca_certs={{ .CAFilePath }}
kombu_ssl_version=tlsv1_2
rabbit_health_check_interval=10
collectors={{ .CollectorServerList }}
enable_latency_stats_log=False
enable_api_stats_log=True

[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))

// ConfigDeviceManagerConfig is the template of the DeviceManager service configuration.
var ConfigDeviceManagerConfig = template.Must(template.New("").Parse(`[DEFAULTS]
host_ip={{ .FabricMgmtIP }}
http_server_ip={{ .InstrospectListenAddress }}
api_server_ip={{ .ApiServerList}}
api_server_port=8082
http_server_port={{ .DeviceManagerIntrospectPort}}
api_server_use_ssl=True
analytics_server_ip={{ .AnalyticsServerList}}
analytics_server_port=8081
push_mode=1
log_file=/var/log/contrail/contrail-device-manager.log
log_level={{ .LogLevel }}
log_local=1
cassandra_server_list={{ .CassandraServerList }}
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
zk_server_ip={{ .ZookeeperServerList }}
# configure directories for job manager
# the same directories must be mounted to dnsmasq and DM container
dnsmasq_conf_dir=/var/lib/dnsmasq
tftp_dir=/var/lib/tftp
dhcp_leases_file=/var/lib/dnsmasq/dnsmasq.leases
dnsmasq_reload_by_signal=True
rabbit_server={{ .RabbitmqServerList }}
rabbit_vhost={{ .RabbitmqVhost }}
rabbit_user={{ .RabbitmqUser }}
rabbit_password={{ .RabbitmqPassword }}
rabbit_use_ssl=True
kombu_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kombu_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kombu_ssl_ca_certs={{ .CAFilePath }}
kombu_ssl_version=tlsv1_2
rabbit_health_check_interval=10
collectors={{ .CollectorServerList }}
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))

// ConfigKeystoneAuthConf is the template of the DeviceManager keystone auth configuration.
var ConfigKeystoneAuthConf = template.Must(template.New("").Parse(`
{{ if eq .AuthMode "keystone" }}
[KEYSTONE]
admin_password = {{ .KeystoneAuthParameters.AdminPassword }}
admin_tenant_name = {{ .KeystoneAuthParameters.AdminTenant }}
admin_user = {{ .KeystoneAuthParameters.AdminUsername }}
auth_host = {{ .KeystoneAuthParameters.Address }}
auth_port = {{ .KeystoneAuthParameters.AdminPort }}
auth_protocol = {{ .KeystoneAuthParameters.AuthProtocol }}
auth_url = {{ .KeystoneAuthParameters.AuthProtocol }}://{{ .KeystoneAuthParameters.Address }}:{{ .KeystoneAuthParameters.AdminPort }}/v3
auth_type = password
{{ if eq .KeystoneAuthParameters.AuthProtocol "https" }}
{{ if .KeystoneAuthParameters.Insecure }}
insecure = {{ .KeystoneAuthParameters.Insecure }}
{{ else }}
cafile = {{ .CAFilePath }}
keyfile = /etc/certificates/server-key-{{ .PodIP }}.pem
certfile = /etc/certificates/server-{{ .PodIP }}.crt
{{ end }}
{{ end }}
user_domain_name = {{ .KeystoneAuthParameters.UserDomainName }}
project_domain_name = {{ .KeystoneAuthParameters.ProjectDomainName }}
region_name = {{ .KeystoneAuthParameters.Region }}
{{ end }}`))

// FabricAnsibleConf is the template of the DeviceManager configuration for fabric management.
var FabricAnsibleConf = template.Must(template.New("").Parse(`[DEFAULTS]
log_file = /var/log/contrail/contrail-fabric-ansible.log
log_level={{ .LogLevel }}
log_local=1
collectors={{ .CollectorServerList }}

[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))

// ConfigDNSMasqBaseConfig is the template of the DNSMasq service configuration.
var ConfigDNSMasqBaseConfig = template.Must(template.New("").Parse(`log-facility=/var/log/contrail/dnsmasq.log
bogus-priv
log-dhcp
{{ if not .UseExternalTFTP }}
enable-tftp
tftp-root=/var/lib/tftp
{{ end }}
`))

// ConfigDNSMasqConfig is the template of the main DNSMasq service configuration.
var ConfigDNSMasqConfig = `conf-dir=/var/lib/dnsmasq/,*.conf
`

// ConfigSchematransformerConfig is the template of the SchemaTransformer service configuration.
var ConfigSchematransformerConfig = template.Must(template.New("").Parse(`[DEFAULTS]
host_ip={{ .ListenAddress }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port={{ .SchemaIntrospectPort}}
api_server_ip={{ .ApiServerList}}
api_server_port=8082
api_server_use_ssl=True
log_file=/var/log/contrail/contrail-schema.log
log_level={{ .LogLevel }}
log_local=1
cassandra_server_list={{ .CassandraServerList }}
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
zk_server_ip={{ .ZookeeperServerList }}
rabbit_server={{ .RabbitmqServerList }}
rabbit_vhost={{ .RabbitmqVhost }}
rabbit_user={{ .RabbitmqUser }}
rabbit_password={{ .RabbitmqPassword }}
rabbit_use_ssl=True
kombu_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kombu_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kombu_ssl_ca_certs={{ .CAFilePath }}
kombu_ssl_version=tlsv1_2
rabbit_health_check_interval=10
collectors={{ .CollectorServerList }}
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}
[SECURITY]
use_certs=True
ca_certs={{ .CAFilePath }}
certfile=/etc/certificates/server-{{ .PodIP }}.crt
keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem`))

// ConfigServicemonitorConfig is the template of the ServiceMonitor service configuration.
var ConfigServicemonitorConfig = template.Must(template.New("").Parse(`[DEFAULTS]
host_ip={{ .ListenAddress }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port={{ .SvcMonitorIntrospectPort}}
api_server_ip={{ .ApiServerList }}
api_server_port=8082
api_server_use_ssl=True
log_file=/var/log/contrail/contrail-svc-monitor.log
log_level={{ .LogLevel }}
log_local=1
cassandra_server_list={{ .CassandraServerList }}
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
zk_server_ip={{ .ZookeeperServerList }}
rabbit_server={{ .RabbitmqServerList }}
rabbit_vhost={{ .RabbitmqVhost }}
rabbit_user={{ .RabbitmqUser }}
rabbit_password={{ .RabbitmqPassword }}
rabbit_use_ssl=True
kombu_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kombu_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kombu_ssl_ca_certs={{ .CAFilePath }}
kombu_ssl_version=tlsv1_2
rabbit_health_check_interval=10
collectors={{ .CollectorServerList }}
analytics_api_ssl_enable = True
analytics_api_insecure_enable = False
analytics_api_ssl_certfile = /etc/certificates/server-{{ .PodIP }}.crt
analytics_api_ssl_keyfile = /etc/certificates/server-key-{{ .PodIP }}.pem
analytics_api_ssl_ca_cert = {{ .CAFilePath }}
[SECURITY]
use_certs=True
keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
certfile=/etc/certificates/server-{{ .PodIP }}.crt
ca_certs={{ .CAFilePath }}
[SCHEDULER]
# Analytics server list used to get vrouter status and schedule service instance
analytics_server_list={{ .AnalyticsServerList }}
aaa_mode={{ .AAAMode }}
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))
