#!/bin/bash -e

[ "${DEBUG,,}" == "true" ] && set -x

my_file="$(readlink -e "$0")"
my_dir="$(dirname $my_file)"
cd $my_dir/..

CONTRAIL_CONTAINER_TAG=${CONTRAIL_CONTAINER_TAG:-'latest'}
CONTRAIL_DEPLOYER_CONTAINER_TAG=${CONTRAIL_DEPLOYER_CONTAINER_TAG:-'latest'}
CONTRAIL_HA=${CONTRAIL_HA:-'1node'}

# apply crds
echo "$(date): INFO: apply CRDs"
kubectl apply -f ./deploy/crds/

# apply operator
echo "$(date): INFO: apply operator"
tries=0
while ! kubectl wait crds --for=condition=Established --timeout=2m managers.contrail.juniper.net ; do
  tries=$(( tries + 1 ))
  if (( 30 > $tries )) ; then
    echo "$(date): ERROR: operator resource is not ready during timeout $(( 30*2 )) seconds"
    exit 1
  fi
  sleep 2s
done

echo "$(date): INFO: apply tf-operator"
kubectl apply -k ./deploy/kustomize/operator/$CONTRAIL_DEPLOYER_CONTAINER_TAG/

echo "$(date): INFO: apply tf"
kubectl apply -k ./deploy/kustomize/contrail/$CONTRAIL_HA/$CONTRAIL_CONTAINER_TAG/
