FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

MAINTAINER "Tungsten Fabric"
### Required OpenShift Labels
LABEL name="TF operator" \
      maintainer="Tungsten Fabric" \
      vendor="Tungsten Fabric" \
      version="0.0.1-2011" \
      release="0.0.1-2011" \
      summary="Tungsten Fabric SDN operator" \
      description="This operator will deploy and manage Tungsten Fabric to the cluster"

ENV OPERATOR=/usr/local/bin/tf-operator \
    USER_UID=1999 \
    USER_NAME=tf-operator

# install operator binary
COPY build/_output/bin/tf-operator ${OPERATOR}
ENV CGO_ENABLED=1

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
