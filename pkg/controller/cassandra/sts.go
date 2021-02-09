package cassandra

import (
	"bytes"
	"text/template"

	"github.com/Juniper/contrail-operator/pkg/apis/contrail/v1alpha1"
	"github.com/ghodss/yaml"
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
      terminationGracePeriodSeconds: 10
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
      - image: cassandra:3.11.4
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
        startupProbe:
          failureThreshold: 30
          periodSeconds: 5
          exec:
            command:
            - /bin/bash
            - -c
            - "if [[ $(nodetool -p {{ .LocalJmxPort }} status | grep ${POD_IP} |awk '{print $1}') != 'UN' ]]; then exit -1; fi;"
        readinessProbe:
          initialDelaySeconds: 30
          timeoutSeconds: 3
          failureThreshold: 3
          exec:
            command:
            - /bin/bash
            - -c
            - "if [[ $(nodetool -p {{ .LocalJmxPort }} status | grep ${POD_IP} |awk '{print $1}') != 'UN' ]]; then exit -1; fi;"
        name: cassandra
        securityContext:
          capabilities:
            add:
              - IPC_LOCK
              - SYS_NICE
          privileged: false
          procMount: Default
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        #volumeMounts:
        #- mountPath: /var/log/cassandra
        #  name: cassandra-logs
        #- mountPath: /var/lib/cassandra
        #  name: cassandra-data
      - name: nodemanager
        image: tungstenfabric/contrail-nodemgr:latest
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
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/log/contrail
          name: cassandra-logs
        - mountPath: /var/crashes
          name: crashes
        - mountPath: /var/run
          name: var-run
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
        volumeMounts:
        - mountPath: /var/log/contrail
          name: cassandra-logs
        - mountPath: /var/crashes
          name: crashes
      initContainers:
      - command:
        - sh
        - -c
        - until grep ready /tmp/podinfo/pod_labels > /dev/null 2>&1; do sleep 1; done
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        image: busybox:latest
        name: init
        resources: {}
        securityContext:
          privileged: false
          procMount: Default
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /tmp/podinfo
          name: status
      volumes:
      - hostPath:
          path: /var/log/contrail/cassandra
          type: ""
        name: cassandra-logs
      - hostPath:
          path: /var/run
          type: ""
        name: var-run
      - hostPath:
          path: /var/crashes/contrail/
          type: ""
        name: crashes
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
  volumeClaimTemplates:
  - metadata:
      name: cassandra-data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 5G
  - metadata:
      name: cassandra-logs
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 5G
`))

// GetSTS returns cassandra sts object by template
func GetSTS(cassandraConfig *v1alpha1.CassandraConfiguration) *appsv1.StatefulSet {
	var buf bytes.Buffer
	yamlDatacassandraSTS.Execute(&buf, struct {
		LocalJmxPort int
	}{
		LocalJmxPort: *cassandraConfig.JmxLocalPort,
	})
	strSts := buf.String()
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(strSts), &sts)
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
