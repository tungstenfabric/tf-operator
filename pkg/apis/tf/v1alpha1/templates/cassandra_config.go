package templates

import "text/template"

// CassandraConfig is the template of a full Cassandra configuration.
var CassandraConfig = template.Must(template.New("").Parse(`cluster_name: ContrailConfigDB
num_tokens: 256
hinted_handoff_enabled: true
max_hint_window_in_ms: 10800000 # 3 hours
hinted_handoff_throttle_in_kb: 1024
max_hints_delivery_threads: 2
hints_directory: /var/lib/cassandra/hints
hints_flush_period_in_ms: 10000
max_hints_file_size_in_mb: 128
batchlog_replay_throttle_in_kb: 1024
authenticator: AllowAllAuthenticator
authorizer: AllowAllAuthorizer
role_manager: CassandraRoleManager
roles_validity_in_ms: 2000
permissions_validity_in_ms: 2000
credentials_validity_in_ms: 2000
partitioner: org.apache.cassandra.dht.Murmur3Partitioner
data_file_directories:
- /var/lib/cassandra/data
commitlog_directory: /var/lib/cassandra/commitlog
disk_failure_policy: stop
commit_failure_policy: stop
key_cache_size_in_mb:
key_cache_save_period: 14400
row_cache_size_in_mb: 0
row_cache_save_period: 0
counter_cache_size_in_mb:
counter_cache_save_period: 7200
saved_caches_directory: /var/lib/cassandra/saved_caches
commitlog_sync: periodic
commitlog_sync_period_in_ms: 10000
commitlog_segment_size_in_mb: 32
seed_provider:
- class_name: org.apache.cassandra.locator.SimpleSeedProvider
  parameters:
  - seeds: {{ .Seeds }}
concurrent_reads: {{ or .Parameters.ConcurrentReads 32 }}
concurrent_writes: {{ or .Parameters.ConcurrentWrites 32 }}
concurrent_counter_writes: {{ or .Parameters.ConcurrentCounterWrites 32 }}
concurrent_materialized_view_writes: {{ or .Parameters.ConcurrentMaterializedViewWrites 32 }}
concurrent_compactors: {{ or .Parameters.ConcurrentCompactors 1 }}
memtable_flush_writers: {{ or .Parameters.MemtableFlushWriters 2 }}
disk_optimization_strategy: ssd
memtable_allocation_type: {{ or .Parameters.MemtableAllocationType "heap_buffers" }}
index_summary_capacity_in_mb:
index_summary_resize_interval_in_minutes: 60
trickle_fsync: false
trickle_fsync_interval_in_kb: 10240
storage_port: {{ .StoragePort}}
ssl_storage_port: {{ .SslStoragePort }}
listen_address: {{ .ListenAddress }}
broadcast_address: {{ .BroadcastAddress }}
start_native_transport: true
native_transport_port: {{ .CqlPort }}
start_rpc: {{ .StartRPC }}
rpc_address: {{ .RPCAddress }}
rpc_port: {{ .RPCPort }}
broadcast_rpc_address: {{ .RPCBroadcastAddress}}
rpc_keepalive: true
rpc_server_type: sync
thrift_framed_transport_size_in_mb: 15
incremental_backups: false
snapshot_before_compaction: false
auto_snapshot: true
tombstone_warn_threshold: 1000
tombstone_failure_threshold: 100000
column_index_size_in_kb: 64
batch_size_warn_threshold_in_kb: 5
batch_size_fail_threshold_in_kb: 50
compaction_throughput_mb_per_sec: {{ or .Parameters.CompactionThroughputMbPerSec 16 }}
compaction_large_partition_warning_threshold_mb: 100
sstable_preemptive_open_interval_in_mb: 50
read_request_timeout_in_ms: 5000
range_request_timeout_in_ms: 10000
write_request_timeout_in_ms: 2000
counter_write_request_timeout_in_ms: 5000
cas_contention_timeout_in_ms: 1000
truncate_request_timeout_in_ms: 60000
request_timeout_in_ms: 10000
cross_node_timeout: false
endpoint_snitch: SimpleSnitch
dynamic_snitch_update_interval_in_ms: 100
dynamic_snitch_reset_interval_in_ms: 600000
dynamic_snitch_badness_threshold: 0.1
request_scheduler: org.apache.cassandra.scheduler.NoScheduler
# node-to-node encrypion
server_encryption_options:
  internode_encryption: all
  keystore: /etc/keystore/server-keystore.jks
  keystore_password: {{ .KeystorePassword }}
  truststore: /etc/keystore/server-truststore.jks
  truststore_password: {{ .TruststorePassword }}
  require_client_auth: true
  store_type: JKS
# client-to-node encrypion
client_encryption_options:
  enabled: true
  optional: false
  keystore: /etc/keystore/server-keystore.jks
  keystore_password: {{ .KeystorePassword }}
  truststore: /etc/keystore/server-truststore.jks
  truststore_password: {{ .TruststorePassword }}
  require_client_auth: false
  store_type: JKS
internode_compression: all
inter_dc_tcp_nodelay: false
tracetype_query_ttl: 86400
tracetype_repair_ttl: 604800
gc_warn_threshold_in_ms: 1000
enable_user_defined_functions: false
enable_scripted_user_defined_functions: false
windows_timer_interval: 1
transparent_data_encryption_options:
  enabled: false
  chunk_length_kb: 64
  cipher: AES/CBC/PKCS5Padding
  key_alias: testing:1
  key_provider:
  - class_name: org.apache.cassandra.security.JKSKeyProvider
    parameters:
    - keystore: conf/.keystore
      keystore_password: cassandra
      store_type: JCEKS
      key_password: cassandra
auto_bootstrap: true
`))

// CassandraCqlShrc is a template for cqlsh tool
var CassandraCqlShrc = template.Must(template.New("").Parse(`
[ssl]
certfile = {{ .CAFilePath }}
version = SSLv23
userkey = /etc/certificates/client-key-{{ .ListenAddress }}.pem
usercert = /etc/certificates/client-{{ .ListenAddress }}.crt
`))

// CassandraCommandTemplate start script
var CassandraCommandTemplate = template.Must(template.New("").Parse(`
function _prepare_keystore() {
  local type=$1
  rm -f /etc/keystore/${type}-truststore.jks /etc/keystore/${type}-keystore.jks ;
  mkdir -p /etc/keystore ;
  openssl pkcs12 -export -in /etc/certificates/${type}-${POD_IP}.crt -inkey /etc/certificates/${type}-key-${POD_IP}.pem -chain -CAfile {{ .CAFilePath }} -password pass:{{ .TruststorePassword }} -name $type -out TmpFileKeyStore.$type ;
  openssl pkcs12 -password pass:{{ .TruststorePassword }} -in TmpFileKeyStore.$type -info -chain -nokeys
  openssl pkcs12 -password pass:{{ .TruststorePassword }} -in TmpFileKeyStore.$type -info -chain -nokeys -cacerts 2>/dev/null | sed -n '/-\+BEGIN.*-\+/,/-\+END .*-\+/p' > TmpCA.pem
  cat TmpCA.pem
  keytool -keystore /etc/keystore/${type}-truststore.jks -keypass {{ .KeystorePassword }} -storepass {{ .TruststorePassword }} -noprompt -alias CARoot -import -file TmpCA.pem ;
  keytool -importkeystore -deststorepass {{ .KeystorePassword }} -destkeypass {{ .KeystorePassword }} -destkeystore /etc/keystore/${type}-keystore.jks -deststoretype pkcs12 -srcstorepass {{ .TruststorePassword }} -srckeystore TmpFileKeyStore.$type -srcstoretype PKCS12 -alias $type -noprompt ;
}

# generate server keystore for ssl
_prepare_keystore server

# generate client keystore for ssl
_prepare_keystore client

# for cqlsh cmd tool
ln -sf /etc/contrailconfigmaps/cqlshrc.${POD_IP} /root/.cqlshrc ;

# cassandra docker-entrypoint tries patch the config, and nodemanager uses hardcoded path to
# detect cassandra data path for size checks, this file will contains wrong seeds as entrypoint
# sets it from env variable
rm -f /etc/cassandra/cassandra.yaml ;
cp /etc/contrailconfigmaps/cassandra.${POD_IP}.yaml /etc/cassandra/cassandra.yaml ;
cat /etc/cassandra/cassandra.yaml ;

# for gracefull shutdown implemented in docker-entrypoint.sh in trap_cassandra_term
export CASSANDRA_JMX_LOCAL_PORT={{ .JmxLocalPort }}
export CASSANDRA_LISTEN_ADDRESS=${POD_IP}

# start service
exec /docker-entrypoint.sh -f -Dcassandra.jmx.local.port={{ .JmxLocalPort }} -Dcassandra.config=file:///etc/contrailconfigmaps/cassandra.${POD_IP}.yaml
`))
