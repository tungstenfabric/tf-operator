#!/bin/bash -x

WORKSPACE=${WORKSPACE:-$HOME}

cd $WORKSPACE

kubectl delete -k ./tf-operator/deploy/kustomize/contrail/1node/latest/
kubectl delete -k ./tf-operator/deploy/kustomize/operator/latest/
kubectl delete -f ./tf-operator/deploy/crds/

# TODO: implement for HA setup
kubectl delete pv  cassandra1-pv-0 zookeeper1-pv-0
sudo rm -rf \
  /mnt/cassandra \
  /mnt/zookeeper \
  /var/lib/contrail \
  /var/log/contrail \
  /var/crashes/contrail \
  /etc/cni/net.d/10-tf-cni.conf
