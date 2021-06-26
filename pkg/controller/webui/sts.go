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
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      labels:
        app: webui
        tf_manager: webui
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
        - name: webuiweb
          image: tungstenfabric/contrail-controller-webui-web:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
        - name: webuijob
          image: tungstenfabric/contrail-controller-webui-job:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
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
