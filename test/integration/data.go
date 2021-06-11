package integration

import (
	"bytes"
	"fmt"

	// "html/template"
	"os"
	"text/template"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type objectTemplate struct {
	template *template.Template
	object   runtime.Object
}

// Map of version => map of templates ( reconciler + template CR (CRDs + STS) loaded from ymal data files)
var objectTemplates map[string]map[string]objectTemplate

func addTemplate(version, name string, templ *template.Template, obj runtime.Object) {
	if objectTemplates == nil {
		objectTemplates = make(map[string]map[string]objectTemplate)
	}
	if objectTemplates[version] == nil {
		objectTemplates[version] = make(map[string]objectTemplate)
	}
	objectTemplates[version][name] = struct {
		template *template.Template
		object   runtime.Object
	}{
		template: templ,
		object:   obj,
	}
}

func joinObjectMaps(v1, v2 map[string]runtime.Object) map[string]runtime.Object {
	for k, v := range v1 {
		v2[k] = v
	}
	return v2
}
func joinStrMaps(v1, v2 map[string]string) map[string]string {
	for k, v := range v1 {
		v2[k] = v
	}
	return v2
}

var testData2011 = map[string]runtime.Object{
	"manager.yaml":        &v1alpha1.ManagerList{},
	"cassandra.yaml":      &v1alpha1.CassandraList{},
	"rabbitmq.yaml":       &v1alpha1.RabbitmqList{},
	"zookeeper.yaml":      &v1alpha1.ZookeeperList{},
	"config.yaml":         &v1alpha1.ConfigList{},
	"control.yaml":        &v1alpha1.ControlList{},
	"webui.yaml":          &v1alpha1.WebuiList{},
	"analyticsalarm.yaml": &v1alpha1.AnalyticsAlarmList{},
	"analyticssnmp.yaml":  &v1alpha1.AnalyticsSnmpList{},
	"kubemanager.yaml":    &v1alpha1.KubemanagerList{},
	"vrouter.yaml":        &v1alpha1.VrouterList{},
	"sts.yaml":            &appsv1.StatefulSetList{},
	"ds.yaml":             &appsv1.DaemonSetList{},
	"nodes.yaml":          &corev1.NodeList{},
	"secrets.yaml":        &corev1.SecretList{},
	"kubeadm-config.yaml": &corev1.ConfigMap{},
}

var testDataMaster = joinObjectMaps(testData2011, map[string]runtime.Object{
	"redis.yaml":       &v1alpha1.RedisList{},
	"analytics.yaml":   &v1alpha1.AnalyticsList{},
	"queryengine.yaml": &v1alpha1.QueryEngineList{},
})

var testDataTypes2011 = map[string]string{
	"manager.yaml":        "ManagerList",
	"cassandra.yaml":      "CassandraList",
	"rabbitmq.yaml":       "RabbitmqList",
	"zookeeper.yaml":      "ZookeeperList",
	"config.yaml":         "ConfigList",
	"control.yaml":        "ControlList",
	"webui.yaml":          "WebuiList",
	"analyticsalarm.yaml": "AnalyticsAlarmList",
	"analyticssnmp.yaml":  "AnalyticsSnmpList",
	"kubemanager.yaml":    "KubemanagerList",
	"vrouter.yaml":        "VrouterList",
	"sts.yaml":            "StatefulSetList",
	"ds.yaml":             "DaemonSetList",
	"nodes.yaml":          "NodeList",
	"secrets.yaml":        "SecretList",
	"kubeadm-config.yaml": "ConfigMap",
}
var testDataTypesMaster = joinStrMaps(testDataTypes2011, map[string]string{
	"redis.yaml":       "RedisList",
	"analytics.yaml":   "AnalyticsList",
	"queryengine.yaml": "QueryEngineList",
})

var testData = map[string]map[string]runtime.Object{
	"master": testDataMaster,
	"2011":   testData2011,
}

var testDataTypes = map[string]map[string]string{
	"master": testDataTypesMaster,
	"2011":   testDataTypes2011,
}

func init() {
	for version := range testData {
		// TODO: skip 2011 as no testdata ready
		if version == "2011" {
			continue
		}
		for fName, obj := range testData[version] {
			file, err := os.Open("testdata/" + version + "/" + fName)
			if err != nil {
				panic(err)
			}
			var buf bytes.Buffer
			if _, err = buf.ReadFrom(file); err != nil {
				panic(err)
			}
			addTemplate(version, testDataTypes[version][fName], template.Must(template.New("").Parse(buf.String())), obj)
		}
	}
}

func renderTestData(version string, objTempl objectTemplate) runtime.Object {
	obj := objTempl.object
	var err error
	var buf bytes.Buffer
	if err = objTempl.template.Execute(&buf, struct{ Tag string }{Tag: version}); err != nil {
		panic(fmt.Errorf("Failed to render %s for %s err=%+v", version, objTempl.object, err))
	}
	if err = yaml.Unmarshal(buf.Bytes(), obj); err != nil {
		panic(fmt.Errorf("Failed to Unmarshal %s for %s err=%+v", version, objTempl.object, err))
	}
	var dataJson []byte
	if dataJson, err = yaml.YAMLToJSON(buf.Bytes()); err != nil {
		panic(fmt.Errorf("Failed to YAMLToJSON %s for %s err=%+v", version, objTempl.object, err))
	}
	if err = yaml.Unmarshal(dataJson, obj); err != nil {
		panic(fmt.Errorf("Failed to Unmarshal json %s for %s err=%+v", version, objTempl.object, err))
	}
	return obj
}

// to reuse master templates for version 'new'
// (this to avoid copy of yaml files for similar versions)
var dataBindings = map[string]string{
	"new":    "master",
	"master": "master",
}

func GetTestData(version, name string) runtime.Object {
	objTempl, ok := objectTemplates[version][name]
	if !ok {
		var bv string
		if bv, ok = dataBindings[version]; ok {
			objTempl, ok = objectTemplates[bv][name]
		}
	}
	if !ok {
		panic(fmt.Errorf("Nil template for %s/%s: templates=%+v", version, name, objectTemplates))
	}
	return renderTestData(version, objTempl)
}

func GetAllTestData(version string) map[string]runtime.Object {
	dataTypes, ok := testDataTypes[version]
	if !ok {
		var bv string
		if bv, ok = dataBindings[version]; ok {
			dataTypes, ok = testDataTypes[bv]
		}
	}
	if !ok {
		panic(fmt.Errorf("No data types for version %s: testDataTypes=%+v", version, testDataTypes))
	}
	res := map[string]runtime.Object{}
	for _, name := range dataTypes {
		res[name] = GetTestData(version, name)
	}
	return res
}
