package templates

import (
	htemplate "html/template"

	"github.com/Masterminds/sprig"
)

// KafkaConfig is the template of a Kafka configuration.
var KafkaConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`
broker.id={{ default "1" .BrokerId }}
port=9092
listeners=SSL://{{ .PodIP }}:9092
advertised.listeners=SSL://{{ .PodIP }}:9092
num.network.threads=3
num.io.threads=8
socket.send.buffer.bytes=102400
socket.receive.buffer.bytes=102400
socket.request.max.bytes=104857600
ssl.keystore.location=/etc/keystore/server-keystore.jks
ssl.truststore.location=/etc/keystore/server-truststore.jks
ssl.keystore.password={{ .KeystorePassword }}
ssl.key.password={{ .KeystorePassword }}
ssl.truststore.password={{ .TruststorePassword }}
security.inter.broker.protocol=SSL
ssl.endpoint.identification.algorithm=
zookeeper.connect={{ .ZookeeperServers }}
zookeeper.connection.timeout.ms=6000
advertised.host.name={{ .Hostname }}
log.retention.bytes=268435456
log.retention.hours=24
log.segment.bytes=268435456
log.dirs=/tmp/kafka-logs
num.recovery.threads.per.data.dir=1
num.partitions=30
offsets.topic.replication.factor=1
transaction.state.log.replication.factor=1
transaction.state.log.min.isr=1
default.replication.factor={{ default "1" .ReplicationFactor }}
min.insync.replicas={{ default "1" .MinInsyncReplicas }}
group.initial.rebalance.delay.ms=0
log.cleanup.policy=delete
log.cleaner.threads=2
log.cleaner.dedupe.buffer.size=250000000
offsets.topic.replication.factor=1
reserved.broker.max.id=100001`))
