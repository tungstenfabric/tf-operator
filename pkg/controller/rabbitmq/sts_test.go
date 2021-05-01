package rabbitmq

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tungstenfabric/tf-operator/pkg/apis/tf/v1alpha1"
)

func TestSTSEnv(t *testing.T) {
	instance := v1alpha1.Rabbitmq{}
	sts := GetSTS(&instance)
	for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "NODE_TYPE" {
			assert.Equal(t, "config-database", env.Value)
		}
	}
}
