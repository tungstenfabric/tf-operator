package templates

import "text/template"

var addFunc = template.FuncMap{
	"add": func(i, j int) int {
		return i + j
	},
}

// RabbitmqConfig is the template of the Rabbitmq service configuration.
var RabbitmqConfig = template.Must(template.New("").Parse(`#!/bin/bash
function link_file() {
  local src=/etc/contrailconfigmaps/$1
  local dst=/etc/rabbitmq/${2:-${1}}
  echo INFO: $(date): wait for $src
  while [ ! -e $src ] ; do sleep 1; done
  echo INFO: $(date): link $src => $dst
  ln -sf $src $dst
}

mkdir -p /etc/rabbitmq
link_file rabbitmq.conf.${POD_IP} rabbitmq.conf
link_file rabbitmq-env.conf.${POD_IP} rabbitmq-env.conf
link_file rabbitmq.nodes
link_file plugins.conf
link_file definitions.json
link_file rabbitmq-common.env
link_file 0

source /etc/rabbitmq/rabbitmq-common.env
mkdir -p /var/lib/rabbitmq /var/log/rabbitmq
echo $RABBITMQ_ERLANG_COOKIE > /var/lib/rabbitmq/.erlang.cookie
set -x
chmod 0600 /var/lib/rabbitmq/.erlang.cookie
touch /var/run/rabbitmq.pid
chown -R rabbitmq:rabbitmq /var/lib/rabbitmq /var/log/rabbitmq /var/run/rabbitmq.pid /etc/rabbitmq
export RABBITMQ_NODENAME=rabbit@${POD_IP}
bootstrap_node="rabbit@$(cat /etc/rabbitmq/0)"
if [[ "$RABBITMQ_NODENAME" == "$bootstrap_node" ]] ; then
  rabbitmq-server
else
  while true ; do
    rabbitmqctl --node $RABBITMQ_NODENAME shutdown || true
    rabbitmq-server -detached || exit 1
    while true; do
      rabbitmqctl --node $bootstrap_node ping && rabbitmqctl --node $RABBITMQ_NODENAME ping && break
    done
    sleep $(( $RANDOM % 5 ))
    rabbitmqctl --node $RABBITMQ_NODENAME stop_app || continue
    rabbitmqctl --node $bootstrap_node forget_cluster_node $RABBITMQ_NODENAME
    rabbitmqctl --node $RABBITMQ_NODENAME join_cluster $bootstrap_node || continue
    break
  done
  rabbitmqctl --node $RABBITMQ_NODENAME shutdown
  rabbitmq-server
fi
`))

// RabbitmqDefinition is the template for Rabbitmq user/vhost configuration
var RabbitmqDefinition = template.Must(template.New("").Funcs(addFunc).Parse(`{
  "users": [
    {
      "name": "{{ .RabbitmqUser }}",
      "password_hash": "{{ .RabbitmqPassword }}",
      "tags": "administrator"
    }
  ],
  "vhosts": [
    {
      "name": "{{ .RabbitmqVhost }}"
    }
  ],
  "permissions": [
    {
      "user": "{{ .RabbitmqUser }}",
      "vhost": "{{ .RabbitmqVhost }}",
      "configure": ".*",
      "write": ".*",
      "read": ".*"
    }
  ],
  "policies": [
    {
      "vhost": "{{ .RabbitmqVhost }}",
      "name": "ha",
      "pattern": "^(?!amq\.).*",
      "definition": {
          "ha-mode": "{{ .MirroredQueueMode }}",
          "ha-sync-mode": "automatic",
          "ha-sync-batch-size": 5
      }
    }
  ]
}
`))

// RabbitmqPodConfig is the template for Rabbitmq pod configuration
var RabbitmqPodConfig = template.Must(template.New("").Funcs(addFunc).Parse(`listeners.tcp = none
listeners.ssl.default = {{ .RabbitmqPort }}
loopback_users = none
management.tcp.port = {{ add .RabbitmqPort 10000}}
management.load_definitions = /etc/rabbitmq/definitions.json
ssl_options.cacertfile = {{ .SignerCAFilepath }}
ssl_options.keyfile = /etc/certificates/server-key-{{ .PodIP }}.pem
ssl_options.certfile = /etc/certificates/server-{{ .PodIP }}.crt
ssl_options.verify = verify_peer
ssl_options.fail_if_no_peer_cert = true
cluster_partition_handling = {{ .ClusterPartitionHandling }}
{{ if .TCPListenOptions }}
{{ if .TCPListenOptions.Backlog }}tcp_listen_options.backlog = {{ .TCPListenOptions.Backlog }}{{ end }}
{{ if .TCPListenOptions.Nodelay }}tcp_listen_options.nodelay = {{ .TCPListenOptions.Nodelay }}{{ end }}
{{ if .TCPListenOptions.LingerOn }}tcp_listen_options.linger.on = {{ .TCPListenOptions.LingerOn }}{{ end }}
{{ if .TCPListenOptions.LingerTimeout }}tcp_listen_options.linger.timeout = {{ .TCPListenOptions.LingerTimeout }}{{ end }}
{{ if .TCPListenOptions.ExitOnClose }}tcp_listen_options.exit_on_close = {{ .TCPListenOptions.ExitOnClose }}{{ end }}
{{ end }}
{{ $podsCount := len .PodsList }}{{ if gt $podsCount 1 }}cluster_formation.peer_discovery_backend = classic_config
{{ range $idx, $pod := .PodsList }}cluster_formation.classic_config.nodes.{{ add $idx 1 }} = rabbit@{{ $pod.Status.PodIP }}
{{ end }}
{{ end }}
`))
