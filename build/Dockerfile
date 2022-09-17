FROM registry.access.redhat.com/ubi8:8.4

ARG VERSION="21.4.0"
ARG YUM_SM_PLUGIN_ENABLED=0

MAINTAINER "Tungsten Fabric"

### Required OpenShift Labels
LABEL \
    name="TF operator" \
    maintainer="Tungsten Fabric" \
    vendor="Tungsten Fabric" \
    version=$VERSION \
    release=$VERSION \
    summary="Tungsten Fabric SDN operator" \
    description="This operator will deploy and manage Tungsten Fabric to the cluster"

ENV USER_UID=1999 \
    USER_NAME=tf-operator \
    HOME=/home/tf-operator \
    CGO_ENABLED=1

RUN \
    sed -i "s/enabled=.*/enabled=$YUM_SM_PLUGIN_ENABLED/g" /etc/yum/pluginconf.d/subscription-manager.conf && \
    mkdir -p ${HOME} && \
    chown -R ${USER_UID}:0 ${HOME} && \
    chmod -R ug+rwx ${HOME} && \
    echo "${USER_NAME}:x:${USER_UID}:${USER_UID}:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd && \
    mkdir -p /licenses && \
    yum -y update-minimal --security --sec-severity=Important --sec-severity=Critical && \
    yum clean all -y && \
    rm -rf /var/cache/yum

COPY build/_output/bin/* /usr/local/bin/
COPY LICENSE /licenses/

ENTRYPOINT [ "/usr/local/bin/tf-operator" ]

USER ${USER_UID}
