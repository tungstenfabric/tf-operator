package cassandra

import (
	"bytes"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDatacassandraSTS = template.Must(template.New("").Parse(`
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: cassandra
spec:
  selector:
    matchLabels:
      app: cassandra
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      partition: 0
  template:
    metadata:
      labels:
        app: cassandra
        cassandra_cr: cassandra
        tf_manager: cassandra
    spec:
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      restartPolicy: Always
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      containers:
      - name: cassandra
        image: tungstenfabric/contrail-external-cassandra:latest
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: NODE_TYPE
          value: {{ .DatabaseNodeType }}
        - name: CQLSH_HOST
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: CQLSH_PORT
          value: {{ .CqlPort }}
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - nodetool -p {{ .LocalJmxPort }} drain
              #- nodetool -p {{ .LocalJmxPort }} decommission
        securityContext:
          capabilities:
            add:
              - IPC_LOCK
              - SYS_NICE
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/log/cassandra
          name: contrail-logs
        - mountPath: /var/lib/cassandra
          name: {{ .DatabaseNodeType }}-cassandra-data
      - name: nodemanager
        image: tungstenfabric/contrail-nodemgr:latest
        securityContext:
          privileged: true
        env:
        - name: NODE_TYPE
          value: {{ .DatabaseNodeType }}
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
          value: {{ .DatabaseNodeType }}
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
          path: /var/log/contrail/{{ .DatabaseNodeType }}
          type: ""
        name: contrail-logs
      - hostPath:
          path: /var/lib/contrail/{{ .DatabaseNodeType }}
          type: ""
        name: {{ .DatabaseNodeType }}-cassandra-data
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

`))

// GetSTS returns cassandra sts object by template
func GetSTS(cassandraConfig *v1alpha1.CassandraConfiguration, databaseNodeType string) *appsv1.StatefulSet {
	var buf bytes.Buffer
	err := yamlDatacassandraSTS.Execute(&buf, struct {
		LocalJmxPort     int
		DatabaseNodeType string
		CqlPort          int
	}{
		LocalJmxPort:     *cassandraConfig.JmxLocalPort,
		DatabaseNodeType: databaseNodeType,
		CqlPort:          *cassandraConfig.CqlPort,
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
