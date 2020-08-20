//
// Copyright 2020 IBM Corporation
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

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	testdata "github.com/IBM/operand-deployment-lifecycle-manager/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandConfig controller", func() {
	const (
		name              = "common-service"
		namespace         = "ibm-common-services"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		registry  *operatorv1alpha1.OperandRegistry
		config    *operatorv1alpha1.OperandConfig
		configKey types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName := createNSName(namespace)
		operatorNamespaceName := createNSName(operatorNamespace)
		registry = testdata.OperandRegistryObj(name, namespaceName, operatorNamespace)
		config = testdata.OperandConfigObj(name, namespaceName)
		configKey = types.NamespacedName{Name: name, Namespace: namespaceName}

		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(operatorNamespaceName))).Should(Succeed())

		By("Creating the OperandConfig")

		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())
	})

	AfterEach(func() {

		By("Deleting the OperandConfig")
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
	})

	Context("Initializing OperandConfig Status", func() {
		// Because OperandConfig depends on the status of InstallPlan and ClusterServiceVersion, which can't be simulated in the unit test.
		// Therefore the status of the OperandRegistry will keep in initialized.
		It("Should the status of OperandConfig be initialized", func() {

			registryInstance := &operatorv1alpha1.OperandRegistry{}
			Expect(k8sClient.Get(ctx, configKey, registryInstance)).Should(Succeed())

			By("Checking status of the OperandConfig")
			Eventually(func() operatorv1alpha1.ServicePhase {
				configInstance := &operatorv1alpha1.OperandConfig{}
				Expect(k8sClient.Get(ctx, configKey, configInstance)).Should(Succeed())
				return configInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ServiceInit))
		})
	})
})
