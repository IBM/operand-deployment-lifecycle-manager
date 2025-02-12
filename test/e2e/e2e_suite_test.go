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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestODLME2E(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t,
		"Operand Deployment Lifecycle Manager TestSuite")
}

var _ = BeforeSuite(func(done Done) {

	// Initialize the test suite
	initSuite()

	// End your controllers test logic
	By("Creating the Namespace for the first OperandRequest")
	createTestNamespace(OperandRequestNamespace1)
	By("Creating the Namespace for the second OperandRequest")
	createTestNamespace(OperandRequestNamespace2)
	By("Creating the Namespace for OperandRegistry")
	createTestNamespace(OperandRegistryNamespace)
	By("Creating the Namespace for Operators")
	createTestNamespace(OperatorNamespace)

	close(done)

}, 600)

var _ = AfterSuite(func() {

	By("Delete the Namespace for the first OperandRequest")
	deleteTestNamespace(OperandRequestNamespace1)
	By("Delete the Namespace for the second OperandRequest")
	deleteTestNamespace(OperandRequestNamespace2)
	By("Delete the Namespace for OperandRegistry")
	deleteTestNamespace(OperandRegistryNamespace)
	By("Delete the Namespace for Operators")
	deleteTestNamespace(OperatorNamespace)

	// Close the test suite
	tearDownSuite()
}, 1200)
