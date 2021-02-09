package templates

import "text/template"

// NodemanagerRunner is the template of the Provisioner runner
// TODO: might be moved to tf-container-builder as main entrypoint or function
// to commonize approach with sighup between orchestrators
var NodemanagerRunner = template.Must(template.New("").Parse(`#!/bin/bash
[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x
cfg_file=/etc/contrailconfigmaps/${NODE_TYPE}-nodemanager.conf.${POD_IP}
vnc_file=/etc/contrailconfigmaps/vnc_api_lib.ini.${POD_IP}
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
	
	echo "INFO: $(date): wait for config $cfg_file"
	while [ ! -e $cfg_file ] ; do sleep 5 ; done
	echo "INFO: $(date): got config:"
	cat $cfg_file
	echo ""
	echo "INFO: $(date): wait for vnc config $vnc_file"
	while [ ! -e $vnc_file ] ; do sleep 5 ; done
	echo "INFO: $(date): got vnc config"
	cat $vnc_file
	echo ""
	echo "INFO: $(date): link configs to /etc/contrail"
	ln -sf $cfg_file /etc/contrail/contrail-${NODE_TYPE}-nodemgr.conf
	ln -sf $vnc_file /etc/contrail/vnc_api_lib.ini

	/usr/bin/contrail-nodemgr --nodetype=contrail-${NODE_TYPE} &
	pid=$!
	echo $pid > $pid_file
	echo "INFO: $(date): nodemgr $NODE_TYPE started, pid=$pid"
	wait $pid
done

`))
