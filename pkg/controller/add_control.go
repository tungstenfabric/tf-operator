package controller

import (
	"github.com/tungstenfabric/tf-operator/pkg/controller/control"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, control.Add)
}
