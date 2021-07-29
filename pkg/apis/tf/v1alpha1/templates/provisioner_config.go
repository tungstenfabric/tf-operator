package templates

import "text/template"

// ProvisionerConfig is the template of the Provisioner env configuration
var ProvisionerConfig = template.Must(template.New("").Parse(`export SSL_ENABLE=true
export SERVER_CA_CERTFILE={{ .SignerCAFilepath }}
export SERVER_CERTFILE="/etc/certificates/server-${POD_IP}.crt"
export SERVER_KEYFILE="/etc/certificates/server-key-${POD_IP}.pem"
{{ if .ClusterNodes.ConfigNodes }}
export CONFIG_NODES={{ .ClusterNodes.ConfigNodes }}
{{ end }}
{{ if .ClusterNodes.ControlNodes }}
export CONTROL_NODES={{ .ClusterNodes.ControlNodes }}
{{ end }}
{{ if .ClusterNodes.AnalyticsNodes }}
export ANALYTICS_NODES={{ .ClusterNodes.AnalyticsNodes }}
{{ end }}
{{ if .ClusterNodes.AnalyticsDBNodes }}
export ANALYTICSDB_NODES={{ .ClusterNodes.AnalyticsDBNodes }}
{{ end }}
{{ if .ClusterNodes.AnalyticsSnmpNodes }}
export ANALYTICS_SNMP_NODES={{ .ClusterNodes.AnalyticsSnmpNodes }}
{{ end }}
{{ if .ClusterNodes.AnalyticsAlarmNodes }}
export ANALYTICS_ALARM_NODES={{ .ClusterNodes.AnalyticsAlarmNodes }}
{{ end }}
{{ if .Hostname }}
export CONTROL_HOSTNAME={{ .Hostname }}
export VROUTER_HOSTNAME={{ .Hostname }}
{{ end }}
{{ if .Retries }}
export PROVISION_RETRIES={{ .Retries }}
{{ end }}
{{ if .Delay }}
export PROVISION_DELAY={{ .Delay }}
{{ end }}
export AUTH_MODE={{ .AuthMode }}
{{ if eq .AuthMode "keystone" }}
export KEYSTONE_AUTH_PROTO="{{ .KeystoneAuthParameters.AuthProtocol }}"
export KEYSTONE_AUTH_HOST="{{ .KeystoneAuthParameters.Address }}"
export KEYSTONE_AUTH_ADMIN_PORT="{{ .KeystoneAuthParameters.AdminPort }}"
export KEYSTONE_AUTH_PROJECT_DOMAIN_NAME="{{ .KeystoneAuthParameters.ProjectDomainName }}"
export KEYSTONE_AUTH_INSECURE="{{ .KeystoneAuthParameters.Insecure }}"
{{ if eq .KeystoneAuthParameters.AuthProtocol "https" }}
{{ if not .KeystoneAuthParameters.Insecure }}
export KEYSTONE_AUTH_CA_CERTFILE="{{ .SignerCAFilepath }}"
{{ end }}
{{ end }}
export KEYSTONE_AUTH_ADMIN_PASSWORD="{{ .KeystoneAuthParameters.AdminPassword }}"
export KEYSTONE_AUTH_ADMIN_TENANT="{{ .KeystoneAuthParameters.AdminTenant }}"
export KEYSTONE_AUTH_ADMIN_USER="{{ .KeystoneAuthParameters.AdminUsername }}"
{{ end }}
`))

// ProvisionerRunner is the template of the Provisioner runner
var ProvisionerRunner = template.Must(template.New("").Parse(`#!/bin/bash
[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x
function source_env() {
  local cfg=$1
  [ -e $cfg ] || return 1
  source $cfg
  echo -e "INFO: $(date): config\n$(cat $cfg)"
  [ -n "$CONFIG_NODES" ] || return 1
  return 0
}

cfg="/etc/contrailconfigmaps/{{ .ConfigName }}"
echo "INFO: $(date): wait for config $cfg or $cfg.${POD_IP}"
while true ; do
  source_env "${cfg}" && break
  source_env "${cfg}.${POD_IP}" && break
  sleep 5
done
export PROVISION_RETRIES=1000
export PROVISION_DELAY=5
exec /entrypoint.sh /usr/bin/tail -f /dev/null
`))
