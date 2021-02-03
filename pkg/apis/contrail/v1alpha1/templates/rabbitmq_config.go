package templates

import "text/template"

// RabbitmqConfig is the template of the Rabbitmq service configuration.
var RabbitmqConfig = template.Must(template.New("").Parse(`#!/bin/bash
echo $RABBITMQ_ERLANG_COOKIE > /var/lib/rabbitmq/.erlang.cookie
chmod 0600 /var/lib/rabbitmq/.erlang.cookie
touch /var/run/rabbitmq.pid
chown -R rabbitmq:rabbitmq /var/lib/rabbitmq /var/run/rabbitmq.pid
if [[ $(grep $POD_IP /etc/rabbitmq/0) ]] ; then
  exec rabbitmq-server
else
  rabbitmqctl --node rabbit@${POD_IP} forget_cluster_node rabbit@${POD_IP}
  rabbitmqctl --node rabbit@$(cat /etc/rabbitmq/0) ping
  while [[ $? -ne 0 ]]; do
	  rabbitmqctl --node rabbit@$(cat /etc/rabbitmq/0) ping
  done
  rabbitmq-server -detached
  rabbitmqctl --node rabbit@$(cat /etc/rabbitmq/0) node_health_check
  while [[ $? -ne 0 ]]; do
	  rabbitmqctl --node rabbit@$(cat /etc/rabbitmq/0) node_health_check
  done
  rabbitmqctl stop_app
  sleep 2
  rabbitmqctl join_cluster rabbit@$(cat /etc/rabbitmq/0)
  rabbitmqctl shutdown
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
