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
	"testing"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
)

func TestGenerateClusterObjects(t *testing.T) {
	r := &Reconciler{}

	o := &operatorv1alpha1.Operator{
		Namespace:           "test-namespace",
		TargetNamespaces:    []string{"target-namespace"},
		InstallMode:         "namespace",
		Channel:             "test-channel",
		PackageName:         "test-package",
		SourceName:          "test-source",
		SourceNamespace:     "test-source-namespace",
		InstallPlanApproval: "Automatic",
		StartingCSV:         "test-csv",
		SubscriptionConfig: &olmv1alpha1.SubscriptionConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "key",
					Value: "value",
				},
			},
		},
	}

	registryKey := types.NamespacedName{
		Namespace: "test-registry-namespace",
		Name:      "test-registry-name",
	}
	requestKey := types.NamespacedName{
		Namespace: "test-namespace",
		Name:      "test-opreq",
	}

	co := r.generateClusterObjects(o, registryKey, requestKey)

	assert.NotNil(t, co.namespace)
	assert.Equal(t, "Namespace", co.namespace.Kind)
	assert.Equal(t, "v1", co.namespace.APIVersion)
	assert.Equal(t, "test-namespace", co.namespace.Name)
	assert.Equal(t, map[string]string{"operator.ibm.com/opreq-control": "true"}, co.namespace.Labels)

	assert.NotNil(t, co.operatorGroup)
	assert.Equal(t, "OperatorGroup", co.operatorGroup.Kind)
	assert.Equal(t, "test-namespace", co.operatorGroup.Namespace)
	assert.Equal(t, []string{"target-namespace"}, co.operatorGroup.Spec.TargetNamespaces)

	assert.NotNil(t, co.subscription)
	assert.Equal(t, "Subscription", co.subscription.Kind)
	assert.Equal(t, "test-namespace", co.subscription.Namespace)
	assert.Equal(t, "test-channel", co.subscription.Spec.Channel)
	assert.Equal(t, "test-package", co.subscription.Spec.Package)
	assert.Equal(t, "test-source", co.subscription.Spec.CatalogSource)
	assert.Equal(t, "test-source-namespace", co.subscription.Spec.CatalogSourceNamespace)
	assert.Equal(t, olmv1alpha1.Approval("Automatic"), co.subscription.Spec.InstallPlanApproval)
	assert.Equal(t, "test-csv", co.subscription.Spec.StartingCSV)
	assert.NotNil(t, co.subscription.Spec.Config)
	assert.Equal(t, "value", co.subscription.Spec.Config.Env[0].Value)
}

func TestCheckSubAnnotationsForUninstall(t *testing.T) {

	reqNameA := "common-service"
	reqNsOld := "ibm-common-services"
	opNameV3 := "ibm-iam"
	opChannelV3 := "v3"

	reqNameB := "other-request"
	reqNsNew := "other-namespace"
	opNameV4 := "ibm-im"
	opChannelV4 := "v4.0"

	reqNameC := "common-service-postgresql"
	opNameC := "common-service-postgresql"
	opChannelC := "stable-v1.22"

	reqNameD := "edb-keycloak"
	opNameD := "edb-keycloak"
	opChannelD := "stable"

	// The annotation key has prefix: <requestNamespace>.<requestName>.<operatorName>/*

	// When following conditions are met, the operator should be uninstalled:
	// If all remaining <prefix>/operatorNamespace annotations' values are not the same as subscription's namespace, the operator should be uninstalled.

	// When one of following conditions are met, the operand will NOT be uninstalled:
	// 1. operator is not uninstalled AND intallMode is no-op.
	// 2. operator is uninstalled AND  at least one other <prefix>/operatorNamespace annotation exists.
	// 2. remaining <prefix>/request annotation's values contain the same operator name

	// Test case 1: uninstallOperator is true, uninstallOperand is true for uninstalling operator with v3 no-op installMode
	// The operator and operand should be uninstalled because no remaining annotation is left.
	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/request":           opChannelV3,
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/operatorNamespace": reqNsOld,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand := checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV3, operatorv1alpha1.InstallModeNoop, sub)

	assert.True(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/operatorNamespace")

	// Test case 2: uninstallOperator is true, uninstallOperand is true for uninstalling operator with general installMode
	// The operator and operand should be uninstalled because no remaining annotation is left.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV4 + "/request":           opChannelV4,
				reqNsOld + "." + reqNameA + "." + opNameV4 + "/operatorNamespace": reqNsOld,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV4, operatorv1alpha1.InstallModeNamespace, sub)

	assert.True(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV4+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV4+"/operatorNamespace")

	// Test case 3: uninstallOperator is true, uninstallOperand is false for all namespace migration
	// where operator is uninstalled and will be reinstalled in new namespace and operand is not for old operator with v3 no-op installMode
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/request":           opChannelV3,
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/operatorNamespace": reqNsOld,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/request":           opChannelV4,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/operatorNamespace": reqNsNew,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV3, operatorv1alpha1.InstallModeNoop, sub)

	assert.True(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/request")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/operatorNamespace")

	// Test case 4: uninstallOperator is true, uninstallOperand is false for all namespace migration
	// where operator is uninstalled and will be reinstalled in new namespace and operand is not for operator with general installMode
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/request":           opChannelV3,
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/operatorNamespace": reqNsOld,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/request":           opChannelV4,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/operatorNamespace": reqNsNew,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV3, operatorv1alpha1.InstallModeNamespace, sub)

	assert.True(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/request")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/operatorNamespace")

	// Test case 5: uninstallOperator is false, uninstallOperand is true for removing operand only for specific operandrequest
	// The operator should not be uninstalled because the remaining request is requesting operator is in the same namespace as the Subscription.
	// The operand should be uninstalled because this operand is NOT noop InstallMode And no other OperandRequest is requesting the same operand.
	// For example, no OperandRequest is requesting common-service-postgresql.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameC + "." + opNameC + "/request":           opChannelC,
				reqNsOld + "." + reqNameC + "." + opNameC + "/operatorNamespace": reqNsOld,
				reqNsOld + "." + reqNameD + "." + opNameD + "/request":           opChannelD,
				reqNsOld + "." + reqNameD + "." + opNameD + "/operatorNamespace": reqNsOld,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameC, reqNsOld, opNameC, operatorv1alpha1.InstallModeNamespace, sub)

	assert.False(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameC+"."+opNameC+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameC+"."+opNameC+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsOld+"."+reqNameD+"."+opNameD+"/request")
	assert.Contains(t, sub.Annotations, reqNsOld+"."+reqNameD+"."+opNameD+"/operatorNamespace")

	// Test case 6: uninstallOperator is false, uninstallOperand is false for removing one no-op operand only for upgrade scenario
	// The operator should not be uninstalled because the remaining request is requesting operator is in the same namespace as the Subscription.
	// The operand should NOT be uninstalled because this operand is noop InstallMode And other OperandRequest is requesting the newer version operand.
	// For example, IAM -> IM in-place upgrade scenario.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/request":           opChannelV3,
				reqNsOld + "." + reqNameA + "." + opNameV3 + "/operatorNamespace": reqNsOld,
				reqNsOld + "." + reqNameB + "." + opNameV4 + "/request":           opChannelV4,
				reqNsOld + "." + reqNameB + "." + opNameV4 + "/operatorNamespace": reqNsOld,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV3, operatorv1alpha1.InstallModeNoop, sub)

	assert.False(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV3+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsOld+"."+reqNameB+"."+opNameV4+"/request")
	assert.Contains(t, sub.Annotations, reqNsOld+"."+reqNameB+"."+opNameV4+"/operatorNamespace")

	// Test case 7: uninstallOperator is false, uninstallOperand is false
	// The operator and operand should not be uninstalled because at least one different OperandRequest requesting same operator
	// For example, two OperandRequests are requesting IM.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsNew,
			Annotations: map[string]string{
				reqNsNew + "." + reqNameA + "." + opNameV4 + "/request":           opChannelV4,
				reqNsNew + "." + reqNameA + "." + opNameV4 + "/operatorNamespace": reqNsNew,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/request":           opChannelV4,
				reqNsNew + "." + reqNameB + "." + opNameV4 + "/operatorNamespace": reqNsNew,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "true",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsNew, opNameV4, operatorv1alpha1.InstallModeNoop, sub)

	assert.False(t, uninstallOperator)
	assert.False(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsNew+"."+reqNameA+"."+opNameV4+"/request")
	assert.NotContains(t, sub.Annotations, reqNsNew+"."+reqNameA+"."+opNameV4+"/operatorNamespace")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/request")
	assert.Contains(t, sub.Annotations, reqNsNew+"."+reqNameB+"."+opNameV4+"/operatorNamespace")

	// Test case 8: uninstallOperator is false, uninstallOperand is true for operator with general installMode
	// The operator should be NOT uninstalled because operator is not been managed by ODLM.
	// The operand should be uninstalled because no other OperandRequest is requesting the same operand.
	sub = &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: reqNsOld,
			Annotations: map[string]string{
				reqNsOld + "." + reqNameA + "." + opNameV4 + "/request":           opChannelV4,
				reqNsOld + "." + reqNameA + "." + opNameV4 + "/operatorNamespace": reqNsOld,
			},
			Labels: map[string]string{
				constant.OpreqLabel: "false",
			},
		},
	}

	uninstallOperator, uninstallOperand = checkSubAnnotationsForUninstall(reqNameA, reqNsOld, opNameV4, operatorv1alpha1.InstallModeNamespace, sub)

	assert.False(t, uninstallOperator)
	assert.True(t, uninstallOperand)

	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV4+"/request")
	assert.NotContains(t, sub.Annotations, reqNsOld+"."+reqNameA+"."+opNameV4+"/operatorNamespace")
}
