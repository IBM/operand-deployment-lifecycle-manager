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

package operandrequest

import (
	"context"

	v1beta2 "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandRegistry controller", func() {
	const (
		name1             = "ibm-cloudpak-name"
		name2             = "ibm-cloudpack-name-2"
		namespace         = "ibm-cloudpak"
		registryName1     = "common-service"
		registryName2     = "common-service-2"
		registryNamespace = "ibm-common-services"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		namespaceName         string
		registryNamespaceName string
		operatorNamespaceName string
		registry1             *operatorv1alpha1.OperandRegistry
		registry2             *operatorv1alpha1.OperandRegistry
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
		By("Creating the OperandRegistry")
		Expect(k8sClient.Create(ctx, registry1)).Should(Succeed())
		Expect(k8sClient.Create(ctx, registry2)).Should(Succeed())
		By("Creating the OperandConfig")
		Expect(k8sClient.Create(ctx, config1)).Should(Succeed())
		Expect(k8sClient.Create(ctx, config2)).Should(Succeed())
	})

	AfterEach(func() {

		By("Deleting the CatalogSource")
		Expect(k8sClient.Delete(ctx, catalogSource)).Should(Succeed())
		By("Deleting the OperandConfig")
		Expect(k8sClient.Delete(ctx, config1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, config2)).Should(Succeed())
		By("Deleting the OperandCRegistry")
		Expect(k8sClient.Delete(ctx, registry1)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, registry2)).Should(Succeed())
	})

	Context("Initializing OperandRequest Status", func() {

		It("Should create the CR via OperandRequest", func() {
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
				etcdSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "etcd", Namespace: operatorNamespaceName}, etcdSub)).Should(Succeed())
				etcdSub.Status = testutil.SubscriptionStatus("etcd", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, etcdSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				jenkinsSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins", Namespace: operatorNamespaceName}, jenkinsSub)).Should(Succeed())
				jenkinsSub.Status = testutil.SubscriptionStatus("jenkins", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jenkinsSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			etcdCSV := testutil.ClusterServiceVersion("etcd-csv.v0.0.1", operatorNamespaceName, testutil.EtcdExample)
			Expect(k8sClient.Create(ctx, etcdCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				etcdCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, etcdCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			jenkinsCSV := testutil.ClusterServiceVersion("jenkins-csv.v0.0.1", operatorNamespaceName, testutil.JenkinsExample)
			Expect(k8sClient.Create(ctx, jenkinsCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				jenkinsCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jenkinsCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			etcdIP := testutil.InstallPlan("etcd-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, etcdIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-install-plan", Namespace: operatorNamespaceName}, etcdIP)
				etcdIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, etcdIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			jenkinsIP := testutil.InstallPlan("jenkins-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jenkinsIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-install-plan", Namespace: operatorNamespaceName}, jenkinsIP)
				jenkinsIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jenkinsIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking of the CR of the etcd operator")
			Eventually(func() error {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: namespaceName}, etcdCluster)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Deleting the OperandRequest")
			Expect(k8sClient.Delete(ctx, requestWithCR)).Should(Succeed())

			By("Checking CR of the etcd operator has been deleted")
			Eventually(func() bool {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: namespaceName}, etcdCluster)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Checking operators have been deleted")

			Eventually(func() bool {
				jenkinsSub := &olmv1alpha1.Subscription{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins", Namespace: operatorNamespaceName}, jenkinsSub)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				jenkinsCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				etcdSub := &olmv1alpha1.Subscription{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "etcd", Namespace: operatorNamespaceName}, etcdSub)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				etcdCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})

		It("Should The CR are created by OperandConfig", func() {

			Expect(k8sClient.Create(ctx, request1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, request2)).Should(Succeed())

			By("Checking status of the OperandRegquest")
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
				etcdSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "etcd", Namespace: operatorNamespaceName}, etcdSub)).Should(Succeed())
				etcdSub.Status = testutil.SubscriptionStatus("etcd", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, etcdSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			Eventually(func() error {
				jenkinsSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins", Namespace: operatorNamespaceName}, jenkinsSub)).Should(Succeed())
				jenkinsSub.Status = testutil.SubscriptionStatus("jenkins", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jenkinsSub)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			etcdCSV := testutil.ClusterServiceVersion("etcd-csv.v0.0.1", operatorNamespaceName, testutil.EtcdExample)
			Expect(k8sClient.Create(ctx, etcdCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				etcdCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, etcdCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			jenkinsCSV := testutil.ClusterServiceVersion("jenkins-csv.v0.0.1", operatorNamespaceName, testutil.JenkinsExample)
			Expect(k8sClient.Create(ctx, jenkinsCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				jenkinsCSV.Status = testutil.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jenkinsCSV)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			etcdIP := testutil.InstallPlan("etcd-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, etcdIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-install-plan", Namespace: operatorNamespaceName}, etcdIP)
				etcdIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, etcdIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			jenkinsIP := testutil.InstallPlan("jenkins-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jenkinsIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-install-plan", Namespace: operatorNamespaceName}, jenkinsIP)
				jenkinsIP.Status = testutil.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jenkinsIP)
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Checking of the CR of the etcd operator")
			Eventually(func() error {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: operatorNamespaceName}, etcdCluster)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Disabling the etcd operator from first OperandRequest")
			requestInstance1 := &operatorv1alpha1.OperandRequest{}
			Expect(k8sClient.Get(ctx, requestKey1, requestInstance1)).Should(Succeed())
			requestInstance1.Spec.Requests[0].Operands = requestInstance1.Spec.Requests[0].Operands[1:]
			Expect(k8sClient.Update(ctx, requestInstance1)).Should(Succeed())
			Eventually(func() error {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: operatorNamespaceName}, etcdCluster)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Disabling the etcd operator from second OperandRequest")
			requestInstance2 := &operatorv1alpha1.OperandRequest{}
			Expect(k8sClient.Get(ctx, requestKey2, requestInstance2)).Should(Succeed())
			requestInstance2.Spec.Requests[0].Operands = requestInstance2.Spec.Requests[0].Operands[1:]
			Expect(k8sClient.Update(ctx, requestInstance2)).Should(Succeed())
			Eventually(func() bool {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: operatorNamespaceName}, etcdCluster)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			By("Deleting the first OperandRequest")
			Expect(k8sClient.Delete(ctx, request1)).Should(Succeed())

			By("Checking jenkins operator has not been deleted")
			Eventually(func() error {
				jenkinsCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				return err
			}, testutil.Timeout, testutil.Interval).Should(Succeed())

			By("Deleting the second OperandRequest")
			Expect(k8sClient.Delete(ctx, request2)).Should(Succeed())

			By("Checking operators have been deleted")
			Eventually(func() bool {
				etcdCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())

			Eventually(func() bool {
				jenkinsCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				return err != nil && errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})
})
