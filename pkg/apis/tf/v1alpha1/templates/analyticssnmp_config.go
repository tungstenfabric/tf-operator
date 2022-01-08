package templates

import (
	htemplate "html/template"

	"github.com/Masterminds/sprig"
)

// AnalyticsSnmpCollectorConfig is a templete for snmp collector config
var AnalyticsSnmpCollectorConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
[DEFAULTS]
host_ip={{ .ListenAddress }}
hostname={{ .Hostname }}
scan_frequency={{ default "600" .SnmpCollectorScanFrequency }}
fast_scan_frequency={{ default "60" .SnmpCollectorFastScanFrequency }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port={{ default "5920" .SnmpCollectorIntrospectListenPort }}
log_file={{ default "/var/log/contrail/tf-snmp-collector.log" .LogFile }}
log_level={{ default "SYS_INFO" .LogLevel }}
log_local={{ default "1" .LogLocal }}
collectors={{ .CollectorServers }}
zookeeper={{ .ZookeeperServers }}
[API_SERVER]
api_server_list={{ .ConfigServers }}
api_server_use_ssl=True
[CONFIGDB]
config_db_server_list={{ .ConfigDbServerList }}
config_db_use_ssl=True
config_db_ca_certs={{ .CassandraSslCaCertfile }}
rabbitmq_server_list={{ .RabbitmqServerList }}
rabbitmq_vhost={{ .RabbitmqVhost }}
rabbitmq_user={{ .RabbitmqUser }}
rabbitmq_password={{ .RabbitmqPassword }}
rabbitmq_use_ssl=True
rabbitmq_ssl_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
rabbitmq_ssl_certfile=/etc/certificates/client-{{ .PodIP }}.crt
rabbitmq_ssl_ca_certs={{ .CAFilePath }}
rabbitmq_ssl_version=tlsv1_2
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/client-{{ .PodIP }}.crt
sandesh_server_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_server_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}
`))

// AnalyticsSnmpTopologyConfig is a template for snmp topology config
var AnalyticsSnmpTopologyConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
[DEFAULTS]
host_ip={{ .ListenAddress }}
scan_frequency={{ default "600" .SnmpTopologyScanFrequency }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port={{ default "5921" .SnmpTopologyIntrospectListenPort }}
log_file={{ default "/var/log/contrail/tf-topology.log" .LogFile }}
log_level={{ default "SYS_INFO" .LogLevel }}
log_local={{ default "1" .LogLocal }}
analytics_api={{ .AnalyticsServers }}
collectors={{ .CollectorServers }}
zookeeper={{ .ZookeeperServers }}
[API_SERVER]
api_server_list={{ .ConfigServers }}
api_server_use_ssl=True
[CONFIGDB]
config_db_server_list={{ .ConfigDbServerList }}
config_db_use_ssl=True
config_db_ca_certs={{ .CassandraSslCaCertfile }}
rabbitmq_server_list={{ .RabbitmqServerList }}
rabbitmq_vhost={{ .RabbitmqVhost }}
rabbitmq_user={{ .RabbitmqUser }}
rabbitmq_password={{ .RabbitmqPassword }}
rabbitmq_use_ssl=True
rabbitmq_ssl_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
rabbitmq_ssl_certfile=/etc/certificates/client-{{ .PodIP }}.crt
rabbitmq_ssl_ca_certs={{ .CAFilePath }}
rabbitmq_ssl_version=tlsv1_2
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/client-{{ .PodIP }}.crt
sandesh_server_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_server_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}
`))
