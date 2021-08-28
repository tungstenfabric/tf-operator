package templates

import (
	htemplate "html/template"
	"text/template"

	"github.com/Masterminds/sprig"
)

// NodemanagerEnv env for nodemgr
var NodemanagerEnv = template.Must(template.New("").Parse(`
export ANALYTICSDB_NODES={{ .AnalyticsDBNodes }}
export CONFIGDB_NODES={{ .ConfigDBNodes }}
`))

// NodemanagerConfig is a template of nodemanager configuration
var NodemanagerConfig = htemplate.Must(htemplate.New("").Funcs(sprig.FuncMap()).Parse(`[DEFAULTS]
http_server_ip={{ .InstrospectListenAddress }}
log_file={{ default "/var/log/contrail/nodemgr.log" .LogFile }}
log_level={{ default "SYS_INFO" .LogLevel }}
log_local={{ default "1" .LogLocal }}
hostname={{ .Hostname }}
hostip={{ .ListenAddress }}
db_port={{ .CassandraPort }}
db_jmx_port={{ .CassandraJmxPort }}
db_use_ssl=True
{{ if .MinimumDiskGB -}}
    minimum_diskGB={{ .MinimumDiskGB }}
{{- end }}

[COLLECTOR]
server_list={{ .CollectorServerList }}

[SANDESH]
introspect_ssl_enable=True
introspect_ssl_insecure=True
sandesh_ssl_enable=True
sandesh_keyfile=/etc/certificates/server-key-{{ .PodIP }}.pem
sandesh_certfile=/etc/certificates/server-{{ .PodIP }}.crt
sandesh_ca_cert={{ .CAFilePath }}
`))
