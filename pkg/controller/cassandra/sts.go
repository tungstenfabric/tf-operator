package cassandra

import (
	"bytes"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1"
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
  serviceName: "cassandra"
  replicas: 1
  template:
    metadata:
      labels:
        app: cassandra
        cassandra_cr: cassandra
        contrail_manager: cassandra
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
      containers:
      - name: cassandra
        image: tungstenfabric/contrail-external-cassandra:latest
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: VENDOR_DOMAIN
          value: io.tungsten
        # TODO: move do go code for flexibility
        - name: NODE_TYPE
          value: database
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
          name: cassandra-logs
        - mountPath: /var/lib/cassandra
          name: cassandra-data
      - name: nodemanager
        image: tungstenfabric/contrail-nodemgr:latest
        securityContext:
          privileged: true
        env:
        - name: VENDOR_DOMAIN
          value: io.tungsten
        - name: NODE_TYPE
          value: database
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
          value: database
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
          path: /var/log/contrail/cassandra
          type: ""
        name: cassandra-logs
      - hostPath:
          path: /var/lib/contrail/cassandra
          type: ""
        name: cassandra-data
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
func GetSTS(cassandraConfig *v1alpha1.CassandraConfiguration) *appsv1.StatefulSet {
	var buf bytes.Buffer
	err := yamlDatacassandraSTS.Execute(&buf, struct {
		LocalJmxPort int
	}{
		LocalJmxPort: *cassandraConfig.JmxLocalPort,
	})
	if err != nil {
		panic(err)
	}
	strSts := buf.String()
	sts := appsv1.StatefulSet{}
	err = yaml.Unmarshal([]byte(strSts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(strSts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
