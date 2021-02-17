package templates

import "text/template"

// NodemanagerRunner is the template of the Provisioner runner
// TODO: might be moved to tf-container-builder as main entrypoint or function
// to commonize approach with sighup between orchestrators
var NodemanagerRunner = template.Must(template.New("").Parse(`#!/bin/bash
[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x
function link_file() {
  local src=/etc/contrailconfigmaps/$1
  local dst=/etc/contrail/${2:-${1}}
  echo INFO: $(date): wait for $src
  while [ ! -e $src ] ; do sleep 1; done
  echo INFO: $(date): link $src => $dst
  ln -sf $src $dst
	cat $dst
}

sig_file=/var/run/nodemanager-${NODE_TYPE}-${POD_IP}.pid.sighup
pid_file=/var/run/nodemanager-${NODE_TYPE}-${POD_IP}.pid
trap 'trap_sighup' SIGHUP
trap 'kill_process' SIGQUIT SIGTERM SIGINT
function kill_process() {
	local pid=$(cat $pid_file)
	echo "INFO: $(date): terminate $pid"
	rm -f $pid_file
	[ -n "$pid" ] || exit -1
	kill $pid
	for i in {1..5}; do sleep 2; kill -0 $pid || break; done
	if kill -0 $pid ; then kill -9 $pid ; fi
}
function trap_sighup() {
	touch $sig_file
	echo "INFO: $(date): SIGHUP"
	kill_process
}
touch $sig_file
while [ -e $sig_file ] ; do
	rm -f $sig_file $pid_file

	link_file ${NODE_TYPE}-nodemgr.env.${POD_IP} ${NODE_TYPE}-nodemgr.env
	link_file vnc_api_lib.ini.${POD_IP} vnc_api_lib.ini
	link_file ${NODE_TYPE}-nodemgr.conf.${POD_IP} contrail-${NODE_TYPE}-nodemgr.conf
	
	source /etc/contrail/${NODE_TYPE}-nodemgr.env
	/usr/bin/contrail-nodemgr --nodetype=contrail-${NODE_TYPE} &
	pid=$!
	echo $pid > $pid_file
	echo "INFO: $(date): nodemgr $NODE_TYPE started, pid=$pid"
	wait $pid
done

`))

// NodemanagerEnv env for nodemgr
var NodemanagerEnv = template.Must(template.New("").Parse(`
export ANALYTICSDB_NODES={{ .AnalyticsDBNodes }}
export CONFIGDB_NODES={{ .ConfigDBNodes }}
`))
