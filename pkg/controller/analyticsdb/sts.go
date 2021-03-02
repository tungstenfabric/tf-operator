package analyticsdb

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDataAnalyticsDBSts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: analyticsdb
spec:
  selector:
    matchLabels:
      app: analyticsdb
  serviceName: "analyticsdb"
  replicas: 1
  template:
    metadata:
      labels:
        app: analyticsdb
        contrail_manager: analyticsdb
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
        - name: queryengine
          image: tungstenfabric/contrail-analytics-query-engine:latest
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
            path: /var/log/contrail/analyticsdb
            type: ""
          name: contrail-logs
        - hostPath:
            path: /var/lib/contrail/analyticsdb
            type: ""
          name: analyticsdb-data
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

// GetSTS returns StatesfulSet object created from yamlDataAnalyticsDBSts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDataAnalyticsDBSts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDataAnalyticsDBSts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
