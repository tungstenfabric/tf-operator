package zookeeper

import (
	"bytes"
	"text/template"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDatazookeeper_sts = template.Must(template.New("").Parse(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: zookeeper
spec:
  selector:
    matchLabels:
      app: zookeeper
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      partition: 0
  template:
    metadata:
      labels:
        app: zookeeper
        zookeeper_cr: zookeeper
        tf_manager: zookeeper
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      restartPolicy: Always
      nodeSelector:
        node-role.kubernetes.io/master: ''
      tolerations:
      - operator: Exists
        effect: NoSchedule
      - operator: Exists
        effect: NoExecute
      containers:
      - name: zookeeper
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NODE_TYPE
          value: {{ .DatabaseNodeType }}
        image: tungstenfabric/contrail-external-zookeeper:latest
        startupProbe:
          periodSeconds: 3
          failureThreshold: 30
          exec:
            command:
            - /bin/bash
            - -c
            - "OK=$(echo ruok | nc ${POD_IP} 2181); if [[ ${OK} == \"imok\" ]]; then exit 0; else exit 1;fi"
        readinessProbe:
          initialDelaySeconds: 30
          timeoutSeconds: 3
          failureThreshold: 3
          exec:
            command:
            - /bin/bash
            - -c
            - "OK=$(echo ruok | nc ${POD_IP} 2181); if [[ ${OK} == \"imok\" ]]; then exit 0; else exit 1;fi"
        volumeMounts:
        - mountPath: /tmp/conf
          name: conf
        - mountPath: /var/lib/zookeeper
          name: zookeeper-data
        - mountPath: /var/log/zookeeper
          name: zookeeper-logs
      volumes:
      - name: zookeeper-data
        hostPath:
          path: /var/lib/contrail/zookeeper
      - name: zookeeper-logs
        hostPath:
          path: /var/log/contrail/zookeeper
      - downwardAPI:
          defaultMode: 420
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.labels
            path: pod_labels
        name: status
      - downwardAPI:
          defaultMode: 420
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.labels
            path: zoo.cfg
        name: conf

`))

func GetSTS(databaseNodeType string) *appsv1.StatefulSet {
	var buf bytes.Buffer
	err := yamlDatazookeeper_sts.Execute(&buf, struct {
		DatabaseNodeType string
	}{
		DatabaseNodeType: databaseNodeType,
	})
	if err != nil {
		panic(err)
	}
	sts := appsv1.StatefulSet{}
	err = yaml.Unmarshal(buf.Bytes(), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
