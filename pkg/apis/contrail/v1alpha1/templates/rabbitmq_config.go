package templates

import "text/template"

// RabbitmqConfig is the template of the Rabbitmq service configuration.
var RabbitmqConfig = template.Must(template.New("").Parse(`#!/bin/bash
function link_file() {
  local src=/etc/rabbitmqconfigmaps/$1
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
  rabbitmq-server -detached
  while true ; do
    while true; do
      rabbitmqctl --node $bootstrap_node ping && rabbitmqctl --node $RABBITMQ_NODENAME ping && break
    done
    rabbitmqctl --node $RABBITMQ_NODENAME stop_app || continue
    rabbitmqctl --node $bootstrap_node forget_cluster_node $RABBITMQ_NODENAME
    rabbitmqctl --node $RABBITMQ_NODENAME join_cluster $bootstrap_node || continue
    rabbitmqctl --node $RABBITMQ_NODENAME shutdown
    break
  done  
  rabbitmq-server
fi

`))

// RabbitmqDefinition is the template for Rabbitmq user/vhost configuration
var RabbitmqDefinition = template.Must(template.New("").Parse(`{
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
}
`))
