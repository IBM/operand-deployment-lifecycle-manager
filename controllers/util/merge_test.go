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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DeepMerge", func() {

	Context("Deep Merge two JSON files", func() {
		It("Should two JSON files get deep merged", func() {
			defaultJSON := `{"greetings":{"first":"hi","second":"hello"},"name":"John"}`
			changedJSON := `{"greetings":{"first":"hey"},"name":"Jane"}`
			resultJSON := `{"greetings":{"first":"hey","second":"hello"},"name":"Jane"}`

			changedJSONDecoded := MergeCR([]byte(defaultJSON), []byte(changedJSON))

			mergedJSON, err := json.Marshal(changedJSONDecoded)
			Expect(err).NotTo(HaveOccurred())

			Expect(mergedJSON).Should(Equal([]byte(resultJSON)))
		})
	})

	Context("Deep Merge two JSON files with list", func() {
		It("Should two JSON files get deep merged", func() {
			defaultJSON := `{"age":30,"cars":["Ford","BMW","Fiat"],"bicycle":["Giant"],"name":"John"}`
			changedJSON := `{"age":13,"cars":["Benz","BMW","Fiat"],"plane":["Boeing"],"name":"Jane"}`
			resultJSON := `{"age":13,"bicycle":["Giant"],"cars":["Benz","BMW","Fiat"],"name":"Jane","plane":["Boeing"]}`

			changedJSONDecoded := MergeCR([]byte(defaultJSON), []byte(changedJSON))

			mergedJSON, err := json.Marshal(changedJSONDecoded)
			Expect(err).NotTo(HaveOccurred())

			Expect(mergedJSON).Should(Equal([]byte(resultJSON)))
		})
	})
})
