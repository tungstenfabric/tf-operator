package templates

import (
	htemplate "html/template"

	"github.com/Masterminds/sprig"
)

// AnalyticsAlarmgenConfig is a templete for alarm gen config
var AnalyticsAlarmgenConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
[DEFAULTS]
host_ip={{ .ListenAddress }}
partitions={{ default "30" .AlarmgenPartitions }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port={{ default "5995" .AlarmgenIntrospectListenPort }}
log_file={{ default "/var/log/contrail/tf-alarm-gen.log" .LogFile }}
log_level={{ default "SYS_INFO" .LogLevel }}
log_local={{ default "1" .LogLocal }}
collectors={{ .CollectorServers }}
zk_list={{ .ZookeeperServers }}
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
kombu_ssl_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kombu_ssl_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kombu_ssl_ca_certs={{ .CAFilePath }}
kombu_ssl_version=tlsv1_2
[KAFKA]
kafka_broker_list={{ .KafkaServers }}
kafka_ssl_enable=True
kafka_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
kafka_certfile=/etc/certificates/server-{{ .PodIP }}.crt
kafka_ca_cert={{ .CAFilePath }}
[REDIS]
redis_server_port=6379
redis_uve_list={{ .RedisServerList }}
redis_password=
redis_use_ssl=True
redis_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
redis_certfile=/etc/certificates/server-{{ .PodIP }}.crt
redis_ca_cert={{ .CAFilePath }}
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))

// AnalyticsAlarmNodemanagerConfig is the template of the Analytics Alarm Nodemanager service configuration.
var AnalyticsAlarmNodemanagerConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
[DEFAULTS]
http_server_ip={{ .InstrospectListenAddress }}
log_file={{ default "/var/log/contrail/contrail-analyticsalarm-nodemgr.log" .LogFile }}
log_level={{ default "SYS_INFO" .LogLevel }}
log_local={{ default "1" .LogLocal }}
hostname={{ .Hostname }}
hostip={{ .ListenAddress }}
db_port={{ .CassandraPort }}
db_jmx_port={{ .CassandraJmxPort }}
db_use_ssl=True
[COLLECTOR]
server_list={{ .CollectorServerList }}
[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}`))

// AnalyticsAlarmVncConfig is a template for vnc_api.ini file
var AnalyticsAlarmVncConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
[global]
WEB_SERVER = {{ .ConfigNodes }}
WEB_PORT = {{ .ConfigApiPort }}
BASE_URL = /
use_ssl = True
cafile = {{ .CAFilePath }}
[auth]
AUTHN_TYPE = noauth`))
