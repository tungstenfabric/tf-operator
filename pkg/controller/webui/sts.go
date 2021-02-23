package webui

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDatawebui_sts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: webui
spec:
  selector:
    matchLabels:
      app: webui
  serviceName: "webui"
  replicas: 1
  template:
    metadata:
      labels:
        app: webui
        contrail_manager: webui
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
      initContainers:
        - name: init
          image: busybox:latest
          command:
            - sh
            - -c
            - until grep ready /tmp/podinfo/pod_labels > /dev/null 2>&1; do sleep 1; done
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - mountPath: /tmp/podinfo
              name: status
      containers:
        - name: webuiweb
          image: tungstenfabric/contrail-controller-webui-web:latest
          env:
            - name: WEBUI_SSL_KEY_FILE
              value: /etc/contrail/webui_ssl/cs-key.pem
            - name: WEBUI_SSL_CERT_FILE
              value: /etc/contrail/webui_ssl/cs-cert.pem
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            # TODO: xxx_ENABLE below must be configurable
            - name: ANALYTICSDB_ENABLE
              value: "true"
            - name: ANALYTICS_SNMP_ENABLE
              value: false
            - name: ANALYTICS_ALARM_ENABLE
              value: false
        - name: webuijob
          image: tungstenfabric/contrail-controller-webui-job:latest
          env:
            - name: WEBUI_SSL_KEY_FILE
              value: /etc/contrail/webui_ssl/cs-key.pem
            - name: WEBUI_SSL_CERT_FILE
              value: /etc/contrail/webui_ssl/cs-cert.pem
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: redis
          image: tungstenfabric/contrail-external-redis:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
            - mountPath: /var/lib/redis
              name: webui-data
      volumes:
        - hostPath:
            path: /var/lib/contrail/webui
            type: ""
          name: webui-data
        - hostPath:
            path: /var/log/contrail/webui
            type: ""
          name: contrail-logs
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

// GetSTS returns StatesfulSet object created from YAML yamlDatawebui_sts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDatawebui_sts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDatawebui_sts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
