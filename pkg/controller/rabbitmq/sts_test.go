package rabbitmq

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tungstenfabric/tf-operator/pkg/apis/contrail/v1alpha1"
)

func TestSTSWithoutCTLPort(t *testing.T) {
	instance := v1alpha1.Rabbitmq{}
	sts := GetSTS(&instance)
	for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "RABBITMQ_CTL_DIST_PORT_MIN" {
			assert.Error(t, errors.New("CTL Dist Port Min Found while it should not"))
		}
		if env.Name == "RABBITMQ_CTL_DIST_PORT_MAX" {
			assert.Error(t, errors.New("CTL Dist Port Max Found while it should not"))
		}
	}
}

func TestSTSWithCTLPort(t *testing.T) {
	minPort := 400
	maxPort := 600
	instance := v1alpha1.Rabbitmq{
		Spec: v1alpha1.RabbitmqSpec{
			ServiceConfiguration: v1alpha1.RabbitmqConfiguration{
				CTLDistPorts: &v1alpha1.CTLDistPortsConfig{
					Min: &minPort,
					Max: &maxPort,
				},
			},
		},
	}
	sts := GetSTS(&instance)
	for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "RABBITMQ_CTL_DIST_PORT_MIN" {
			assert.Equal(t, strconv.Itoa(minPort), env.Value)
		}
		if env.Name == "RABBITMQ_CTL_DIST_PORT_MAX" {
			assert.Equal(t, strconv.Itoa(maxPort), env.Value)
		}
	}
}
