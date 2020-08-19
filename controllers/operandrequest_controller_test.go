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

var _ = Describe("OperandRegistry controller", func() {
	const (
		name              = "ibm-cloudpak-name"
		namespace         = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-service"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		registry   *operatorv1alpha1.OperandRegistry
		config     *operatorv1alpha1.OperandConfig
		request    *operatorv1alpha1.OperandRequest
		requestKey types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName := createNSName(namespace)
		registryNamespaceName := createNSName(registryNamespace)
		operatorNamespaceName := createNSName(operatorNamespace)
		registry = testdata.OperandRegistryObj(registryName, registryNamespaceName, operatorNamespaceName)
		config = testdata.OperandConfigObj(registryName, registryNamespaceName)
		request = testdata.OperandRequestObj(registryName, registryNamespaceName, name, namespaceName)
		requestKey = types.NamespacedName{Name: name, Namespace: namespaceName}

		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(registryNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(operatorNamespaceName))).Should(Succeed())

		By("Creating the OperandRequest")

		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())
		Expect(k8sClient.Create(ctx, request)).Should(Succeed())

	})

	AfterEach(func() {

		By("Deleting the OperandRequest")
		Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
	})

	Context("Initializing OperandRequest Status", func() {
		// Because OperandRequest depends on the status of InstallPlan and ClusterServiceVersion, which can't be simulated in the unit test.
		// Therefore the status of the OperandRequest will keep in installing.
		It("Should status of OperandRequest be installing", func() {

			By("Checking status of the OperandRegquest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey, requestInstance)).Should(Succeed())
				return requestInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))
		})
	})
})
