package k8s

import "sigs.k8s.io/yaml"

func YamlToStruct(yamlString string, structPointer interface{}) error {
	jsonData, err := yaml.YAMLToJSON([]byte(yamlString))
	if err != nil {
		return err
	}

	if err = yaml.Unmarshal([]byte(jsonData), structPointer); err != nil {
		return err
	}

	return nil
}
