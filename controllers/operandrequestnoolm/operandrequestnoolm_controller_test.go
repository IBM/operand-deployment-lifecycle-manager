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

package operandrequestnoolm

import (
	"context"
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/testutil"
)

var _ = Describe("OperandRequestNoOLM controller", func() {
	const (
		name1             = "ibm-cloudpak-name"
		namespace         = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "data-ns"
	)

	var (
		ctx context.Context

		namespaceName         string
		registryNamespaceName string
		registry              *operatorv1alpha1.OperandRegistry
		config                *operatorv1alpha1.OperandConfig
		request               *operatorv1alpha1.OperandRequest
		requestKey            types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = testutil.CreateNSName(namespace)
		registryNamespaceName = testutil.CreateNSName(registryNamespace)
		registry = testutil.OperandRegistryObj(registryName, registryNamespaceName, registryNamespaceName)
		config = testutil.OperandConfigObj(registryName, registryNamespaceName)
		request = testutil.OperandRequestObj(registryName, registryNamespaceName, name1, namespaceName)
		requestKey = types.NamespacedName{Name: name1, Namespace: namespaceName}

		By("Creating the Namespace")
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(registryNamespaceName))).Should(Succeed())
	})

	Context("Reconciling OperandRequest without OLM", func() {
		It("Should add finalizer to OperandRequest", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Checking finalizer is added to the OperandRequest")
			Eventually(func() bool {
				instance := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey, instance)).Should(Succeed())
				return len(instance.GetFinalizers()) > 0
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})

		It("Should handle permissions check correctly", func() {
			// Create a mock reconciler with a fake client
			fakeClient := fake.NewClientBuilder().Build()
			reconciler := &Reconciler{
				ODLMOperator: &deploy.ODLMOperator{
					Client: fakeClient,
				},
				Mutex: sync.Mutex{},
			}

			// Test permission check
			By("Verifying no-permission case")
			hasPermission := reconciler.checkPermission(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-request",
					Namespace: "test-namespace",
				},
			})
			Expect(hasPermission).To(BeFalse())
		})

		It("Should initialize request status", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Checking status is initialized")
			Eventually(func() bool {
				instance := &operatorv1alpha1.OperandRequest{}
				Expect(k8sClient.Get(ctx, requestKey, instance)).Should(Succeed())
				return instance.Status.Phase != ""
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})

		It("Should handle deletion correctly", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Deleting the OperandRequest")
			Expect(k8sClient.Delete(ctx, request)).Should(Succeed())

			By("Verifying finalizer is removed")
			Eventually(func() bool {
				instance := &operatorv1alpha1.OperandRequest{}
				err := k8sClient.Get(ctx, requestKey, instance)
				return errors.IsNotFound(err)
			}, testutil.Timeout, testutil.Interval).Should(BeTrue())
		})
	})

	Context("Testing mapper functions", func() {
		It("Should map OperandRegistry to OperandRequests", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			reconciler := &Reconciler{
				ODLMOperator: &deploy.ODLMOperator{
					Client: k8sClient,
				},
			}

			By("Verifying getRegistryToRequestMapper works")
			mapper := reconciler.getRegistryToRequestMapper()
			requests := mapper(registry)
			Expect(len(requests)).To(BeNumerically(">", 0))
		})

		It("Should map OperandConfig to OperandRequests", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			reconciler := &Reconciler{
				ODLMOperator: &deploy.ODLMOperator{
					Client: k8sClient,
				},
			}

			By("Verifying getConfigToRequestMapper works")
			mapper := reconciler.getConfigToRequestMapper()
			requests := mapper(config)
			Expect(len(requests)).To(BeNumerically(">", 0))
		})

		It("Should map references to OperandRequests", func() {
			By("Creating the OperandRegistry")
			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			By("Creating the OperandConfig")
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())
			By("Creating the OperandRequest")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Creating a ConfigMap with reference annotation")
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: registryNamespaceName,
					Annotations: map[string]string{
						constant.ODLMReferenceAnnotation: fmt.Sprintf("OperandRegistry.%s.%s", registryNamespaceName, registryName),
					},
				},
				Data: map[string]string{
					"test-key": "test-value",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())

			reconciler := &Reconciler{
				ODLMOperator: &deploy.ODLMOperator{
					Client: k8sClient,
				},
			}

			By("Verifying getReferenceToRequestMapper works")
			mapper := reconciler.getReferenceToRequestMapper()
			requests := mapper(cm)
			Expect(len(requests)).To(BeNumerically(">", 0))
		})
	})
})
