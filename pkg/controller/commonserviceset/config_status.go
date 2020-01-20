package commonserviceset

import (
	"context"

	operatorv1alpha1 "github.ibm.com/IBMPrivateCloud/common-service-operator/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileCommonServiceSet) updateServiceStatus(cr *operatorv1alpha1.CommonServiceConfig, operatorName, serviceName string, serviceStatus operatorv1alpha1.ServicePhase) error {

	if cr.Status.ServiceStatus == nil {
		cr.Status.ServiceStatus = make(map[string]*operatorv1alpha1.CrStatus)
	}

	_, ok := cr.Status.ServiceStatus[operatorName]
	if !ok {
		cr.Status.ServiceStatus[operatorName] = &operatorv1alpha1.CrStatus{}
	}

	if cr.Status.ServiceStatus[operatorName].CrStatus == nil {
		cr.Status.ServiceStatus[operatorName].CrStatus = make(map[string]operatorv1alpha1.ServicePhase)
	}

	cr.Status.ServiceStatus[operatorName].CrStatus[serviceName] = serviceStatus
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}
