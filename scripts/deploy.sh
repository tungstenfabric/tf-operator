#!/bin/bash -e

WORKSPACE=${WORKSPACE:-$HOME}

CONTRAIL_VERSION=${CONTRAIL_VERSION:-'latest'}
OPERATOR_VERSION=${OPERATOR_VERSION:-'latest'}
CONTRAIL_HA=${CONTRAIL_HA:-'1node'}

# apply crds
echo "$(date): INFO: apply CRDs"
kubectl apply -f ${WORKSPACE}/tf-operator/deploy/crds/

# apply operator
echo "$(date): INFO: apply operator"
kubectl apply -k ${WORKSPACE}/tf-operator/deploy/kustomize/operator/$OPERATOR_VERSION/
tries=0
while ! kubectl wait crds --for=condition=Established --timeout=2m managers.contrail.juniper.net ; do
  tries=$(( tries + 1 ))
  if (( 30 > $tries )) ; then
    echo "$(date): ERROR: operator resource is not ready during timeout $(( 30*2 )) seconds"
    exit 1
  fi
  sleep 2s
done

echo "$(date): INFO: apply tf"
kubectl apply -k ${WORKSPACE}/tf-operator/deploy/kustomize/contrail/$CONTRAIL_HA/$CONTRAIL_VERSION/
