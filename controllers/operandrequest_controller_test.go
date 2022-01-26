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

package controllers

import (
	"context"

	v1beta2 "github.com/coreos/etcd-operator/pkg/apis/etcd/v1beta2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
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
		registryNamespace = "ibm-common-services"
		operatorNamespace = "ibm-operators"
	)

	var (
		ctx context.Context

		namespaceName         string
		registryNamespaceName string
		operatorNamespaceName string
		registry              *operatorv1alpha1.OperandRegistry
		config                *operatorv1alpha1.OperandConfig
		request               *operatorv1alpha1.OperandRequest
		catalogSource         *olmv1alpha1.CatalogSource
		requestKey            types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = createNSName(namespace)
		registryNamespaceName = createNSName(registryNamespace)
		operatorNamespaceName = createNSName(operatorNamespace)
		registry = testdata.OperandRegistryObj(registryName, registryNamespaceName, operatorNamespaceName)
		config = testdata.OperandConfigObj(registryName, registryNamespaceName)
		request = testdata.OperandRequestObj(registryName, registryNamespaceName, name, namespaceName)
		catalogSource = testdata.CatalogSource("community-operators", "openshift-marketplace")
		requestKey = types.NamespacedName{Name: name, Namespace: namespaceName}

		By("Creating the Namespace")
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(registryNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj(operatorNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testdata.NamespaceObj("openshift-marketplace")))

		By("Creating the CatalogSource")
		Expect(k8sClient.Create(ctx, catalogSource)).Should(Succeed())
		catalogSource.Status = testdata.CatalogSourceStatus()
		Expect(k8sClient.Status().Update(ctx, catalogSource)).Should(Succeed())
		By("Creating the OperandRegistry")
		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		By("Creating the OperandConfig")
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())
	})

	AfterEach(func() {

		By("Deleting the CatalogSource")
		Expect(k8sClient.Delete(ctx, catalogSource)).Should(Succeed())
		By("Deleting the OperandConfig")
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
		By("Deleting the OperandCRegistry")
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
	})

	Context("Initializing OperandRequest Status", func() {

		It("Should The CR are created", func() {

			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Checking status of the OperandRegquest")
			Eventually(func() operatorv1alpha1.ClusterPhase {
				requestInstance := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey, requestInstance)).Should(Succeed())
				return requestInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.ClusterPhaseInstalling))

			By("Setting status of the Subscriptions")
			Eventually(func() error {
				etcdSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "etcd", Namespace: operatorNamespaceName}, etcdSub)).Should(Succeed())
				etcdSub.Status = testdata.SubscriptionStatus("etcd", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, etcdSub)
			}, timeout, interval).Should(Succeed())

			Eventually(func() error {
				jenkinsSub := &olmv1alpha1.Subscription{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins", Namespace: operatorNamespaceName}, jenkinsSub)).Should(Succeed())
				jenkinsSub.Status = testdata.SubscriptionStatus("jenkins", operatorNamespaceName, "0.0.1")
				return k8sClient.Status().Update(ctx, jenkinsSub)
			}, timeout, interval).Should(Succeed())

			By("Creating and Setting status of the ClusterServiceVersions")
			etcdCSV := testdata.ClusterServiceVersion("etcd-csv.v0.0.1", operatorNamespaceName, testdata.EtcdExample)
			Expect(k8sClient.Create(ctx, etcdCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				etcdCSV.Status = testdata.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, etcdCSV)
			}, timeout, interval).Should(Succeed())

			jenkinsCSV := testdata.ClusterServiceVersion("jenkins-csv.v0.0.1", operatorNamespaceName, testdata.JenkinsExample)
			Expect(k8sClient.Create(ctx, jenkinsCSV)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				jenkinsCSV.Status = testdata.ClusterServiceVersionStatus()
				return k8sClient.Status().Update(ctx, jenkinsCSV)
			}, timeout, interval).Should(Succeed())

			By("Creating and Setting status of the InstallPlan")
			etcdIP := testdata.InstallPlan("etcd-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, etcdIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-install-plan", Namespace: operatorNamespaceName}, etcdIP)
				etcdIP.Status = testdata.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, etcdIP)
			}, timeout, interval).Should(Succeed())

			jenkinsIP := testdata.InstallPlan("jenkins-install-plan", operatorNamespaceName)
			Expect(k8sClient.Create(ctx, jenkinsIP)).Should(Succeed())
			Eventually(func() error {
				k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-install-plan", Namespace: operatorNamespaceName}, jenkinsIP)
				jenkinsIP.Status = testdata.InstallPlanStatus()
				return k8sClient.Status().Update(ctx, jenkinsIP)
			}, timeout, interval).Should(Succeed())

			By("Checking of the CR of the etcd operator")
			Eventually(func() error {
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: operatorNamespaceName}, etcdCluster)
				return err
			}, timeout, interval).Should(Succeed())

			By("Disabling the etcd operator")
			Eventually(func() bool {
				requestInstance := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey, requestInstance)).Should(Succeed())
				requestInstance.Spec.Requests[0].Operands = requestInstance.Spec.Requests[0].Operands[1:]
				Expect(k8sClient.Update(ctx, requestInstance)).Should(Succeed())
				etcdCluster := &v1beta2.EtcdCluster{}
				err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: operatorNamespaceName}, etcdCluster)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Deleting the OperandRequest")
			Expect(k8sClient.Delete(ctx, request)).Should(Succeed())

			By("Checking operators have been deleted")
			Eventually(func() bool {
				etcdCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "etcd-csv.v0.0.1", Namespace: operatorNamespaceName}, etcdCSV)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				jenkinsCSV := &olmv1alpha1.ClusterServiceVersion{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "jenkins-csv.v0.0.1", Namespace: operatorNamespaceName}, jenkinsCSV)
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
