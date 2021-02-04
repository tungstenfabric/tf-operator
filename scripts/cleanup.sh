#!/bin/bash -x

[ "${DEBUG,,}" == "true" ] && set -x

my_file="$(readlink -e "$0")"
my_dir="$(dirname $my_file)"
cd $my_dir/..

kubectl delete -k ./deploy/kustomize/contrail/1node/latest/
kubectl delete -k ./deploy/kustomize/operator/latest/
kubectl delete -f ./deploy/crds/

# TODO: implement for HA setup
kubectl delete pv  cassandra1-pv-0 zookeeper1-pv-0
sudo rm -rf \
  /mnt/cassandra \
  /mnt/zookeeper \
  /var/lib/contrail \
  /var/log/contrail \
  /var/crashes/contrail \
  /etc/cni/net.d/10-tf-cni.conf
