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

package operandregistry

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandRegistry controller", func() {
	const (
		name              = "common-service"
		namespace         = "ibm-common-services"
		requestName       = "ibm-cloudpak-name"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		namespaceName         string
		operatorNamespaceName string
		requestNamespaceName  string
		registry              *operatorv1alpha1.OperandRegistry
		config                *operatorv1alpha1.OperandConfig
		request               *operatorv1alpha1.OperandRequest
		catalogSource         *olmv1alpha1.CatalogSource
		registryKey           types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = testutil.CreateNSName(namespace)
		operatorNamespaceName = testutil.CreateNSName(operatorNamespace)
		requestNamespaceName = testutil.CreateNSName(namespace)
		registry = testutil.OperandRegistryObj(name, namespaceName, operatorNamespaceName)
		config = testutil.OperandConfigObj(name, namespaceName)
		request = testutil.OperandRequestObj(name, namespaceName, requestName, requestNamespaceName)
		catalogSource = testutil.CatalogSource("community-operators", "openshift-marketplace")
		registryKey = types.NamespacedName{Name: name, Namespace: namespaceName}

		By("Creating the Namespace")
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(operatorNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(requestNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj("openshift-marketplace")))

		By("Creating the CatalogSource")
		Expect(k8sClient.Create(ctx, catalogSource)).Should(Succeed())
		catalogSource.Status = testutil.CatalogSourceStatus()
		Expect(k8sClient.Status().Update(ctx, catalogSource)).Should(Succeed())
		By("Creating the OperandRegistry")
		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		By("Creating the OperandConfig")
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())
	})

	AfterEach(func() {
		By("Deleting the CatalogSource")
		Expect(k8sClient.Delete(ctx, catalogSource)).Should(Succeed())
		By("Deleting the OperandRequest")
		Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
		By("Deleting the OperandConfig")
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		By("Deleting the OperandRegistry")
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())

	})

	Context("Initializing OperandRegistry Status", func() {

		It("Should status of OperandRegistry be waiting for CatalogSource", func() {

			By("Checking status of the OperandRegistry")
			Eventually(func() operatorv1alpha1.RegistryPhase {
				registryInstance := &operatorv1alpha1.OperandRegistry{}
				Expect(k8sClient.Get(ctx, registryKey, registryInstance)).Should(Succeed())
				return registryInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.RegistryReady))

			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Setting status of the Subscriptions")
			jaegerSub := testutil.Subscription("jaeger", operatorNamespaceName)
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)
				jaegerSub.Status = testutil.SubscriptionStatus("jaeger", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jaegerSub)
			}, timeout, interval).Should(Succeed())

			mongodbSub := testutil.Subscription("mongodb-atlas-kubernetes", operatorNamespaceName)
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes", Namespace: operatorNamespaceName}, mongodbSub)
				mongodbSub.Status = testutil.SubscriptionStatus("mongodb-atlas-kubernetes", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, mongodbSub)
			}, timeout, interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			jaegerCSV := testutil.ClusterServiceVersion("jaeger-csv.v0.0.1", "jaeger", operatorNamespaceName, testutil.JaegerExample)
			Expect(k8sClient.Create(ctx, jaegerCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				jaegerCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jaegerCSV)
			}, timeout, interval).Should(Succeed())

			mongodbCSV := testutil.ClusterServiceVersion("mongodb-atlas-kubernetes-csv.v0.0.1", "mongodb-atlas-kubernetes", operatorNamespaceName, testutil.MongodbExample)
			Expect(k8sClient.Create(ctx, mongodbCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				mongodbCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, mongodbCSV)
			}, timeout, interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			jaegerIP := testutil.InstallPlan("jaeger-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jaegerIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-install-plan", Namespace: operatorNamespaceName}, jaegerIP)
				jaegerIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jaegerIP)
			}, timeout, interval).Should(Succeed())

			mongodbIP := testutil.InstallPlan("mongodb-atlas-kubernetes-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, mongodbIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-install-plan", Namespace: operatorNamespaceName}, mongodbIP)
				mongodbIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, mongodbIP)
			}, timeout, interval).Should(Succeed())

			By("Checking status of the OperandRegistry")
			Eventually(func() operatorv1alpha1.RegistryPhase {
				registryInstance := &operatorv1alpha1.OperandRegistry{}
				Expect(k8sClient.Get(ctx, registryKey, registryInstance)).Should(Succeed())
				return registryInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.RegistryRunning))

			By("Cleaning up olm resources")
			Expect(k8sClient.Delete(ctx, jaegerSub)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, mongodbSub)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, jaegerCSV)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, mongodbCSV)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, jaegerIP)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, mongodbIP)).Should(Succeed())
		})
	})
})
