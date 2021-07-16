package templates

import "text/template"

// ProvisionerConfig is the template of the Provisioner env configuration
var ProvisionerConfig = template.Must(template.New("").Parse(`export SSL_ENABLE=true
export SERVER_CA_CERTFILE={{ .SignerCAFilepath }}
export SERVER_CERTFILE="/etc/certificates/server-${POD_IP}.crt"
export SERVER_KEYFILE="/etc/certificates/server-key-${POD_IP}.pem"
{{ if .ConfigAPINodes }}
export CONFIG_NODES={{ .ConfigAPINodes }}
{{ end }}
{{ if .ControlNodes }}
export CONTROL_NODES={{ .ControlNodes }}
{{ end }}
{{ if .Hostname }}
export VROUTER_HOSTNAME={{ .Hostname }}
export CONTROL_HOSTNAME={{ .Hostname }}
{{ end }}
{{ if .L3MHCidr }}
export L3MH_CIDR={{ .L3MHCidr }}
{{ end }}
{{ if .PhysicalInterface }}
export PHYSICAL_INTERFACE={{ .PhysicalInterface }}
{{ end }}
{{ if .VrouterGateway }}
export VROUTER_GATEWAY={{ .VrouterGateway }}
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
cfg="/etc/contrailconfigmaps/{{ .ConfigName }}"
cfg_="/etc/contrailconfigmaps/{{ .ConfigName }}.$POD_IP"
echo "INFO: $(date): wait for config $cfg"
config=""
while true ; do
  sleep 5
  [ -e $cfg ] && config=$cfg
  [ -e $cfg_ ] && config=$cfg_
  if [ -n "$config" ]; then
    source $config
    echo -e "INFO: $(date): config\n$(cat $config)"
    [ -z "$CONFIG_NODES" ] || break
  fi
done
export PROVISION_RETRIES=1000
export PROVISION_DELAY=5
exec /entrypoint.sh /usr/bin/tail -f /dev/null
`))
