package controller

import (
	"github.ibm.com/IBMPrivateCloud/common-service-operator/pkg/controller/commonserviceset"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, commonserviceset.Add)
}
