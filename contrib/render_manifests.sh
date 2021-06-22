#!/usr/bin/env bash
#
# Acceptd following env variable for rendering
# CONTAINER_REGISTRY                - TF container registry (default dockerhub)
# CONTRAIL_CONTAINER_TAG            - TF container tag (default latest)
# DEPLOYER_CONTAINER_REGISTRY       - TF operator container registry (default CONTAINER_REGISTRY)
# CONTRAIL_DEPLOYER_CONTAINER_TAG   - TF operator container tag (default CONTRAIL_CONTAINER_TAG)
# CONTRAIL_REPLICAS                 - number of replicas (default calculated based on CONTROLLER_NODES
#                                     if set, otherwise 1)
# CONTROLLER_NODES                  - CONTROLLER_NODES - for integratiio with tf dev tools, it is comma
#                                     separated list of IPs of master nodes (used to calculate replicas)

DEBUG=$(echo $DEBUG | tr '[:upper:]' '[:lower:]')
[ "${DEBUG}" != "true" ] || set -x

which greadlink >/dev/null 2>&1 && rlink='greadlink' || rlink='readlink'

my_file="$($rlink -e "$0")"
my_dir="$(dirname $my_file)"

export WORKSPACE=${WORKSPACE:-${my_dir}/../..}
export OPERATOR_REPO=${OPERATOR_REPO:-$WORKSPACE/tf-operator}

function _process_manifest() {
    local folder=$1
    local templates_to_render=`ls $folder/*.j2`
    local template
    for template in $templates_to_render ; do
        local rendered_yaml=$(echo "${template%.*}")
        "$my_dir/jinja2_render.py" < $template > $rendered_yaml
    done
}

echo "Render operator templates"
if ! _process_manifest $OPERATOR_REPO/deploy/kustomize/operator/templates ; then
  echo "Failed"
  exit -1
fi

echo "Render contrail templates"
if ! _process_manifest $OPERATOR_REPO/deploy/kustomize/contrail/templates ; then
  echo "Failed"
  exit -1
fi

echo "Success"
