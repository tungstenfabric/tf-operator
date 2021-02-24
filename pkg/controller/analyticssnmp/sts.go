package analyticssnmp

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

// StatefulsetYamlData is a basic yaml data for the AnalyticsSnmp statefulset.
var StatefulsetYamlData = `
apiVersion: app/v1
kind: StatefulSet
metadata:
  name: analyticssnmp
spec:
  selector:
    matchLabels:
      contrail_manager: analyticssnmp
  serviceName: "analyticssnmp"
  replicas: 1
  template:
    metadata:
      labels:
        contrail_manager: analyticssnmp
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      # nodemanager
      shareProcessNamespace: true
      restartPolicy: Always
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - effect: NoExecute
          operator: Exists
      initContainers:
        - name: init
          image: busybox:latest
          command:
            - sh
            - -c
            - until grep ready /tmp/podinfo/pod_labels > dev/null 2>&1; do sleep 1; done
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - mountPath: /tmp/podinfo
              name: status
      containers:
        - name: analytics-snmp-collector
          image: "tungstenfabric/contrail-analytics-snmp-collector:latest"
          env:
            - name: NODE_TYPE
              value: analytics-snmp
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: analytics-snmp-topology
          image: "tungstenfabric/contrail-analytics-snmp-topology:latest"
          env:
            - name: NODE_TYPE
              value: analytics-snmp
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: nodemanager
          image: "tungstenfabric/contrail-nodemgr:latest"
          securityContext:
            privileged: true
          env:
            - name: NODE_TYPE
              value: analytics-snmp
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
        - name: provisioner
          image: "tungstenfabric/contrail-provisioner:latest"
          env:
            - name: NODE_TYPE
              value: analytics-snmp
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
      volumes:
        - hostPath:
            path: /var/log/contrail/analytics-snmp
            type: ""
          name: contrail-logs
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
          name: status
`

// GetStatefulsetFromYaml returns basic StatesfulSet created from StatefulsetYamlData
func GetStatefulsetFromYaml() (*appsv1.StatefulSet, error) {
	sts := appsv1.StatefulSet{}
	if err := yaml.Unmarshal([]byte(StatefulsetYamlData), &sts); err != nil {
		return nil, err
	}

	jsonData, err := yaml.YAMLToJSON([]byte(StatefulsetYamlData))
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal([]byte(jsonData), &sts); err != nil {
		return nil, err
	}

	return &sts, nil
}
