package templates

import (
	"text/template"
)

// CommonRunConfig is the template of the common run service actions
var CommonRunConfig = template.Must(template.New("").Parse(`#!/bin/bash

[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x

cmd_file="/tmp/command.sh"
pid_file="${cmd_file}.pid"
sig_file="${cmd_file}.sighup"

cat <<\EOF > /tmp/command.sh
#!/bin/bash
[[ "$LOG_LEVEL" != "SYS_DEBUG" ]] || set -x
{{ .Command }}
EOF
chmod +x /tmp/command.sh

function wait_file() {
  local src=$1
  echo "INFO: $(date): wait for $src"
  while [ ! -e $src ] ; do sleep 1; done
  echo "INFO: $(date): wait for $src completed"
  local hash=$(md5sum $src | awk '{print($1)}')
  echo $hash > /tmp/$(basename $src).md5sum
  if [[ "$LOG_LEVEL" != "SYS_DEBUG" ]] ; then
    echo "INFO: $(date): hash $hash"
  else
    echo -e "INFO: $(date): hash $hash\n$(cat $src)"
  fi
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
  fi
}

function term_process() {
  local pid=$1
  local signal=TERM
  echo "INFO: $(date): $0: term_command $pid"
  if [ -n "$pid" ] ; then
    kill -${signal} $pid
    echo "INFO: $(date): $0: term_command $pid: wait child job"
    for i in {1..20}; do
      kill -0 $pid >/dev/null 2>&1 || break
      sleep 6
    done
    if kill -0 $pid >/dev/null 2>&1 ; then
      echo "INFO: $(date): $0: term_command $pid: faild to wait child job.. exit to relaunch container"
      [ -z "$sig_file" ] || rm -f $sig_file
      exit 1
    fi
  fi
}

function trap_sigterm() {
  echo "INFO: $(date): $0: trap_sigterm: start"
  local pid=$(cat $pid_file 2>/dev/null)
  term_process $pid
  echo "INFO: $(date): $0: trap_sigterm: done"
  [ -z "$sig_file" ] || rm -f $sig_file
}

function trap_sighup() {
  [ -z "$sig_file" ] || touch $sig_file
  local pid=$(cat $pid_file 2>/dev/null)
  echo "INFO: $(date): $0: trap_sighup: pid=$pid"
  kill -HUP $pid
}

function check_hash_impl() {
  local src=$1
  local new=$(md5sum $src | awk '{print($1)}')
  local old=$(cat /tmp/$(basename $src).md5sum)
  if [[ "$new" != "$old" ]] ; then
    echo "INFO: $(date): File changed $src: old=$old new=$new"
    return 1
  fi
  return 0
}

function check_hash() {
  check_hash_impl {{ .ConfigMapMount }}/$1
}

function configs_unchanged() {
  local changed=0
  {{ range $src, $dst := .Configs }}
  check_hash {{ $src }} || changed=1
  {{ end }}
  check_hash_impl /etc/certificates/server-key-${POD_IP}.pem || changed=1
  check_hash_impl /etc/certificates/server-${POD_IP}.crt || changed=1
  check_hash_impl {{ .CAFilePath }} || changed=1
  return $changed
}

{{ if .InitCommand }}
{{ .InitCommand }}
{{ end }}

export -f trap_sighup
export -f trap_sigterm
export -f wait_file
export -f link_file

update_signal={{ .UpdateSignal }}

trap 'trap_sighup' SIGHUP
trap 'trap_sigterm' SIGTERM

touch $sig_file
while [ -e $sig_file ] ; do
  wait_file /etc/certificates/server-key-${POD_IP}.pem
  wait_file /etc/certificates/server-${POD_IP}.crt
  wait_file {{ .CAFilePath }}
  {{ range $src, $dst := .Configs }}
  link_file {{ $src }} {{ $dst }}
  {{ end }}
  while [ -e $sig_file ] ; do
    pid=$(cat $pid_file 2>/dev/null)
    if [ -z "$pid" ] || ! kill -0 $pid >/dev/null 2>&1 ; then
      $cmd_file &
      pid=$!
      echo $pid > $pid_file
      echo "INFO: $(date): command started pid=$pid"
    else
      if ! configs_unchanged ; then
        delay=$(( $RANDOM % 60 ))
        echo "INFO: $(date): delay reload for $delay sec"
        sleep $delay
        if [[ "$update_signal" == 'TERM' ]] ; then 
          term_process $pid
        elif [[ "$update_signal" == 'HUP' ]] ; then
          trap_sighup
        else
          echo "INFO: $(date): unsupported signal $update_signal"
          exit 1
        fi
        break
      fi
    fi
    sleep 10
  done
done

`))
