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

package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("Testing ODLM", func() {

	Context("Create Multiple OperandRequests", func() {

		It("Should operators and operands are managed", func() {
			// Create OperandRegistry
			By("Create OperandRegistry")
			reg, err := createOperandRegistry(OperandRegistryNamespace, OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(reg).ToNot(BeNil())

			// Check the status of the OperandRegistry
			By("Check the status of the created OperandRegistry")
			_, err = waitRegistryStatus(operatorv1alpha1.RegistryReady)
			Expect(err).ToNot(HaveOccurred())

			// Create OperandConfig
			By("Create OperandConfig")
			con, err := createOperandConfig(OperandRegistryNamespace)
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
			Expect(err).ToNot(HaveOccurred())
			Expect(req1).ToNot(BeNil())

			// Check the status of the OperandRequest
			By("Check the status of the created OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Status.Members)).Should(Equal(1))

			// Check if the subscription is created
			By("Check if the subscription is created")
			sub, err := retrieveSubscription("jaeger", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).ToNot(BeNil())

			// Check if the custom resource is created
			By("Check if the custom resource is created")
			JaegerCR, err := retrieveJaeger("my-jaeger", OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(JaegerCR).ToNot(BeNil())

			// Update the Mongodb operator to the public scope
			By("Update the Mongodb operator to the public scope")
			err = updateMongodbScope(OperandRegistryNamespace)
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
			sub, err = retrieveSubscription("mongodb-atlas-kubernetes", OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(sub).ToNot(BeNil())

			// Check if the custom resource is created
			By("Check if the custom resource is created")
			mongodb, err := retrieveMongodb("my-atlas-deployment", OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(mongodb).ToNot(BeNil())

			// Check if the k8s resource is created
			By("Check if the k8s resource is created")
			mongodbConfigmap, err := retrieveConfigmap("mongodb-configmap", OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(mongodbConfigmap).ToNot(BeNil())

			// Update the OperandConfig
			By("Update the OperandConfig")
			Eventually(func() bool {
				err = updateJaegerStrategy(OperandRegistryNamespace)
				if err != nil {
					return false
				}
				jaegerCR, err := retrieveJaeger("my-jaeger", OperandRegistryNamespace)
				if err != nil {
					return false
				}
				if jaegerCR.Object["spec"].(map[string]interface{})["strategy"].(string) != "allinone" {
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
			Expect(len(reg.Status.OperatorsStatus["mongodb-atlas-kubernetes"].ReconcileRequests)).Should(Equal(1))

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
			Expect(len(reg.Status.OperatorsStatus["mongodb-atlas-kubernetes"].ReconcileRequests)).Should(Equal(2))

			bi, err = waitBindInfoStatus(operatorv1alpha1.BindInfoCompleted, OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(bi).ToNot(BeNil())

			// Check if the secret and configmap are copied
			By("Check if the secret and configmap are copied")
			sec, err := retrieveSecret("mongodb-secret", OperandRequestNamespace2)
			Expect(err).ToNot(HaveOccurred())
			Expect(sec).ToNot(BeNil())

			cm, err := retrieveConfigmap("mongodb-configmap", OperandRequestNamespace2)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			// Update OperandBindInfo
			By("Update OperandBindInfo")
			bi, err = updateOperandBindInfo(OperatorNamespace)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			cm, err = retrieveConfigmap("mongodb-public-bindinfo-mongodb-second-configmap", OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(cm).ToNot(BeNil())

			// Delete the last operator and related operands from the first OperandRequest
			By("Delete the last operator and related operands from the first OperandRequest")
			req1, err = absentOperandFromRequest(OperandRequestNamespace1, "mongodb-atlas-kubernetes")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Spec.Requests[0].Operands)).Should(Equal(1))

			By("Check the status of the first OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())
			Expect(req1.Status.Phase).Should(Equal(operatorv1alpha1.ClusterPhaseRunning))

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["mongodb-atlas-kubernetes"].ReconcileRequests)).Should(Equal(1))

			// Add an operator into the first OperandRequest
			By("Add an operator into the first OperandRequest")
			req1, err = presentOperandFromRequest(OperandRequestNamespace1, "mongodb-atlas-kubernetes")
			Expect(err).ToNot(HaveOccurred())
			Expect(len(req1.Spec.Requests[0].Operands)).Should(Equal(2))

			By("Check the status of the first OperandRequest")
			req1, err = waitRequestStatusRunning(OperandRequestNamespace1)
			Expect(err).ToNot(HaveOccurred())

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["mongodb-atlas-kubernetes"].ReconcileRequests)).Should(Equal(2))

			// Delete the second OperandRequest
			By("Delete the second OperandRequest")
			err = deleteOperandRequest(req2)
			Expect(err).ToNot(HaveOccurred())

			By("Check the status of the OperandRegistry")
			reg, err = waitRegistryStatus(operatorv1alpha1.RegistryRunning)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(reg.Status.OperatorsStatus["mongodb-atlas-kubernetes"].ReconcileRequests)).Should(Equal(1))

			// Delete the first OperandRequest
			By("Delete the first OperandRequest")
			err = deleteOperandRequest(req1)
			Expect(err).ToNot(HaveOccurred())

			// Check if the k8s resource is deleted
			By("Check if the k8s resource is deleted")
			err = waitConfigmapDeletion("mongodb-configmap", OperandRegistryNamespace)
			Expect(err).ToNot(HaveOccurred())

			// Update the channel of the jaeger operator
			By("Update the channel of the jaeger operator")
			err = updateJaegerChannel(OperandRegistryNamespace)
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
			sub, err = retrieveSubscription("jaeger", "openshift-operators")
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
