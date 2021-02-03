#!/bin/bash -e

# this script install golang and contrail operator and add go bit path to your PATH

sudo yum install -y epel-release
sudo yum install -y wget gcc jq bind-utils

# golang setup
wget https://dl.google.com/go/go1.14.2.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.14.2.linux-amd64.tar.gz
rm -f go1.14.2.linux-amd64.tar.gz
cat <<EOF >> ~/.bash_profile
export PATH=\$PATH:/usr/local/go/bin
EOF
source ~/.bash_profile

curl -LO https://github.com/operator-framework/operator-sdk/releases/download/v0.17.2/operator-sdk-v0.17.2-x86_64-linux-gnu
chmod u+rx ./operator-sdk-v0.17.2-x86_64-linux-gnu
sudo mv ./operator-sdk-v0.17.2-x86_64-linux-gnu /usr/local/bin/operator-sdk

# docker setup
sudo docker run -d -p 5000:5000 --restart=always --name registry registry:2
cat <<EOF >daemon.json
{
  "insecure-registries" : ["tf-nexus.progmaticlab.com:5002"]
}
EOF
sudo mv daemon.json /etc/docker/
sudo systemctl restart docker
