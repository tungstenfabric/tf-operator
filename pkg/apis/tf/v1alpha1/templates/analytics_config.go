package templates

import "text/template"

// AnalyticsapiConfig is the template of the AnalyticsAPI service configuration.
var AnalyticsapiConfig = template.Must(template.New("").Parse(`[DEFAULTS]
host_ip={{ .ListenAddress }}
http_server_port={{ .AnalyticsApiIntrospectPort}}
http_server_ip={{ .InstrospectListenAddress }}
rest_api_port=8081
rest_api_ip={{ .ListenAddress }}
{{ if .QueryEngineEnabled }}
partitions=30
{{ end }}
aaa_mode={{ .AAAMode }}
log_file=/var/log/contrail/contrail-analytics-api.log
log_level={{ .LogLevel }}
log_local=1
# Sandesh send rate limit can be used to throttle system logs transmitted per
# second. System logs are dropped if the sending rate is exceeded
#sandesh_send_rate_limit =
collectors={{ .CollectorServerList}}
api_server={{ .ApiServerList }}
api_server_use_ssl=True
zk_list={{ .ZookeeperServerList }}
analytics_api_ssl_enable = True
analytics_api_insecure_enable = True
analytics_api_ssl_certfile = /etc/certificates/server-{{ .PodIP }}.crt
analytics_api_ssl_keyfile = /etc/certificates/server-key-{{ .PodIP }}.pem
analytics_api_ssl_ca_cert = {{ .CAFilePath }}
[REDIS]
{{ if .QueryEngineEnabled }}
redis_query_port={{ .RedisPort }}
{{ end }}
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
sandesh_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/client-{{ .PodIP }}.crt
sandesh_server_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_server_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}
`))

// CollectorConfig is the template of the Collector service configuration.
var CollectorConfig = template.Must(template.New("").Parse(`[DEFAULT]
analytics_data_ttl={{ .AnalyticsDataTTL }}
analytics_config_audit_ttl={{ .AnalyticsConfigAuditTTL }}
analytics_statistics_ttl={{ .AnalyticsStatisticsTTL }}
analytics_flow_ttl={{ .AnalyticsFlowTTL }}
partitions=30
hostname={{ .Hostname }}
hostip={{ .ListenAddress }}
http_server_port={{ .CollectorIntrospectPort}}
http_server_ip={{ .InstrospectListenAddress }}
syslog_port=514
sflow_port=6343
ipfix_port=4739
# log_category=
log_file=/var/log/contrail/contrail-collector.log
log_files_count=10
log_file_size=1048576
log_level={{ .LogLevel }}
log_local=1
# sandesh_send_rate_limit=
{{ if .QueryEngineEnabled }}
cassandra_server_list={{ .AnalyticsdbCassandraServerList }}
{{ end }}
zookeeper_server_list={{ .ZookeeperServerList }}
[CASSANDRA]
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
[COLLECTOR]
port=8086
server={{ .ListenAddress }}
protobuf_port=3333
[STRUCTURED_SYSLOG_COLLECTOR]
# TCP & UDP port to listen on for receiving structured syslog messages
port=3514
# List of external syslog receivers to forward structured syslog messages in ip:port format separated by space
# tcp_forward_destination=10.213.17.53:514
[API_SERVER]
# List of api-servers in ip:port format separated by space
api_server_list={{ .ApiServerList }}
api_server_use_ssl=True
[REDIS]
port={{ .RedisPort }}
server=127.0.0.1
password=
{{ if .KafkaServerList }}
[KAFKA]
kafka_broker_list={{ .KafkaServerList }}
kafka_ssl_enable=True
kafka_keyfile=/etc/certificates/client-key-{{ .PodIP }}.pem
kafka_certfile=/etc/certificates/client-{{ .PodIP }}.crt
kafka_ca_cert={{ .CAFilePath }}
{{ end }}
[CONFIGDB]
config_db_server_list={{ .CassandraServerList }}
config_db_use_ssl=True
config_db_ca_certs={{ .CAFilePath }}
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
{{ if .QueryEngineEnabled }}
[DATABASE]
disk_usage_percentage.high_watermark0=90
disk_usage_percentage.low_watermark0=85
disk_usage_percentage.high_watermark1=80
disk_usage_percentage.low_watermark1=75
disk_usage_percentage.high_watermark2=70
disk_usage_percentage.low_watermark2=60
pending_compaction_tasks.high_watermark0=400
pending_compaction_tasks.low_watermark0=300
pending_compaction_tasks.high_watermark1=200
pending_compaction_tasks.low_watermark1=150
pending_compaction_tasks.high_watermark2=100
pending_compaction_tasks.low_watermark2=80
high_watermark0.message_severity_level=SYS_EMERG
low_watermark0.message_severity_level=SYS_ALERT
high_watermark1.message_severity_level=SYS_ERR
low_watermark1.message_severity_level=SYS_WARN
high_watermark2.message_severity_level=SYS_DEBUG
low_watermark2.message_severity_level=INVALID
{{ end }}
`))
