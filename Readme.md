# Supported environments
- CentOS 7
- K8s >= 1.16 installed

# Simple AIO setup 
- Prepare VM with CentOS 7 with 32GB RAM, 4 CPUs, 64GB diskspace

- Deploy kubernetes with any preferable way, e.g. with help of kubespray (https://github.com/kubernetes-sigs/kubespray.git)

- Prepare manifests for operator and contrail, take deploy/kustomize/contrail/templates and deploy/kustomize/operator/templates as an example (or process them with jijnja parser)

- Deploy manifests, e.g.
```bash
kubectl apply -f ./deploy/crds/
kubectl wait crds --for=condition=Established --timeout=2m managers.contrail.juniper.net
kubectl apply -k ./deploy/kustomize/operator/templates/
kubectl apply -k ./deploy/kustomize/contrail/templates/
```