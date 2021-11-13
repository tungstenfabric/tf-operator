FROM registry.access.redhat.com/ubi8:8.4

MAINTAINER "Tungsten Fabric"
### Required OpenShift Labels
LABEL name="TF operator" \
      maintainer="Tungsten Fabric" \
      vendor="Tungsten Fabric" \
      version="1.0.0-master" \
      release="1.0.0-master" \
      summary="Tungsten Fabric SDN operator" \
      description="This operator will deploy and manage Tungsten Fabric to the cluster"

ENV OPERATOR=/usr/local/bin/tf-operator \
    USER_UID=1999 \
    USER_NAME=tf-operator

RUN mkdir -p /licenses
COPY LICENSE /licenses/

# install operator binary
COPY build/_output/bin/tf-operator ${OPERATOR}
ENV CGO_ENABLED=1

COPY build/bin /usr/local/bin
RUN /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
