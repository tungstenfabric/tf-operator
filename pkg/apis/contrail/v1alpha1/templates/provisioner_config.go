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
{{ if .Retries }}
export PROVISION_RETRIES={{ .Retries }}
{{ end }}
{{ if .Delay }}
export PROVISION_DELAY={{ .Delay }}
{{ end }}
`))

// ProvisionerRunner is the template of the Provisioner runner
var ProvisionerRunner = template.Must(template.New("").Parse(`#!/bin/bash
[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x
cfg=/etc/contrailconfigmaps/{{ .ConfigName }}
echo "INFO: $(date): wait for config $cfg"
while true ; do
	sleep 5
	[ -e $cfg ] || continue
	source $cfg
	echo -e "INFO: $(date): config\n$(cat $cfg)"
	[ -z "$CONFIG_NODES" ] || break
done
exec /entrypoint.sh /usr/bin/tail -f /dev/null
`))
