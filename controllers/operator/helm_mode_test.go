package operator

import (
	"context"
	"testing"

	apiv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
)

func TestHelmModeSkipsOLMLookups(t *testing.T) {
	t.Setenv("NO_OLM", "true")
	// Nil clients and config ensure neither discovery nor resource calls occur.
	operator := &ODLMOperator{}
	sub, err := operator.GetSubscription(context.Background(), "test", "operators", "services", "test")
	if err != nil || sub != nil {
		t.Fatalf("GetSubscription() = %v, %v; want nil, nil", sub, err)
	}
	registry := &apiv1alpha1.OperandRegistry{
		Spec: apiv1alpha1.OperandRegistrySpec{Operators: []apiv1alpha1.Operator{{Name: "test", PackageName: "test-package"}}},
	}
	operand, err := operator.GetOperandFromRegistry(context.Background(), registry, "test")
	if err != nil || operand == nil || operand.PackageName != "test-package" {
		t.Fatalf("GetOperandFromRegistry() = %v, %v; want registry operand without OLM lookup", operand, err)
	}
}
