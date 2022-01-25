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
