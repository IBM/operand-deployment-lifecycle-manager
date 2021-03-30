//
// Copyright 2021 IBM Corporation
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

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("NamespaceScope controller", func() {
	const (
		namespace        = "ibm-operators"
		requestName      = "ibm-cloudpak-name"
		requestNamespace = "ibm-cloudpak"
		registryName     = "common-service"
	)

	var (
		ctx context.Context

		namespaceName        string
		requestNamespaceName string
		registry             *operatorv1alpha1.OperandRegistry
		request              *operatorv1alpha1.OperandRequest
		config               *operatorv1alpha1.OperandConfig
		nss                  *nssv1.NamespaceScope
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = createNSName(namespace)
		requestNamespaceName = createNSName(requestNamespace)
		registry = testutil.OperandRegistryObj(registryName, requestNamespaceName, namespaceName)
		config = testutil.OperandConfigObj(registryName, requestNamespaceName)
		request = testutil.OperandRequestObj(registryName, requestNamespaceName, requestName, requestNamespaceName)
		nss = testutil.NamespaceScopeObj(namespace)

		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(namespace))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(requestNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, nss)).Should(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, nss)).Should(Succeed())
	})

	Context("Add OperandRequest Namespace into NamespaceScope CR", func() {
		It("Should NamespaceScope CR have two namespaces", func() {
			By("Create OperandRequest instance")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())
			By("Create OperandRegistry instance")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			Eventually(func() int {
				reg := &operatorv1alpha1.OperandRegistry{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: registryName, Namespace: requestNamespaceName}, reg)
				if err != nil {
					return -1
				}
				return len(reg.Status.OperatorsStatus)
			}, timeout, interval).Should(Equal(2))
			By("Check the namespace number in the NamespaceScope CR")
			Eventually(func() int {
				nss := &nssv1.NamespaceScope{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: constant.NamespaceScopeCrName, Namespace: namespace}, nss)
				if err != nil {
					return -1
				}
				return len(nss.Spec.NamespaceMembers)
			}, timeout, interval).Should(Equal(3))
			By("Delete OperandRequest instance")
			Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
			By("Check the namespace number in the NamespaceScope CR")
			Eventually(func() int {
				nss := &nssv1.NamespaceScope{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: constant.NamespaceScopeCrName, Namespace: namespace}, nss)
				if err != nil {
					return -1
				}
				return len(nss.Spec.NamespaceMembers)
			}, timeout, interval).Should(Equal(1))
		})
	})
})
