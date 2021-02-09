package templates

import "text/template"

var ConfigAPIVNC = template.Must(template.New("").Parse(`[global]
WEB_SERVER = {{ .APIServerList }}
WEB_PORT = {{ .APIServerPort }} ; connection to api-server directly
BASE_URL = /
use_ssl = True
cafile = {{ .CAFilePath }}

{{ if eq .AuthMode "keystone" }}
[auth]
AUTHN_TYPE = {{ .AuthMode }}
; Authentication settings (optional)
AUTHN_PROTOCOL = {{ .KeystoneAuthProtocol }}
AUTHN_SERVER = {{ .KeystoneAddress }}
AUTHN_PORT = {{ .KeystonePort }}
AUTHN_URL = /v3/auth/tokens
AUTHN_DOMAIN = {{ .KeystoneUserDomainName }}
cafile = {{ .CAFilePath }}
;AUTHN_TOKEN_URL = http://127.0.0.1:35357/v2.0/tokens
{{ else }}
[auth]
AUTHN_TYPE = noauth
{{ end }}

`))
