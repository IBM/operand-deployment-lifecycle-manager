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
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/go-cmp/cmp"
	jaegerv1 "github.com/jaegertracing/jaeger-operator/apis/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandRequest controller", func() {
	const (
		name1             = "ibm-cloudpak-name"
		name2             = "ibm-cloudpack-name-2"
		namespace         = "ibm-cloudpak"
		registryName1     = "common-service"
		registryName2     = "common-service-2"
		registryNamespace = "data-ns"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		namespaceName         string
		registryNamespaceName string
		operatorNamespaceName string
		registry1             *operatorv1alpha1.OperandRegistry
		registry2             *operatorv1alpha1.OperandRegistry
		registrywithCfg       *operatorv1alpha1.OperandRegistry
		config1               *operatorv1alpha1.OperandConfig
		config2               *operatorv1alpha1.OperandConfig
		request1              *operatorv1alpha1.OperandRequest
		request2              *operatorv1alpha1.OperandRequest
		requestWithCR         *operatorv1alpha1.OperandRequest
		catalogSource         *olmv1alpha1.CatalogSource
		requestKey1           types.NamespacedName
		requestKey2           types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = testutil.CreateNSName(namespace)
		registryNamespaceName = testutil.CreateNSName(registryNamespace)
		operatorNamespaceName = testutil.CreateNSName(operatorNamespace)
		registry1 = testutil.OperandRegistryObj(registryName1, registryNamespaceName, operatorNamespaceName)
		registry2 = testutil.OperandRegistryObj(registryName2, registryNamespaceName, operatorNamespaceName)
		registrywithCfg = testutil.OperandRegistryObjwithCfg(registryName1, registryNamespaceName, operatorNamespaceName)
		config1 = testutil.OperandConfigObj(registryName1, registryNamespaceName)
		config2 = testutil.OperandConfigObj(registryName2, registryNamespaceName)
		request1 = testutil.OperandRequestObj(registryName1, registryNamespaceName, name1, namespaceName)
		request2 = testutil.OperandRequestObj(registryName2, registryNamespaceName, name2, namespaceName)
		requestWithCR = testutil.OperandRequestObjWithCR(registryName1, registryNamespaceName, name1, namespaceName)
		catalogSource = testutil.CatalogSource("community-operators", "openshift-marketplace")
		requestKey1 = types.NamespacedName{Name: name1, Namespace: namespaceName}
		requestKey2 = types.NamespacedName{Name: name2, Namespace: namespaceName}

		By("Creating the Namespace")
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(registryNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(operatorNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj("openshift-marketplace")))

		By("Creating the CatalogSource")
		Expect(k8sClient.Create(ctx, catalogSource)).Should(Succeed())
		catalogSource.Status = testutil.CatalogSourceStatus()
		Expect(k8sClient.Status().Update(ctx, catalogSource)).Should(Succeed())
	})

	AfterEach(func() {
		By("Deleting the CatalogSource")
		Expect(k8sClient.Delete(ctx, catalogSource)).Should(Succeed())
	})

	Context("Initializing OperandRequest Status", func() {

		It("Should create the CR via OperandRequest", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, registry2)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, config2)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, requestWithCR)).Should(Succeed())
			Eventually(func() error {
				req := &operatorv1alpha1.OperandRequest{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name1, Namespace: namespaceName}, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking status of the OperandRequest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance1 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
				return requestInstance1.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))

			By("Setting status of the Subscriptions")
			Eventually(func() error {
				jaegerSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)).Should(Succeed())
				jaegerSub.Status = testutil.SubscriptionStatus("jaeger", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jaegerSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				mongodbSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes", Namespace: operatorNamespaceName}, mongodbSub)).Should(Succeed())
				mongodbSub.Status = testutil.SubscriptionStatus("mongodb-atlas-kubernetes", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, mongodbSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			jaegerCSV := testutil.ClusterServiceVersion("jaeger-csv.v0.0.1", "jaeger", operatorNamespaceName, testutil.JaegerExample)
			Expect(k8sClient.Create(ctx, jaegerCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				jaegerCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jaegerCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbCSV := testutil.ClusterServiceVersion("mongodb-atlas-kubernetes-csv.v0.0.1", "mongodb-atlas-kubernetes", operatorNamespaceName, testutil.MongodbExample)
			Expect(k8sClient.Create(ctx, mongodbCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				mongodbCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, mongodbCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			jaegerIP := testutil.InstallPlan("jaeger-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jaegerIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-install-plan", Namespace: operatorNamespaceName}, jaegerIP)
				jaegerIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jaegerIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbIP := testutil.InstallPlan("mongodb-atlas-kubernetes-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, mongodbIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-install-plan", Namespace: operatorNamespaceName}, mongodbIP)
				mongodbIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, mongodbIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking first CR of the jaeger operator")
			Eventually(func() error {
				jaegerCR := &jaegerv1.Jaeger{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "my-jaeger", Namespace: namespaceName}, jaegerCR)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking second CR of the jaeger operator")
			Eventually(func() error {
				jaegerCR := &jaegerv1.Jaeger{}
				crInfo := sha256.Sum256([]byte("jaegertracing.io/v1" + "Jaeger" + "1"))
				jaegerCRName := name1 + "-" + hex.EncodeToString(crInfo[:7])
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: jaegerCRName, Namespace: namespaceName}, jaegerCR)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Deleting the OperandRequest")
			Expect(k8sClient.Delete(ctx, requestWithCR)).Should(Succeed())

			By("Checking CR of the jaeger operator has been deleted")
			Eventually(func() bool {
				jaegerCR := &jaegerv1.Jaeger{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "my-jaeger", Namespace: namespaceName}, jaegerCR)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking operators have been deleted")

			Eventually(func() bool {
				mongodbSub := &olmv1alpha1.Subscription{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes", Namespace: operatorNamespaceName}, mongodbSub)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				mongodbCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				jaegerSub := &olmv1alpha1.Subscription{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				jaegerCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Deleting the OperandConfig")
			Expect(k8sClient.Delete(ctx, config1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, config2)).Should(Succeed())
			By("Deleting the OperandRegistry")
			Expect(k8sClient.Delete(ctx, registry1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, registry2)).Should(Succeed())
		})

		It("Should create the CR via OperandConfig", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, registry2)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, config2)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, request2)).Should(Succeed())

			By("Checking status of the OperandRequest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance1 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
				return requestInstance1.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance2 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey2, requestInstance2)).Should(Succeed())
				return requestInstance2.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))

			By("Setting status of the Subscriptions")
			Eventually(func() error {
				jaegerSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)).Should(Succeed())
				jaegerSub.Status = testutil.SubscriptionStatus("jaeger", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jaegerSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				mongodbSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes", Namespace: operatorNamespaceName}, mongodbSub)).Should(Succeed())
				mongodbSub.Status = testutil.SubscriptionStatus("mongodb-atlas-kubernetes", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, mongodbSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			jaegerCSV := testutil.ClusterServiceVersion("jaeger-csv.v0.0.1", "jaeger", operatorNamespaceName, testutil.JaegerExample)
			Expect(k8sClient.Create(ctx, jaegerCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				jaegerCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jaegerCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbCSV := testutil.ClusterServiceVersion("mongodb-atlas-kubernetes-csv.v0.0.1", "mongodb-atlas-kubernetes", operatorNamespaceName, testutil.MongodbExample)
			Expect(k8sClient.Create(ctx, mongodbCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				mongodbCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, mongodbCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			jaegerIP := testutil.InstallPlan("jaeger-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jaegerIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-install-plan", Namespace: operatorNamespaceName}, jaegerIP)
				jaegerIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jaegerIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbIP := testutil.InstallPlan("mongodb-atlas-kubernetes-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, mongodbIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-install-plan", Namespace: operatorNamespaceName}, mongodbIP)
				mongodbIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, mongodbIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking of the CR of the jaeger operator")
			Eventually(func() error {
				jaegerCR := &jaegerv1.Jaeger{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "my-jaeger", Namespace: registryNamespaceName}, jaegerCR)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking of the k8s resource of the jaeger operator")
			Eventually(func() error {
				jaegerConfigMap := &corev1.ConfigMap{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "jaeger-configmap", Namespace: registryNamespaceName}, jaegerConfigMap)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				jaegerConfigMap := &corev1.ConfigMap{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "jaeger-configmap-reference", Namespace: registryNamespaceName}, jaegerConfigMap)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking the status of first OperandRequest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance1 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
				return requestInstance1.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseRunning))

			By("Disabling the jaeger operator from first OperandRequest")
			requestInstance1 := &operatorv1alpha1.OperandRequest{}
			Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
			requestInstance1.Spec.Requests[0].Operands = requestInstance1.Spec.Requests[0].Operands[1:]
			Eventually(func() error {
				return k8sClient.Update(ctx, requestInstance1)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
			Eventually(func() error {
				jaegerCR := &jaegerv1.Jaeger{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "my-jaeger", Namespace: registryNamespaceName}, jaegerCR)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking the status of first OperandRequest after updating the Operand")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance1 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
				return requestInstance1.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseRunning))

			By("Disabling the jaeger operator from second OperandRequest")
			requestInstance2 := &operatorv1alpha1.OperandRequest{}
			Expect(k8sClient.Get(ctx, requestKey2, requestInstance2)).Should(Succeed())
			requestInstance2.Spec.Requests[0].Operands = requestInstance2.Spec.Requests[0].Operands[1:]
			Eventually(func() error {
				return k8sClient.Update(ctx, requestInstance2)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())
			Eventually(func() bool {
				jaegerCR := &jaegerv1.Jaeger{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "my-jaeger", Namespace: registryNamespaceName}, jaegerCR)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking jaeger k8s resource has been deleted")
			Eventually(func() bool {
				jaegerConfigMap := &corev1.ConfigMap{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "jaeger-configmap", Namespace: registryNamespaceName}, jaegerConfigMap)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
			Eventually(func() bool {
				jaegerConfigMap := &corev1.ConfigMap{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "jaeger-configmap-reference", Namespace: registryNamespaceName}, jaegerConfigMap)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking jaeger operators have been deleted")
			Eventually(func() bool {
				jaegerCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking the status of second OperandRequest after updating the Operand")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance2 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey2, requestInstance2)).Should(Succeed())
				return requestInstance2.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseRunning))

			By("Deleting the first OperandRequest")
			Expect(k8sClient.Delete(ctx, request1)).Should(Succeed())

			By("Checking mongodb operator has not been deleted")
			Eventually(func() error {
				mongodbCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Deleting the second OperandRequest")
			Expect(k8sClient.Delete(ctx, request2)).Should(Succeed())

			By("Checking the mongodb-atlas operator has been deleted")
			Eventually(func() bool {
				mongodbCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Deleting the OperandConfig")
			Expect(k8sClient.Delete(ctx, config1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, config2)).Should(Succeed())
			By("Deleting the OperandRegistry")
			Expect(k8sClient.Delete(ctx, registry1)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, registry2)).Should(Succeed())
		})

		It("Should Config Operator by OperandRegistry", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registrywithCfg)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config1)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request1)).Should(Succeed())
			Eventually(func() error {
				req := &operatorv1alpha1.OperandRequest{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: name1, Namespace: namespaceName}, req)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking status of the OperandRequest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance1 := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
				return requestInstance1.Status.Phase
			}, testutil.Timeout, testutil.Interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))

			By("Setting status of the Subscriptions")
			Eventually(func() error {
				jaegerSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)).Should(Succeed())
				jaegerSub.Status = testutil.SubscriptionStatus("jaeger", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jaegerSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				mongodbSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes", Namespace: operatorNamespaceName}, mongodbSub)).Should(Succeed())
				mongodbSub.Status = testutil.SubscriptionStatus("mongodb-atlas-kubernetes", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, mongodbSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			jaegerCSV := testutil.ClusterServiceVersion("jaeger-csv.v0.0.1", "jaeger", operatorNamespaceName, testutil.JaegerExample)
			Expect(k8sClient.Create(ctx, jaegerCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-csv.v0.0.1", Namespace: operatorNamespaceName}, jaegerCSV)
				jaegerCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jaegerCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbCSV := testutil.ClusterServiceVersion("mongodb-atlas-kubernetes-csv.v0.0.1", "mongodb-atlas-kubernetes", operatorNamespaceName, testutil.MongodbExample)
			Expect(k8sClient.Create(ctx, mongodbCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-csv.v0.0.1", Namespace: operatorNamespaceName}, mongodbCSV)
				mongodbCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, mongodbCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			jaegerIP := testutil.InstallPlan("jaeger-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jaegerIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger-install-plan", Namespace: operatorNamespaceName}, jaegerIP)
				jaegerIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jaegerIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			mongodbIP := testutil.InstallPlan("mongodb-atlas-kubernetes-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, mongodbIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "mongodb-atlas-kubernetes-install-plan", Namespace: operatorNamespaceName}, mongodbIP)
				mongodbIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, mongodbIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			// Check subscription
			Eventually(func() bool {
				jaegerSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)).Should(Succeed())
				return (jaegerSub != nil)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				jaegerSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jaeger", Namespace: operatorNamespaceName}, jaegerSub)).Should(Succeed())
				return cmp.Equal(jaegerSub.Spec.Config, testutil.SubConfig)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Deleting the OperandConfig")
			Expect(k8sClient.Delete(ctx, config1)).Should(Succeed())
			By("Deleting the OperandRegistry")
			Expect(k8sClient.Delete(ctx, registrywithCfg)).Should(Succeed())
		})
	})
})
