package templates

import (
	"text/template"
)

// StunnelConfig is the template for the Stunnel container
var StunnelConfig = template.Must(template.New("").Parse(`
cert=/etc/stunnel/private.pem
pid=/var/run/stunnel/stunnel.pid
sslVersion=TLSv1.2
foreground=yes
[redis]
accept={{ .ListenAddress }}:{{ .RedisPort }}
connect=127.0.0.1:{{ .RedisPort }}
`))
