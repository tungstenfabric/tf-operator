# Supported environments
- CentOS 7
- K8s >= 1.16 installed

# Simple AIO setup 
- Prepare VM with CentOS 7 with 32GB RAM, 4 CPUs, 64GB diskspace

- Deploy kubernetes with any preferable way, e.g. with help of kubespray (https://github.com/kubernetes-sigs/kubespray.git)

- Prepare manifests for operator and contrail, take deploy/kustomize/contrail/templates and deploy/kustomize/operator/templates as an example (or process them with jijnja parser)

- Deploy manifests, e.g.
```bash
git clone https://github.com/tungstenfabric/tf-operator
cd tf-operator
kubectl apply -f ./deploy/crds/
kubectl wait crds --for=condition=Established --timeout=2m managers.contrail.juniper.net
kubectl apply -k ./deploy/kustomize/operator/templates/
kubectl apply -k ./deploy/kustomize/contrail/templates/
```

# Building tf-operator

- Prepare build tools
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
sudo wget  -nv https://github.com/operator-framework/operator-sdk/releases/download/v0.17.2/operator-sdk-v0.17.2-x86_64-linux-gnu -O /usr/local/bin/operator-sdk
sudo chmod 755 /usr/local/bin/operator-sdk

#update user variables
echo export PATH=$PATH:/usr/local/go/bin >> $HOME/.bashrc
echo export CGO_ENABLED=1 >> $HOME/.bashrc
sudo usermod -aG docker $(whoami)

# relogin to usermod takes effect
```

- Build
```bash
git clone https://github.com/tungstenfabric/tf-operator
cd tf-operator
target=localhost:5000/tf-operator:latest
operator-sdk build $target
docker push $target
```

# Documentation
[Tungsten Fabric Operator official documentation](https://docs.tungsten.io/en/latest/tungsten-fabric-operator/index.html)
