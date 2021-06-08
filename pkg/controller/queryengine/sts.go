package queryengine

import (
	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
)

var yamlDataQueryEngineSts = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: queryengine
spec:
  selector:
    matchLabels:
      app: queryengine
  updateStrategy:
    type: OnDelete
  template:
    metadata:
      labels:
        app: queryengine
        tf_manager: queryengine
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
        - name: queryengine
          image: tungstenfabric/contrail-analytics-query-engine:latest
          env:
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
          volumeMounts:
      volumes:
        - hostPath:
            path: /var/log/contrail/queryengine
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
          name: status`

// GetSTS returns StatesfulSet object created from yamlDataQueryEngineSts
func GetSTS() *appsv1.StatefulSet {
	sts := appsv1.StatefulSet{}
	err := yaml.Unmarshal([]byte(yamlDataQueryEngineSts), &sts)
	if err != nil {
		panic(err)
	}
	jsonData, err := yaml.YAMLToJSON([]byte(yamlDataQueryEngineSts))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal([]byte(jsonData), &sts)
	if err != nil {
		panic(err)
	}
	return &sts
}
