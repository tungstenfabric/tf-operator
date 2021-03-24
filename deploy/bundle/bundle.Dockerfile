FROM scratch

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=tf-operator
LABEL operators.operatorframework.io.bundle.channels.v1=R2011
LABEL operators.operatorframework.io.bundle.channel.default.v1=R2011

COPY deploy/bundle/manifests /manifests/
COPY deploy/bundle/metadata/annotations.yaml /metadata/annotations.yaml
