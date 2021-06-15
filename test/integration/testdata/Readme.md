# Data for ZIU unittests are to be downloaded for a live cluster

## Prepare HA cluster

## Make script
``` bash
cat <<EOF >collect_resources.sh
#!/bin/bash -xe
version=\${1:-"master"}
target_dir="\${WORKSPACE:-.}/resources"
objects="
analyticsalarm
analyticssnmp
cassandra
config
configmaps
control
ds
kubemanager
manager
nodes
pods
rabbitmq
secrets
sts
vrouter
webui
zookeeper
"
[[ "\$version" == '2011' ]] || objects+=" analytics queryengine redis"
rm -rf \$target_dir
mkdir -p \$target_dir
cd \$target_dir
for i in \$objects ; do
  echo \$i;
  kubectl -n tf get \$i -o yaml >\$i.yaml
  sed -i -e 's/image: \([a-zA-z\.0-9-]\+:[0-9]\+\)\(.*\):.*/image: tungstenfabric\2:{{ .Tag }}/g' \$i.yaml
done
kubectl get configmap -n kube-system kubeadm-config -o yaml >kubeadm-config.yaml
EOF
chmod +x ./collect_resources.sh
```

## For Master
``` bash
./collect_resources.sh
```

## For 2011
``` bash
./collect_resources.sh 2011
```


## Download resources folder from cluster and copy into appropriate folder into testdata

## Cleanup non needed meta and data from resources
