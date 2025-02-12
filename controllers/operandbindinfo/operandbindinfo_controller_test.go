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

package operandbindinfo

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/testutil"
)

// +kubebuilder:docs-gen:collapse=Imports

var _ = Describe("OperandBindInfo controller", func() {
	const (
		name              = "ibm-operators-bindinfo"
		namespace         = "ibm-operators"
		requestName       = "ibm-cloudpak-name"
		requestNamespace  = "ibm-cloudpak"
		registryName      = "common-service"
		registryNamespace = "ibm-common-services"
	)

	var (
		ctx context.Context

		namespaceName         string
		registryNamespaceName string
		requestNamespaceName  string
		registry              *operatorv1alpha1.OperandRegistry
		config                *operatorv1alpha1.OperandConfig
		request               *operatorv1alpha1.OperandRequest
		bindInfo              *operatorv1alpha1.OperandBindInfo
		bindInfoKey           types.NamespacedName
		secret1               *corev1.Secret
		secret2               *corev1.Secret
		secret3               *corev1.Secret
		secret4               *corev1.Secret
		configmap1            *corev1.ConfigMap
		configmap2            *corev1.ConfigMap
		configmap3            *corev1.ConfigMap
		configmap4            *corev1.ConfigMap
		secret4Key            types.NamespacedName
		cm4Key                types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		namespaceName = testutil.CreateNSName(namespace)
		registryNamespaceName = testutil.CreateNSName(registryNamespace)
		requestNamespaceName = testutil.CreateNSName(requestNamespace)
		registry = testutil.OperandRegistryObj(registryName, registryNamespaceName, namespaceName)
		config = testutil.OperandConfigObj(registryName, registryNamespaceName)
		request = testutil.OperandRequestObj(registryName, registryNamespaceName, requestName, requestNamespaceName)
		bindInfo = testutil.OperandBindInfoObj(name, namespaceName, registryName, registryNamespaceName)
		bindInfoKey = types.NamespacedName{Name: name, Namespace: namespaceName}

		secret1 = testutil.SecretObj("secret1", registryNamespaceName)
		secret2 = testutil.SecretObj("secret2", registryNamespaceName)
		secret3 = testutil.SecretObj("secret3", registryNamespaceName)
		secret4 = testutil.SecretObj("secret4", requestNamespaceName)
		configmap1 = testutil.ConfigmapObj("cm1", registryNamespaceName)
		configmap2 = testutil.ConfigmapObj("cm2", registryNamespaceName)
		configmap3 = testutil.ConfigmapObj("cm3", registryNamespaceName)
		configmap4 = testutil.ConfigmapObj("cm4", requestNamespaceName)
		secret4Key = types.NamespacedName{Name: "secret4", Namespace: requestNamespaceName}
		cm4Key = types.NamespacedName{Name: "cm4", Namespace: requestNamespaceName}

		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(namespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(registryNamespaceName))).Should(Succeed())
		Expect(k8sClient.Create(ctx, testutil.NamespaceObj(requestNamespaceName))).Should(Succeed())

		Expect(k8sClient.Create(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Create(ctx, config)).Should(Succeed())

		By("Creating a new OperandBindInfo")
		Expect(k8sClient.Create(ctx, bindInfo)).Should(Succeed())

		By("By creating a OperandRequest to trigger BindInfo controller")
		Expect(k8sClient.Create(ctx, request)).Should(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, registry)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, config)).Should(Succeed())
	})

	Context("Sharing the the secret and configmap with public scope", func() {
		It("Should Status of the OperandBindInfo be completed", func() {

			By("Prepare init resources for OperandBindInfo controller")
			Expect(k8sClient.Create(ctx, secret1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret3)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap3)).Should(Succeed())

			By("Check if the public secret and configmap are shared")
			Eventually(func() []byte {
				secret4 := &corev1.Secret{}
				err := k8sClient.Get(ctx, secret4Key, secret4)
				if err != nil {
					return []byte("")
				}
				return secret4.Data["test"]
			}, timeout, interval).Should(Equal([]byte("secret1")))
			Eventually(func() bool {
				cm4 := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, cm4Key, cm4)
				return err == nil && cm4.Data["test"] == "cm1"
			}, timeout, interval).Should(BeTrue())

			By("Check if the private secret and configmap are shared")

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-secret2", Namespace: requestNamespaceName}, &corev1.Secret{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-cm2", Namespace: requestNamespaceName}, &corev1.ConfigMap{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Check if the protected secret and configmap are shared")

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-secret3", Namespace: requestNamespaceName}, &corev1.Secret{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-cm3", Namespace: requestNamespaceName}, &corev1.ConfigMap{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Check status of the OperandBindInfo")
			Eventually(func() operatorv1alpha1.BindInfoPhase {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return bindInfoInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.BindInfoCompleted))

			Eventually(func() int {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return len(bindInfoInstance.Status.RequestNamespaces)
			}, timeout, interval).Should(Equal(1))

			By("Deleting the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())

			By("Check if the public secret and configmap are deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secret4Key, &corev1.Secret{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cm4Key, &corev1.ConfigMap{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Sharing the the secret and configmap with protected scope", func() {
		It("Should Status of the OperandBindInfo be completed", func() {

			request = testutil.OperandRequestObjWithProtected(registryName, registryNamespaceName, requestName, requestNamespaceName)

			By("By updating a OperandRequest to trigger BindInfo controller")
			Expect(k8sClient.Delete(ctx, request)).Should(Succeed())
			Expect(k8sClient.Create(ctx, request)).Should(Succeed())

			By("Prepare init resources for OperandBindInfo controller")
			Expect(k8sClient.Create(ctx, secret1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret3)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap3)).Should(Succeed())

			By("Check if the public secret and configmap are shared")
			Eventually(func() []byte {
				secret4 := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-secret1", Namespace: requestNamespaceName}, secret4)
				if err != nil {
					return []byte("")
				}
				return secret4.Data["test"]
			}, timeout, interval).Should(Equal([]byte("secret1")))
			Eventually(func() bool {
				cm4 := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-cm1", Namespace: requestNamespaceName}, cm4)
				return err == nil && cm4.Data["test"] == "cm1"
			}, timeout, interval).Should(BeTrue())

			By("Check if the private secret and configmap are shared")

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-secret2", Namespace: requestNamespaceName}, &corev1.Secret{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: name + "-cm2", Namespace: requestNamespaceName}, &corev1.ConfigMap{})
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Check if the protected secret and configmap are shared")

			Eventually(func() []byte {
				secret5 := &corev1.Secret{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "secret5", Namespace: requestNamespaceName}, secret5)
				if err != nil {
					return []byte("")
				}
				return secret5.Data["test"]
			}, timeout, interval).Should(Equal([]byte("secret3")))
			Eventually(func() bool {
				cm5 := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "cm5", Namespace: requestNamespaceName}, cm5)
				return err == nil && cm5.Data["test"] == "cm3"
			}, timeout, interval).Should(BeTrue())

			By("Check status of the OperandBindInfo")
			Eventually(func() operatorv1alpha1.BindInfoPhase {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return bindInfoInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.BindInfoCompleted))

			Eventually(func() int {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return len(bindInfoInstance.Status.RequestNamespaces)
			}, timeout, interval).Should(Equal(1))

			By("Deleting the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())

			By("Check if the protected secret and configmap are deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "secret5", Namespace: requestNamespaceName}, &corev1.Secret{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "cm5", Namespace: requestNamespaceName}, &corev1.ConfigMap{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("Sharing the secret and configmap with private scope", func() {
		It("Should Status of the OperandBindInfo be initialized", func() {

			By("Prepare init resources for OperandBindInfo controller")
			Expect(k8sClient.Create(ctx, secret2)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap2)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, secret4Key, &corev1.Secret{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cm4Key, &corev1.ConfigMap{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking status of the OperandBindInfo")
			Eventually(func() operatorv1alpha1.BindInfoPhase {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return bindInfoInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.BindInfoWaiting))

			Eventually(func() int {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return len(bindInfoInstance.Status.RequestNamespaces)
			}, timeout, interval).Should(Equal(1))

			By("Deleting the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())
		})
	})

	Context("Sharing the not existing secret and configmap", func() {
		It("Should Status of the OperandBindInfo be initialized", func() {
			Eventually(func() bool {
				err := k8sClient.Get(ctx, secret4Key, &corev1.Secret{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, cm4Key, &corev1.ConfigMap{})
				return err != nil && errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Checking status of the OperandBindInfo")
			Eventually(func() operatorv1alpha1.BindInfoPhase {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return bindInfoInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.BindInfoWaiting))

			Eventually(func() int {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return len(bindInfoInstance.Status.RequestNamespaces)
			}, timeout, interval).Should(Equal(1))

			By("Deleting the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())
		})
	})

	Context("Updating the the secret and configmap with public scope", func() {
		It("Should Status of the OperandBindInfo be completed", func() {

			By("Prepare init resources for OperandBindInfo controller")
			Expect(k8sClient.Create(ctx, secret4)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap4)).Should(Succeed())

			Eventually(func() []byte {
				secret4 := &corev1.Secret{}
				err := k8sClient.Get(ctx, secret4Key, secret4)
				if err != nil {
					return []byte("")
				}
				return secret4.Data["test"]
			}, timeout, interval).Should(Equal([]byte("secret4")))
			Eventually(func() bool {
				cm4 := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, cm4Key, cm4)
				return err == nil && cm4.Data["test"] == "cm4"
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Create(ctx, secret1)).Should(Succeed())
			Expect(k8sClient.Create(ctx, configmap1)).Should(Succeed())

			By("Check if the public secret and configmap are shared")
			Eventually(func() []byte {
				secret4 := &corev1.Secret{}
				err := k8sClient.Get(ctx, secret4Key, secret4)
				if err != nil {
					return []byte("")
				}
				return secret4.Data["test"]
			}, timeout, interval).Should(Equal([]byte("secret1")))
			Eventually(func() bool {
				cm4 := &corev1.ConfigMap{}
				err := k8sClient.Get(ctx, cm4Key, cm4)
				return err == nil && cm4.Data["test"] == "cm1"
			}, timeout, interval).Should(BeTrue())

			By("Check status of the OperandBindInfo")
			Eventually(func() operatorv1alpha1.BindInfoPhase {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return bindInfoInstance.Status.Phase
			}, timeout, interval).Should(Equal(operatorv1alpha1.BindInfoCompleted))

			Eventually(func() int {
				bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
				Expect(k8sClient.Get(ctx, bindInfoKey, bindInfoInstance)).Should(Succeed())
				return len(bindInfoInstance.Status.RequestNamespaces)
			}, timeout, interval).Should(Equal(1))

			By("Deleting the OperandBindInfo")
			Expect(k8sClient.Delete(ctx, bindInfo)).Should(Succeed())
		})
	})
})
