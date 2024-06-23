//
// Copyright 2022 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package operandrequest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	odlm "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
)

var (
	isolatedNs        = "cp4i"
	operatorNamespace = "ibm-common-services"
	opreqName         = "example-service"

	specInIsolatedNs = odlm.OperandRequestSpec{
		Requests: []odlm.Request{
			{
				Registry:          "common-service",
				RegistryNamespace: isolatedNs,
				Operands:          []odlm.Operand{{Name: "ibm-iam-operator"}},
			},
			{
				Registry:          "standalone-registry",
				RegistryNamespace: isolatedNs,
				Operands:          []odlm.Operand{{Name: "ibm-zen-operator"}},
			},
			{
				Registry:          "common-service",
				RegistryNamespace: isolatedNs,
				Operands:          []odlm.Operand{{Name: "keycloak-operator"}, {Name: "ibm-zen-operator"}},
			},
		},
	}

	opreqInIsolatedNs = &odlm.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: isolatedNs,
			Name:      opreqName,
			Annotations: map[string]string{
				"operand.ibm.com/watched-by-odlm-in-" + operatorNamespace: "2024-06-22T23:51:36Z",
			},
		},
		Spec: specInIsolatedNs,
	}

	registryInOperatorNs = &odlm.OperandRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "common-service",
			Namespace: operatorNamespace,
		},
		Spec: odlm.OperandRegistrySpec{
			Operators: []odlm.Operator{
				{
					Name: "ibm-zen-operator",
				},
				{
					Name: "ibm-iam-operator",
				},
			},
		},
	}

	opreqSpecInOperatorNs = &odlm.OperandRequestSpec{
		Requests: []odlm.Request{
			{
				Registry:          "common-service",
				RegistryNamespace: operatorNamespace,
				Operands:          []odlm.Operand{{Name: "ibm-iam-operator"}},
			},
			{
				Registry:          "common-service",
				RegistryNamespace: operatorNamespace,
				Operands:          []odlm.Operand{{Name: "ibm-zen-operator"}},
			},
		},
	}
)

func TestGetNewOpreqSpec(t *testing.T) {
	ctx := context.TODO()

	scheme := runtime.NewScheme()
	utilruntime.Must(odlm.AddToScheme(scheme))
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()

	opreq := opreqInIsolatedNs.DeepCopy()

	registry := registryInOperatorNs.DeepCopy()

	err := kube.Create(ctx, registry)
	assert.NoError(t, err)

	newOpreqSpec, err := GetNewOpreqSpec(ctx, kube, opreq, operatorNamespace)
	assert.NoError(t, err)

	expectedOpreqSpec := opreqSpecInOperatorNs.DeepCopy()
	assert.Equal(t, expectedOpreqSpec, newOpreqSpec)
}
func TestHandle(t *testing.T) {
	ctx := context.TODO()

	scheme := runtime.NewScheme()
	utilruntime.Must(odlm.AddToScheme(scheme))
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()

	defaulter := &Defaulter{
		Client:     kube,
		OperatorNs: operatorNamespace,
	}

	// Create an OperandRequest in the isolated namespace
	opreq := opreqInIsolatedNs.DeepCopy()
	rawOpreq, err := json.Marshal(opreq)
	assert.NoError(t, err)

	// Create an existing OperandRequest in the operatorNamespace
	existingOpreq := &odlm.OperandRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: operatorNamespace,
			Name:      opreqName + "-from-" + isolatedNs,
		},
		Spec: odlm.OperandRequestSpec{
			Requests: []odlm.Request{
				{
					Registry:          "common-service",
					RegistryNamespace: operatorNamespace,
					Operands:          []odlm.Operand{{Name: "ibm-zen-operator"}},
				},
			},
		},
	}
	err = kube.Create(ctx, existingOpreq)
	assert.NoError(t, err)

	registry := registryInOperatorNs.DeepCopy()

	err = kube.Create(ctx, registry)
	assert.NoError(t, err)

	// Create an admission.Request
	admissionReq := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Namespace: isolatedNs,
			Name:      opreqName,
			Operation: admissionv1.Update,
			Object: runtime.RawExtension{
				Raw: rawOpreq,
			},
		},
	}

	decoder, err := admission.NewDecoder(kube.Scheme())
	assert.NoError(t, err)

	err = defaulter.InjectDecoder(decoder)
	assert.NoError(t, err)

	resp := defaulter.Handle(ctx, admissionReq)
	assert.True(t, resp.Allowed)

	// Assert the existing OperandRequest is updated
	err = kube.Get(ctx, types.NamespacedName{Name: existingOpreq.Name, Namespace: existingOpreq.Namespace}, existingOpreq)
	assert.NoError(t, err)

	expectedOpreqSpec := opreqSpecInOperatorNs.DeepCopy()
	assert.Equal(t, *expectedOpreqSpec, existingOpreq.Spec)

	// Remove the existing OperandRequest
	err = kube.Delete(ctx, existingOpreq)
	assert.NoError(t, err)

	resp = defaulter.Handle(ctx, admissionReq)
	assert.True(t, resp.Allowed)

	// Assert a new OperandRequest is created
	newOpreq := &odlm.OperandRequest{}
	err = kube.Get(ctx, types.NamespacedName{Name: opreqName + "-from-" + isolatedNs, Namespace: operatorNamespace}, newOpreq)
	assert.NoError(t, err)

	assert.Equal(t, *expectedOpreqSpec, newOpreq.Spec)

	// Call the Handle function with an delete operation
	admissionReq.Operation = admissionv1.Delete
	resp = defaulter.Handle(ctx, admissionReq)
	assert.True(t, resp.Allowed)

	// Assert the new OperandRequest is deleted
	err = kube.Get(ctx, types.NamespacedName{Name: newOpreq.Name, Namespace: newOpreq.Namespace}, newOpreq)
	assert.Error(t, err)
}
