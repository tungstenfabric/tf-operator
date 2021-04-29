#How to test is tf-operator deploys correctly, when it packaged for the Operator Lifecycle Manager?

1. Create a directory that simulates the olm catalog containing a single `tf-operator` version `1.1.0`:
```bash
# Create file structure
mkdir my-operators
mkdir my-operators/upstream-community-operators
mkdir my-operators/upstream-community-operators/tf-operator
mkdir my-operators/upstream-community-operators/tf-operator/1.1.0

# Add standart dockerfile for catalog
cat > my-operators/catalog.Dockerfile <<EOF
FROM quay.io/operator-framework/upstream-registry-builder as builder

COPY upstream-community-operators manifests
RUN /bin/initializer --permissive -o ./bundles.db

FROM scratch
COPY --from=builder /etc/nsswitch.conf /etc/nsswitch.conf
COPY --from=builder /bundles.db /bundles.db
COPY --from=builder /bin/registry-server /registry-server
COPY --from=builder /bin/grpc_health_probe /bin/grpc_health_probe

EXPOSE 50051
ENTRYPOINT ["/registry-server"]
CMD ["--database", "bundles.db"]
EOF

# Copy bundle to catalog
cp -r deploy/bundle/manifests my-operators/upstream-community-operators/tf-operator/1.1.0/manifests
cp -r deploy/bundle/metadata my-operators/upstream-community-operators/tf-operator/1.1.0/metadata
cp deploy/bundle/bundle.Dockerfile my-operators/upstream-community-operators/tf-operator/1.1.0/bundle.Dockerfile

# Manualy change operator name and operator version in 
# my-operators/upstream-community-operators/tf-operator/1.1.0/manifests/tf-operator.clusterserviceversion.yaml:
# tf-operator.latest  -->  tf-operator.v1.1.0
# version: 0.0.0      -->  version: 1.1.0

# Add packagemanifest and sericeaccount
cat > my-operators/upstream-community-operators/tf-operator/tf-operator.package.yaml <<EOF
channels:
- currentCSV: tf-operator.v1.1.0
  name: latest
defaultChannel: latest
packageName: tf-operator
EOF

cat > my-operators/upstream-community-operators/tf-operator/1.1.0/manifests/tf-operator_v1_serviceaccount.yaml <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: tf-operator
EOF
```

2. Prepare local registry and build the catalog image
``` bash
# Prepare docker
sudo docker run -d -p 5000:5000 --restart=always --name registry registry:2
cat <<EOF | sudo tee /etc/docker/daemon.json
{
    "insecure-registries" : [ "localhost:5000" ]
}
EOF
sudo systemctl restart docker

# Build catalog image
cd my-operators
sudo docker build -f catalog.Dockerfile -t my-operators:latest .
sudo docker tag my-operators:latest localhost:5000/my-operators:latest
sudo docker push localhost:5000/my-operators:latest
```

3. Install OLM
``` bash
curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/install.sh -o install.sh
chmod +x install.sh
./install.sh v0.17.0
```

4. Apply to kubernetes catalogsource, subscription and operator group
``` bash
cat > catalog-source.yaml <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: my-operators
  namespace: olm
spec:
  sourceType: grpc
  image: localhost:5000/my-operators:latest
EOF
kubectl apply -f catalog-source.yaml

cat > operator-subscription.yaml <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: tf-operator-subscription
  namespace: tf
spec:
  channel: latest
  name: tf-operator
  startingCSV: tf-operator.v1.1.0
  source: my-operators
  sourceNamespace: olm
EOF
kubectl apply -f operator-subscription.yaml

cat > operator-group <<EOF
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: tf-operatorgroup
  namespace: tf
spec:
  targetNamespaces:
  - tf
EOF
kubectl apply -f operator-group.yaml
```

## What we expected to see?
- After actions above catalog, subscription and operator group must be created, and we can see `tf-operator` in the list of package manifests:
``` bash
kubectl get packagemanifests | grep tf-operator
```
- If `tf-operator` is already installed on the machine with non `latest` image tag, then the pod will be recreated with image tag `latest`.