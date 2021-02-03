# Supported environments
- CentOS 7
- K8s >= 1.16 installed

# Simple AIO setup with tf-devstack
## Prepare VM with CentOS 7 with 32GB RAM, 4 CPUs, 64GB diskspace
## Run tf-operator and AIO Tingsten fabric cluster
```bash
sudo yum install -y git
git clone https://github.com/tungstenfabric/tf-devstack.git
./tf-devstack/operator/run.sh
```

# Advanced install with building own TF operator
## Install kubernetes prepared for tf  using kubespray
```bash
sudo yum install -y git
git clone https://github.com/tungstenfabric/tf-devstack.git
./tf-devstack/operator/run.sh platform
```
## Prepare SW for build tf-operator
```bash
git clone https://github.com/tungstenfabric/tf-operator.git
./tf-operator/scripts/setup_build_software.sh
```
## Build tf-operator
```bash
./tf-operator/scripts/build.sh
```
## Run tf-operator and AIO Tingsten fabric cluster
```bash
./tf-operato/scripts/run_operator.sh
```

# To descroy AIO environemnt
./tf-operato/scripts/cleanup_aip.sh
