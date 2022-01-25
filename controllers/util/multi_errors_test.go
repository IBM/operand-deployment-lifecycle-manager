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
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Multiple error list", func() {

	Context("Combine multiple errors into one instance", func() {
		It("Should return one instance includes multiple error message", func() {

			By("Initialize a new multiple error")
			merr := &MultiErr{}

			merr.Add(errors.New("this is the First error"))
			merr.Add(errors.New("this is the Second error"))

			errMessage := `the following errors occurred:
  - this is the First error
  - this is the Second error`
			Expect(merr.Error()).Should(Equal(errMessage))
		})
	})

})
