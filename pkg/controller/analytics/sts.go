package analytics

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDataAnalyticsSts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: analytics
spec:
  selector:
    matchLabels:
      app: analytics
  serviceName: "analytics"
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      labels:
        app: analytics
        tf_manager: analytics
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      # nodemanager pidhost and
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
        - name: analyticsapi
          image: tungstenfabric/contrail-analytics-api:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: ANALYTICSDB_ENABLE
              value: "true"
            - name: ANALYTICS_ALARM_ENABLE
              value: "true"
        - name: collector
          image: tungstenfabric/contrail-analytics-collector:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          securityContext:
            capabilities:
              add:
                - SYS_PTRACE
        - name: nodemanager
          image: tungstenfabric/contrail-nodemgr:latest
          securityContext:
            privileged: true
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: NODE_TYPE
              value: analytics
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
              value: analytics
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
          volumeMounts:
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
            path: /var/log/contrail/analytics
            type: ""
          name: contrail-logs
        - hostPath:
            path: /var/lib/contrail/analytics
            type: ""
          name: analytics-data
        - hostPath:
            path: /usr/bin
            type: ""
          name: host-usr-bin
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

// GetSTS returns StatesfulSet object created from yamlDataAnalyticsSts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDataAnalyticsSts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDataAnalyticsSts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
