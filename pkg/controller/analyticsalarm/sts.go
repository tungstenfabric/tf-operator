package analyticsalarm

import (
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

// StatefulsetYamlData is a basic yaml data for the AnalyticsAlarm statefulset.
var StatefulsetYamlData = `
apiVersion: app/v1
kind: StatefulSet
metadata:
  name: analyticsalarm
spec:
  selector:
    matchLabels:
      tf_manager: analyticsalarm
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      labels:
        tf_manager: analyticsalarm
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/master: ""
      restartPolicy: Always
      tolerations:
        - effect: NoSchedule
          operator: Exists
        - effect: NoExecute
          operator: Exists
      containers:
        - name: analytics-alarm-gen
          image: "tungstenfabric/contrail-analytics-alarm-gen:latest"
          env:
            - name: NODE_TYPE
              value: analytics-alarm
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: kafka
          image: "tungstenfabric/contrail-external-kafka:latest"
          env:
            - name: NODE_TYPE
              value: analytics-alarm
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
              value: analytics-alarm
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
              value: analytics-alarm
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
            path: /var/log/contrail/analytics-alarm
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
