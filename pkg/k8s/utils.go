package k8s

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"

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

// CanNeedRetry works for errors from request for update, and returns true if
// object was changed at server side and client needs to read it again and retry update
// operation
func CanNeedRetry(err error) bool {
	regexpString := "Operation cannot be fulfilled on .*: the object has been modified; please apply your changes to the latest version and try again"
	if isMatch, _ := regexp.Match(regexpString, []byte(err.Error())); isMatch {
		return true
	}
	return false
}
