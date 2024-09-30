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

package util

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Get environmental variables", func() {

	Context("Check environmental variables", func() {
		It("Should get OPERATOR_NAMESPACE", func() {
			testNs := "system"
			err := os.Setenv("OPERATOR_NAMESPACE", testNs)
			Expect(err).NotTo(HaveOccurred())

			ns := GetOperatorNamespace()
			Expect(ns).Should(Equal(testNs))
		})

		It("Should get WATCH_NAMESPACE", func() {

			operatorNs := "system"
			err := os.Setenv("OPERATOR_NAMESPACE", operatorNs)
			Expect(err).NotTo(HaveOccurred())

			ns := GetWatchNamespace()
			Expect(ns).Should(Equal(operatorNs))

			watchNs := "system,cloudpak1"
			err = os.Setenv("WATCH_NAMESPACE", watchNs)
			Expect(err).NotTo(HaveOccurred())

			ns = GetWatchNamespace()
			Expect(ns).Should(Equal(watchNs))
		})

		It("Should get INSTALL_SCOPE", func() {
			scope := "namespaced"
			err := os.Setenv("INSTALL_SCOPE", scope)
			Expect(err).NotTo(HaveOccurred())

			ns := GetInstallScope()
			Expect(ns).Should(Equal(scope))
		})

		It("Should string slice be equal", func() {
			a := []string{"apple", "pine", "pineapple"}
			b := []string{"apple", "pineapple", "pine"}
			Expect(StringSliceContentEqual(a, b)).Should(BeTrue())
			c := []string{"apple", "pear", "pineapple"}
			d := []string{"apple", "pineapple", "pine"}
			Expect(StringSliceContentEqual(c, d)).Should(BeFalse())
		})
	})
})

var _ = Describe("Contains", func() {
	It("Should return true if the list contains the string", func() {
		list := []string{"apple", "banana", "cherry"}
		s := "banana"
		Expect(Contains(list, s)).Should(BeTrue())
	})

	It("Should return false if the list does not contain the string", func() {
		list := []string{"apple", "banana", "cherry"}
		s := "orange"
		Expect(Contains(list, s)).Should(BeFalse())
	})

	It("Should return false if the list is empty", func() {
		list := []string{}
		s := "apple"
		Expect(Contains(list, s)).Should(BeFalse())
	})
})

var _ = Describe("Differs", func() {
	It("Should return true if the list contains a different string", func() {
		list := []string{"apple", "banana", "cherry"}
		s := "banana"
		Expect(Differs(list, s)).Should(BeTrue())
	})

	It("Should return false if the list contains only the same string", func() {
		list := []string{"apple", "apple", "apple"}
		s := "apple"
		Expect(Differs(list, s)).Should(BeFalse())
	})

	It("Should return false if the list is empty", func() {
		list := []string{}
		s := "apple"
		Expect(Differs(list, s)).Should(BeFalse())
	})
})

var _ = Describe("FindSemantic", func() {
	It("Should return the semantic vX version substring", func() {
		input := "stable-v1"
		expected := "v1"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})

	It("Should return the semantic vX.Y version substring", func() {
		input := "fast-v1.2"
		expected := "v1.2"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})

	It("Should return the original semantic vX.Y version substring", func() {
		input := "v1.2"
		expected := "v1.2"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})

	It("Should return the semantic vX.Y.Z version substring", func() {
		input := "stable-v1.2.3"
		expected := "v1.2.3"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})

	It("Should return an empty string if no semantic version is found", func() {
		input := "This is a test string without a semantic version"
		expected := "v0.0.0"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})

	It("Should return the first semantic version substring", func() {
		input := "This is a test v1.2.3 string with v0.1.0 multiple semantic versions"
		expected := "v1.2.3"
		Expect(FindSemantic(input)).Should(Equal(expected))
	})
})

var _ = Describe("FindMinSemverFromAnnotations", func() {
	It("Should return the minimal semantic version from annotations", func() {
		annotations := map[string]string{
			"namespace-a.common-service.operator-a/request": "stable",
			"namespace-b.common-service.operator-b/request": "stable-v1.0",
			"namespace-c.common-service.operator-c/request": "stable-v1.1.0",
			"namespace-d.common-service.operator-d/request": "stable-v1.2.0",
		}
		curChannel := "stable-v1.3.0"
		expected := "stable"
		Expect(FindMinSemverFromAnnotations(annotations, curChannel)).Should(Equal(expected))
	})

	It("Should return the minimal semantic version from annotations", func() {
		annotations := map[string]string{
			"namespace-b.common-service.operator-b/request": "v4.0",
			"namespace-c.common-service.operator-c/request": "v4.1",
			"namespace-d.common-service.operator-d/request": "v4.2",
		}
		curChannel := "v3"
		expected := "v4.0"
		Expect(FindMinSemverFromAnnotations(annotations, curChannel)).Should(Equal(expected))
	})

	It("Should return the current channel if it exists in annotations", func() {
		annotations := map[string]string{
			"namespace-a.common-service.operator-a/request": "stable",
			"namespace-b.common-service.operator-b/request": "stable-v1.0",
			"namespace-c.common-service.operator-c/request": "stable-v1.1.0",
			"namespace-d.common-service.operator-d/request": "stable-v1.2",
		}
		curChannel := "stable-v1.1.0"
		expected := "stable-v1.1.0"
		Expect(FindMinSemverFromAnnotations(annotations, curChannel)).Should(Equal(expected))
	})

	It("Should return the current channel if it exists in annotations", func() {
		annotations := map[string]string{
			"namespace-a.common-service.operator-a/request": "stable",
			"namespace-b.common-service.operator-b/request": "v1.0",
			"namespace-c.common-service.operator-c/request": "v2.0",
			"namespace-d.common-service.operator-d/request": "v2.0",
		}
		curChannel := "stable"
		expected := "stable"
		Expect(FindMinSemverFromAnnotations(annotations, curChannel)).Should(Equal(expected))
	})

	It("Should return an empty string if no valid semantic versions are found", func() {
		annotations := map[string]string{
			"namespace-a.common-service.operator-a/config": "stable",
			"namespace-b.common-service.operator-b/config": "stable-v1.0",
			"namespace-c.common-service.operator-c/config": "stable-v1.1.0",
			"namespace-d.common-service.operator-d/config": "stable-v1.2",
		}
		curChannel := "stable-v2.0.0"
		expected := ""
		Expect(FindMinSemverFromAnnotations(annotations, curChannel)).Should(Equal(expected))
	})
})

var _ = Describe("FindMaxSemver", func() {
	It("Should return the maximal semantic version from the given channel and semver list", func() {
		curChannel := "stable-v1.2.0"
		semverList := []string{"v1.0.0", "v1.1.0", "v1.3.0", "v1.4.0"}
		semVerChannelMappings := map[string]string{
			"v1.0.0": "stable-v1.0.0",
			"v1.1.0": "stable-v1.1.0",
			"v1.3.0": "stable-v1.3.0",
			"v1.4.0": "stable-v1.4.0",
		}
		expected := "stable-v1.4.0"
		Expect(FindMaxSemver(curChannel, semverList, semVerChannelMappings)).Should(Equal(expected))
	})

	It("Should return the current channel if it exists in the semver list", func() {
		curChannel := "stable-v1.2.0"
		semverList := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"}
		semVerChannelMappings := map[string]string{
			"v1.0.0": "stable-v1.0.0",
			"v1.1.0": "stable-v1.1.0",
			"v1.2.0": "stable-v1.2.0",
			"v1.3.0": "stable-v1.3.0",
		}
		expected := "stable-v1.2.0"
		Expect(FindMaxSemver(curChannel, semverList, semVerChannelMappings)).Should(Equal(expected))
	})

	It("Should return an empty string if the semver list is empty", func() {
		curChannel := "stable-v1.2.0"
		semverList := []string{}
		semVerChannelMappings := map[string]string{}
		expected := ""
		Expect(FindMaxSemver(curChannel, semverList, semVerChannelMappings)).Should(Equal(expected))
	})

	It("Should return an empty string if the semVerChannelMappings has no valid mappings", func() {
		curChannel := "stable-v1.2.0"
		semverList := []string{"v1.4.0", "v1.5.0", "v1.6.0"}
		semVerChannelMappings := map[string]string{
			"v1.0.0": "stable-v1.0.0",
			"v1.1.0": "stable-v1.1.0",
			"v1.3.0": "stable-v1.3.0",
		}
		expected := ""
		Expect(FindMaxSemver(curChannel, semverList, semVerChannelMappings)).Should(Equal(expected))
	})
})

var _ = Describe("RemoveObjectField", func() {
	It("Should remove the specified field from the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		RemoveObjectField(obj.Object, ".metadata.namespace")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should remove nested fields from the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]interface{}{
						"app": "myapp",
					},
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		RemoveObjectField(obj.Object, ".metadata.labels.app")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
					"labels":    map[string]interface{}{},
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should do nothing if the field does not exist", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		RemoveObjectField(obj.Object, ".metadata.labels.app")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should remove field and all its nested fields from the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]interface{}{
						"app": "myapp",
					},
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		RemoveObjectField(obj.Object, ".metadata")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})
})

var _ = Describe("AddObjectField", func() {
	It("Should add a new field to the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		AddObjectField(obj.Object, ".metadata.namespace", "default")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should add a nested field to the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		AddObjectField(obj.Object, ".metadata.labels.app", "myapp")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
					"labels": map[string]interface{}{
						"app": "myapp",
					},
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should overwrite an existing field in the object", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
		}

		AddObjectField(obj.Object, ".spec.replicas", 5)

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"replicas": 5,
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})

	It("Should create nested maps if they do not exist", func() {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
		}

		AddObjectField(obj.Object, ".spec.template.spec.containers[0].name", "nginx")

		expected := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name": "nginx",
								},
							},
						},
					},
				},
			},
		}

		Expect(obj).Should(Equal(expected))
	})
})
