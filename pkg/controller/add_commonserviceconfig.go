package controller

import (
	"github.ibm.com/IBMPrivateCloud/common-service-operator/pkg/controller/commonserviceconfig"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, commonserviceconfig.Add)
}
