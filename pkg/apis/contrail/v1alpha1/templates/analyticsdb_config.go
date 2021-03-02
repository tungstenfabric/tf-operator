package templates

import "text/template"

// QueryEngineConfig is the template of the AnalyticsDB Nodemanager service configuration.
var QueryEngineConfig = template.Must(template.New("").Parse(`[DEFAULT]
analytics_data_ttl={{ .AnalyticsDataTTL }}
hostname={{ .Hostname }}
hostip={{ .ListenAddress }}
http_server_ip={{ .InstrospectListenAddress }}
http_server_port=8091
log_file=/var/log/contrail/contrail-query-engine.log
log_level={{ .LogLevel }}
log_local=1
max_slice=100
max_tasks=16
start_time=0
# Sandesh send rate limit can be used to throttle system logs transmitted per
# second. System logs are dropped if the sending rate is exceeded
# sandesh_send_rate_limit=
cassandra_server_list={{ .CassandraServerList }}
collectors={{ .CollectorServerList }}
[CASSANDRA]
cassandra_use_ssl=true
cassandra_ca_certs={{ .CAFilePath }}
[REDIS]
server_list={{ .RedisServerList }}
password=
redis_ssl_enable=True
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

// AnalyticsDBNodemanagerConfig is the template of the AnalyticsDB Nodemanager service configuration.
var AnalyticsDBNodemanagerConfig = template.Must(template.New("").Parse(`[DEFAULTS]
http_server_ip={{ .InstrospectListenAddress }}
log_file=/var/log/contrail/contrail-config-nodemgr.log
log_level={{ .LogLevel }}
log_local=1
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
