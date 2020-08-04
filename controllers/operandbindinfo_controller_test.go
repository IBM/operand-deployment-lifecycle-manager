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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	testdata "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandBindInfo controller", func() {
	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		name              = "ibm-operators-bindinfo"
		namespace         = "ibm-operators"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-services"
	)

	Context("When creating OperandBindInfo", func() {
		It("Should BindInfo is created", func() {
			ctx := context.Background()
			secret1 := testdata.SecretObj("secret1", namespace)
			secret2 := testdata.SecretObj("secret2", namespace)
			configmap1 := testdata.ConfigmapObj("cm1", namespace)
			configmap2 := testdata.ConfigmapObj("cm2", namespace)
			registry := testdata.OperandRegistryObj(registryName, registryNamespace, namespace)
			config := testdata.OperandConfigObj(registryName, registryNamespace)
			request := testdata.OperandRequestObj(registryName, registryNamespace, requestName, requestNamespace)
			bindInfo := testdata.OperandBindInfoObj(name, namespace, registryName, registryNamespace)
			secret3Key := types.NamespacedName{Name: "secret3", Namespace: requestNamespace}
			cm3Key := types.NamespacedName{Name: "cm3", Namespace: requestNamespace}

			By("Prepare init resources for OperandBindInfo controller")
			Expect(k8sClient.Create(ctx, testdata.NamespaceObj(namespace))).Should(Succeed())
			Expect(k8sClient.Create(ctx, testdata.NamespaceObj(registryNamespace))).Should(Succeed())
			Expect(k8sClient.Create(ctx, testdata.NamespaceObj(requestNamespace))).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap2)).Should(Succeed())

			Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, config)).Should(Succeed())

			By("Creating a new OperandBindInfo")
			Expect(k8sClient.Create(ctx, bindInfo)).Should(Succeed())

			By("By creating a OperandRequest to trigger BindInfo controller")
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			// We'll need to retry getting this newly created CronJob, given that creation may not immediately happen.
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secret3Key, &corev1.Secret{})
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cm3Key, &corev1.ConfigMap{})
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("By delete the OperandRequest to trigger BindInfo controller")
			Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secret3Key, &corev1.Secret{})
				return err != nil
			}, timeout, interval).ShouldNot(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cm3Key, &corev1.ConfigMap{})
				return err != nil
			}, timeout, interval).ShouldNot(BeTrue())

			By("Deletinng the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())
		})
	})
})
