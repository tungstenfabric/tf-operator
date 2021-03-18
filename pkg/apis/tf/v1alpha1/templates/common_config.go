package templates

import (
	htemplate "text/template"
)

// CommonRunConfig is the template of the common run service actions
var CommonRunConfig = htemplate.Must(htemplate.New("").Parse(`#!/bin/bash
set -ex

function wait_file() {
  local src=$1
  echo "INFO: $(date): wait for $src"
  while [ ! -e $src ] ; do sleep 1; done
  echo "INFO: $(date): wait for $src completed"
  cat $src
}

function link_file() {
  local src={{ .ConfigMapMount }}/$1
  wait_file $src
  if [[ -n "$2" ]] ; then
    local ddir={{ .DstConfigPath }}
    local dst=$ddir/$2
    echo "INFO: $(date): link $src => $dst"
    mkdir -p $ddir
    ln -sf $src $dst
    md5sum $dst | awk '{print($1)}' > $dst.md5sum
  fi
}

wait_file /etc/certificates/server-key-${POD_IP}.pem
wait_file /etc/certificates/server-${POD_IP}.crt
wait_file {{ .CAFilePath }}

{{ range $src, $dst := .Configs }}
link_file {{ $src }} {{ $dst }}
{{ end }}

{{ .Command }}

`))
