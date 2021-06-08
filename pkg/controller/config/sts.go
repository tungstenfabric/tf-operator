package config

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDataConfigSts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: config
spec:
  selector:
    matchLabels:
      app: config
  serviceName: "config"
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      labels:
        app: config
        tf_manager: config
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      # deviceManager pushes configuration to dnsmasq service and then needs to restart it by sending a signal.
      # Therefore those services needs to share a one process namespace
      shareProcessNamespace: true
      restartPolicy: Always
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - effect: NoExecute
          operator: Exists
      containers:
        - name: api
          image: tungstenfabric/contrail-controller-config-api:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          #startupProbe:
          #  failureThreshold: 30
          #  periodSeconds: 5
          #  httpGet:
          #    scheme: HTTPS
          #    path: /
          #    port: 8082
          #readinessProbe:
          #  failureThreshold: 3
          #  periodSeconds: 3
          #  httpGet:
          #    scheme: HTTPS
          #    path: /
          #    port: 8082
        - name: devicemanager
          image: tungstenfabric/contrail-controller-config-devicemgr:latest
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: dnsmasq
          image: tungstenfabric/contrail-external-dnsmasq:latest
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: schematransformer
          image: tungstenfabric/contrail-controller-config-schema:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: servicemonitor
          image: tungstenfabric/contrail-controller-config-svcmonitor:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: nodemanager
          image: tungstenfabric/contrail-nodemgr:latest
          securityContext:
            privileged: true
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: NODE_TYPE
              value: config
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
        - name: provisioner
          image: tungstenfabric/contrail-provisioner:latest
          env:
            - name: NODE_TYPE
              value: config
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
      initContainers:
        - name: nodeinit
          image: tungstenfabric/contrail-node-init:latest
          securityContext:
            privileged: true
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: NODE_TYPE
              value: config
            - name: POD_IP
              valueFrom:
                fieldRef:
                   fieldPath: status.podIP
            - name: INTROSPECT_SSL_ENABLE
              value: True
          volumeMounts:
            - mountPath: /host/usr/bin
              name: host-usr-bin
            - mountPath: /var/run
              name: var-run
            - mountPath: /dev
              name: dev
        - name: nodeinit-status-prefetch
          image: tungstenfabric/contrail-status:latest
        - name: nodeinit-tools-prefetch
          image: tungstenfabric/contrail-tools:latest
      volumes:
        - hostPath:
            path: /var/lib/tftp
            type: ""
          name: tftp
        - hostPath:
            path: /var/lib/dnsmasq
            type: ""
          name: dnsmasq
        - hostPath:
            path: /var/log/contrail/config
            type: ""
          name: contrail-logs
        - hostPath:
            path: /var/lib/contrail/config
            type: ""
          name: config-data
        - hostPath:
            path: /usr/bin
            type: ""
          name: host-usr-bin
        - hostPath:
            path: /dev
            type: ""
          name: dev
        - downwardAPI:
            defaultMode: 420
            items:
            - fieldRef:
                apiVersion: v1
                fieldPath: metadata.labels
              path: pod_labels
            - fieldRef:
                apiVersion: v1
                fieldPath: metadata.labels
              path: pod_labelsx
          name: status`

// GetSTS returns StatesfulSet object created from yamlDataConfigSts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDataConfigSts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDataConfigSts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
