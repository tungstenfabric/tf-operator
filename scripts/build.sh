#!/bin/bash -e

[ "${DEBUG,,}" == "true" ] && set -x

my_file="$(readlink -e "$0")"
my_dir="$(dirname $my_file)"
cd $my_dir/..

type go >/dev/null 2>&1 || {
  export PATH=$PATH:/usr/local/go/bin
}

export CONTAINER_REGISTRY=${CONTAINER_REGISTRY:-"localhost:5000"}
export CONTRAIL_CONTAINER_TAG=${CONTRAIL_CONTAINER_TAG:-"latest"}
export CGO_ENABLED=1

target=${CONTAINER_REGISTRY}/tf-operator:${CONTRAIL_CONTAINER_TAG}

function run_cmd(){
  local me=$(whoami)
  if [[ "root" == "$me" ]] || groups | grep -q 'docker' ; then
    $@
  fi
  echo $@ | sg docker -c bash
}

run_cmd operator-sdk build $target
run_cmd docker push $target
