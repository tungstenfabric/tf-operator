package templates

import "text/template"

// RabbitmqConfig is the template of the Rabbitmq service configuration.
var RabbitmqConfig = template.Must(template.New("").Parse(`#!/bin/bash
mkdir -p /var/lib/rabbitmq /var/log/rabbitmq
echo $RABBITMQ_ERLANG_COOKIE > /var/lib/rabbitmq/.erlang.cookie
set -x
chmod 0600 /var/lib/rabbitmq/.erlang.cookie
touch /var/run/rabbitmq.pid
chown -R rabbitmq:rabbitmq /var/lib/rabbitmq /var/log/rabbitmq /var/run/rabbitmq.pid
export RABBITMQ_NODENAME=rabbit@${POD_IP}
bootstrap_node="rabbit@$(cat /etc/rabbitmq/0)"
if [[ "$RABBITMQ_NODENAME" == "$bootstrap_node" ]] ; then
  exec rabbitmq-server
else
  rabbitmq-server -detached
  while true; do
    rabbitmqctl --node $bootstrap_node ping && break
  done
  rabbitmqctl --node $RABBITMQ_NODENAME stop_app
  rabbitmqctl --node $bootstrap_node forget_cluster_node $RABBITMQ_NODENAME
  rabbitmqctl --node $RABBITMQ_NODENAME join_cluster $bootstrap_node
  rabbitmqctl --node $RABBITMQ_NODENAME shutdown
  exec rabbitmq-server
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
