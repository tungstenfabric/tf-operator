package k8s

import (
	"crypto/md5"
	"encoding/hex"

	"sigs.k8s.io/yaml"
)

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

func Md5Sum(v []byte) string {
	s := md5.Sum(v)
	return hex.EncodeToString(s[:])
}
