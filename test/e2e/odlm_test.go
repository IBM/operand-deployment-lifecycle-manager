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

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("Testing ODLM", func() {

	Context("Create Multiple OperandRequests", func() {

		It("Should operators and operands are managed", func() {
			// Create OperandRegistry
			By("Create OperandRegistry")
			reg, err := createOperandRegistry(OperandRegistryNamespace, OperatorNamespace)
			fmt.Printf("The OperandRegistry is %v\n", reg)
			Expect(err).ToNot(HaveOccurred())
			Expect(reg).ToNot(BeNil())

			// Check the status of the OperandRegistry
			By("Check the status of the created OperandRegistry")
			_, err = waitRegistryStatus(operatorv1alpha1.RegistryReady)
			Expect(err).ToNot(HaveOccurred())

			// Create OperandConfig
			By("Create OperandConfig")
			con, err := createOperandConfig(OperandRegistryNamespace)
			fmt.Printf("The OperandRegistry is %v\n", con)
			Expect(err).ToNot(HaveOccurred())
			Expect(con).ToNot(BeNil())

			// Check the status of the OperandConfig
			By("Check the status of the created OperandConfig")
			_, err = waitConfigStatus(operatorv1alpha1.ServiceInit, OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())

			// Create the first OperandRequest
			By("Create the first OperandRequest")
			req1 := newOperandRequestWithoutBindinfo(OperandRequestCrName, OperandRequestNamespace1, OperandRegistryNamespace)
			req1, err = createOperandRequest(req1)
			fmt.Printf("The OperandRegistry is %v\n", req1)
			Expect(err).ToNot(HaveOccurred())
			Expect(req1).ToNot(BeNil())

			// Check the status of the OperandRequest
			By("Check the status of the created OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Status.Members)).Should(Equal(1))

			// Check if the subscription is created
			By("Check if the subscription is created")
			sub, err := retrieveSubscription("etcd", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).ToNot(BeNil())

			// Check if the custom resource is created
			By("Check if the custom resource is created")
			etcdCluster, err := retrieveEtcd("example", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(etcdCluster).ToNot(BeNil())

			// Update the Jenkins operator to the public scope
			By("Update the Jenkins operator to the public scope")
			err = updateJenkinsScope(OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())

			// Check the status of the OperandRequest
			By("Check the status of the created OperandRequest")
			Eventually(func() bool {
				req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
				if err != nil {
					return false
				}
				if len(req1.Status.Members) != 2 {
					return false
				}
				return true
			}, WaitForTimeout, WaitForRetry).Should(BeTrue())

			// Check if the subscription is created
			By("Check if the subscription is created")
			sub, err = retrieveSubscription("jenkins", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).ToNot(BeNil())

			// Check if the custom resource is created
			By("Check if the custom resource is created")
			jenkins, err := retrieveJenkins("example", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(jenkins).ToNot(BeNil())

			// Update the OperandConfig
			By("Update the OperandConfig")
			Eventually(func() bool {
				err = updateEtcdReplicas(OperandRegistryNamespace)
				if err != nil {
					return false
				}
				etcdCluster, err := retrieveEtcd("example", OperatorNamespace)
				if err != nil {
					return false
				}
				if etcdCluster.Object["spec"].(map[string]interface{})["size"].(int64) != 3 {
					return false
				}
				return true
			}, WaitForTimeout, WaitForRetry).Should(BeTrue())

			// Manual create BindInfo to mock alm-example
			By("Create the OperandBindInfo")
			bi, err := createOperandBindInfo(OperatorNamespace, OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(bi).ToNot(BeNil())

			By("Wait the OperandBindInfo is completed")
			bi, err = waitBindInfoStatus(operatorv1alpha1.BindInfoCompleted, OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(bi).ToNot(BeNil())

			By("Wait the OperandRegistry is running")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests)).Should(Equal(1))

			// Create the second OperandRequest instance
			By("Create the second OperandRequest instance")
			req2 := newOperandRequestWithBindinfo(OperandRequestCrName, OperandRequestNamespace2, OperandRegistryNamespace)
			req2, err = createOperandRequest(req2)
			Expect(err).ToNot(HaveOccurred())
			Expect(req2).ToNot(BeNil())

			By("Check the status of the second OperandRequest")
			req2, err = waitRequestStatusRunning(OperandRequestNamespace2)
			Expect(err).ToNot(HaveOccurred())
			Expect(req2).ToNot(BeNil())

			// Check registry status if updated
			By("Wait the OperandRegistry is running")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests)).Should(Equal(2))

			bi, err = waitBindInfoStatus(operatorv1alpha1.BindInfoCompleted, OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(bi).ToNot(BeNil())

			// Check if the secret and configmap are copied
			By("Check if the secret and configmap are copied")
			sec, err := retrieveSecret("jenkins-operator-credentials-example", OperandRequestNamespace2)
			Expect(err).ToNot(HaveOccurred())
			Expect(sec).ToNot(BeNil())

			cm, err := retrieveConfigmap("jenkins-operator-init-configuration-example", OperandRequestNamespace2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			// Update OperandBindInfo
			By("Update OperandBindInfo")
			bi, err = updateOperandBindInfo(OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			cm, err = retrieveConfigmap("jenkins-public-bindinfo-jenkins-operator-base-configuration-example", OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			// Delete the last operator and related operands from the first OperandRequest
			By("Delete the last operator and related operands from the first OperandRequest")
			req1, err = absentOperandFromRequest(OperandRequestNamespace1, "jenkins")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Spec.Requests[0].Operands)).Should(Equal(1))

			By("Check the status of the first OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(req1.Status.Phase).Should(Equal(operatorv1alpha1.ClusterPhaseRunning))

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests)).Should(Equal(1))

			// Add an operator into the first OperandRequest
			By("Add an operator into the first OperandRequest")
			req1, err = presentOperandFromRequest(OperandRequestNamespace1, "jenkins")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Spec.Requests[0].Operands)).Should(Equal(2))

			By("Check the status of the first OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests)).Should(Equal(2))

			// Delete the second OperandRequest
			By("Delete the second OperandRequest")
			err = deleteOperandRequest(req2)
			Expect(err).ToNot(HaveOccurred())

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["jenkins"].ReconcileRequests)).Should(Equal(1))

			// Delete the first OperandRequest
			By("Delete the first OperandRequest")
			err = deleteOperandRequest(req1)
			Expect(err).ToNot(HaveOccurred())

			// Update the channel of the etcd operator
			By("Update the channel of the etcd operator")
			err = updateEtcdChannel(OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())

			// Create the first OperandRequest
			By("Create the first OperandRequest")
			req1 = newOperandRequestWithoutBindinfo(OperandRequestCrName, OperandRequestNamespace1, OperandRegistryNamespace)
			req1, err = createOperandRequest(req1)
			Expect(err).ToNot(HaveOccurred())
			Expect(req1).ToNot(BeNil())

			// Check the status of the OperandRequest
			By("Check the status of the created OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())

			// Check if the subscription is created
			By("Check if the subscription is created")
			sub, err = retrieveSubscription("etcd", "openshift-operators")
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).ToNot(BeNil())

			// Delete the first OperandRequest
			By("Delete the first OperandRequest")
			err = deleteOperandRequest(req1)
			Expect(err).ToNot(HaveOccurred())

			// Delete the OperandBindInfo
			By("Delete the first OperandBindInfo")
			err = deleteOperandBindInfo(bi)
			Expect(err).ToNot(HaveOccurred())

			// Check the status of the OperandRegistry
			By("Check the status of the created OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryReady)
			Expect(err).ToNot(HaveOccurred())

			// Delete OperandRegistry
			By("Delete OperandRegistry")
			err = deleteOperandRegistry(reg)
			Expect(err).ToNot(HaveOccurred())

			// Check the status of the OperandConfig
			By("Check the status of the created OperandConfig")
			con, err = waitConfigStatus(operatorv1alpha1.ServiceInit, OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())

			// Delete OperandConfig
			By("Delete OperandConfig")
			err = deleteOperandConfig(con)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
