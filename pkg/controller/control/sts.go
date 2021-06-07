package control

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDatacontrol_sts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: control
spec:
  selector:
    matchLabels:
      app: control
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      partition: 0
  template:
    metadata:
      labels:
        app: control
        tf_manager: control
    spec:
      securityContext:
        fsGroup: 1999
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
        - name: control
          image: tungstenfabric/contrail-controller-control-control:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: dns
          image: tungstenfabric/contrail-controller-control-dns:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - mountPath: /etc/contrail
              name: etc-contrail
            - mountPath: /etc/contrail/dns
              name: etc-contrail-dns
        - name: named
          image: tungstenfabric/contrail-controller-control-named:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          securityContext:
            privileged: true
          volumeMounts:
            - mountPath: /etc/contrail
              name: etc-contrail
            - mountPath: /etc/contrail/dns
              name: etc-contrail-dns
        - name: nodemanager
          image: tungstenfabric/contrail-nodemgr:latest
          securityContext:
            privileged: true
          env:
            - name: VENDOR_DOMAIN
              value: io.tungsten
            - name: NODE_TYPE
              value: control
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
            # TODO: remove after PROVISION_HOSTNAME be supported in tf-container-builder
            - name: CONTROL_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
        - name: provisioner
          image: tungstenfabric/contrail-provisioner:latest
          env:
            - name: NODE_TYPE
              value: control
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: PROVISION_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
            # TODO: remove after tf-container-builder supports PROVISION_HOSTNAME
            - name: CONTROL_HOSTNAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.annotations['hostname']
          lifecycle:
            preStop:
              exec:
                command:
                  - python /etc/contrailconfigmaps/deprovision.py.${POD_IP}
      volumes:
        - hostPath:
            path: /var/log/contrail/control
            type: ""
          name: contrail-logs
        - hostPath:
            path: /usr/bin
            type: ""
          name: host-usr-bin
        - emptyDir: {}
          name: etc-contrail
        - emptyDir: {}
          name: etc-contrail-dns
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

// GetSTS returns StatesfulSet object created from yamlDatacontrol_sts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDatacontrol_sts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDatacontrol_sts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
