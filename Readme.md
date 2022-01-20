# Supported environments
- CentOS 7
- K8s >= 1.16 installed

# Simple AIO setup

## Prepare VM with CentOS 7 with 32GB RAM, 4 CPUs, 64GB diskspace

## IMPORTANT: Enable 2MB hugepages on each node (before deploying kubelet), e.g:
```bash
echo 'vm.nr_hugepages=128' | sudo tee /etc/sysctl.d/tf-hugepages.conf
sudo sysctl --system
```
### To deploy w/o hugepsages support in vrouter agent do
```bash
export HUGE_PAGES_2MB=""
```
## Deploy kubernetes with any preferable way, e.g. with help of kubespray (https://github.com/kubernetes-sigs/kubespray.git)


## Download tf-operator project.
```bash
git clone https://github.com/tungstenfabric/tf-operator
```

## Prepare TF manifests
Deployment manifests are jinja templates to be processed:
 - deploy/kustomize/contrail/templates
 - deploy/kustomize/operator/templates
### Examaples of processing
```bash
# Preparing manifests (all containers to be pulled out from dockerhub by default)
./tf-operator/contrib/render_manifests.sh
```
```bash
# Preparing manifests for TF containers to be pulled out from dockerhub and tf-operator from local registry
export CONTRAIL_DEPLOYER_CONTAINER_TAG="latest"
export DEPLOYER_CONTAINER_REGISTRY="localhost:5000"
./tf-operator/contrib/render_manifests.sh
```
```bash
# Preparing manifests for TF all containers to be pulled out from a local registry
export CONTRAIL_CONTAINER_TAG="latest"
export CONTAINER_REGISTRY="localhost:5000"
./tf-operator/contrib/render_manifests.sh
```
```bash
# Preparing manifests for TF containers to be pulled out from a custom registry and tf-operator from local one
export CONTRAIL_CONTAINER_TAG="latest"
export CONTAINER_REGISTRY="customregistry:5000"
export CONTRAIL_DEPLOYER_CONTAINER_TAG="latest"
export DEPLOYER_CONTAINER_REGISTRY="localhost:5000"
./tf-operator/contrib/render_manifests.sh
```
```bash
# Preparing manifests for TF to be used with Openshift
# it needs additionally definitions for cluster & domain names you use
export DEPLOYER="openshift"
export KUBERNETES_CLUSTER_NAME="test1"
export KUBERNETES_CLUSTER_DOMAIN="example.com"
./tf-operator/contrib/render_manifests.sh
```
```bash
# Preparing manifests for TF to use external Keystone for auth
# (keystone is not a part of tf-operator)
export AUTH_MODE="keystone"
export KEYSTONE_AUTH_HOST="<keystone address"
export KEYSTONE_AUTH_ADMIN_PASSWORD="<password>"
# or alternatively create k8s secret with keystone password and use it like
# export KEYSTONE_SECRET_NAME="<secret name>"
# all supported params are in deploy/kustomize/contrail/templates/auth.yaml.j2
./tf-operator/contrib/render_manifests.sh
```

## Set CNI MTU
```bash
export CNI_MTU=1450
# ... other options
./tf-operator/contrib/render_manifests.sh
```

## Enable L3MH
```bash
export L3MH_CIDR="100.1.1.0/42"
# ... other options
./tf-operator/contrib/render_manifests.sh
```

## Use own root CA
```bash
# generate root CA key and cert and provide base64 encoded values
export TF_ROOT_CA_CERT_BASE64=<base64 of root ca cert>
export TF_ROOT_CA_KEY_BASE64=<base64 of root ca key>
# ... other options
./tf-operator/contrib/render_manifests.sh
```

## Prepare for deploy on Ubuntu
```bash
# prepare for deploy on Ubuntu
export DISTRIBUTION=ubuntu
# ... other options
./tf-operator/contrib/render_manifests.sh
```

## Deploy TF cluster with operator
```bash
kubectl apply -f ./tf-operator/deploy/crds/
kubectl wait crds --for=condition=Established --timeout=2m managers.tf.tungsten.io
kubectl apply -k ./tf-operator/deploy/kustomize/operator/templates/
kubectl apply -k ./tf-operator/deploy/kustomize/contrail/templates/
```


# Building tf-operator

## Prepare build tools
```bash
# install docker
sudo yum -y install epel-release wget gcc yum-utils git
sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
sudo yum install -y docker-ce docker-ce-cli containerd.io
sudo docker run -d -p 5000:5000 --restart=always --name registry registry:2
cat <<EOF | sudo tee /etc/docker/daemon.json
{
  "insecure-registries" : [ "localhost:5000" ]
}
EOF
sudo systemctl restart docker

# download go and sdk tool
wget -nv https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.14.2.linux-amd64.tar.gz
rm -f go1.14.2.linux-amd64.tar.gz
sudo wget -nv https://github.com/operator-framework/operator-sdk/releases/download/v0.18.2/operator-sdk-v0.18.2-x86_64-linux-gnu -O /usr/local/bin/operator-sdk
sudo chmod 755 /usr/local/bin/operator-sdk

# for macos:
# wget -nv https://github.com/operator-framework/operator-sdk/releases/download/v0.18.2/operator-sdk-v0.18.2-x86_64-apple-darwin -O /usr/local/bin/operator-sdk

#update user variables
echo export PATH=$PATH:/usr/local/go/bin >> $HOME/.bashrc
echo export CGO_ENABLED=1 >> $HOME/.bashrc
sudo usermod -aG docker $(whoami)

# logout and relogin back to usermod takes effect
exit
# login back
```

## Download tf-operator project
```bash
git clone https://github.com/tungstenfabric/tf-operator
```

## Build operator and push into the local registry
```bash
cd tf-operator
target=localhost:5000/tf-operator:latest
operator-sdk build $target
docker push $target
```

# Documentation
[Tungsten Fabric Operator official documentation](https://docs.tungsten.io/en/latest/tungsten-fabric-operator/index.html)


# [Generate operator bundle](deploy/bundle/Readme.md)

# [Prepare testdata for integration unittests](test/integration/testdata/Readme.md)
