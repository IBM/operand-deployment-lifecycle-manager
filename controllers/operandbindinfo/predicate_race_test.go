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
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
)

// TestBindablePredicatesConcurrentAccess tests for race conditions in predicate functions
// This test is designed to catch the concurrent map access issue reported in #69503
func TestBindablePredicatesConcurrentAccess(t *testing.T) {
	// Use the shared predicate function to avoid code duplication
	bindablePredicates := NewBindablePredicates()

	sharedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "test-ns",
			Labels: map[string]string{
				constant.OpbiTypeLabel: "copy",
				"test-label":           "test-value",
			},
		},
	}

	var wg sync.WaitGroup
	numReaders := 10
	iterationsPerGoroutine := 100

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < iterationsPerGoroutine; j++ {
				createEvent := event.CreateEvent{
					Object: sharedSecret,
				}
				result := bindablePredicates.CreateFunc(createEvent)
				if !result {
					t.Errorf("CreateFunc should return true for object with OpbiTypeLabel")
				}

				updateEvent := event.UpdateEvent{
					ObjectOld: sharedSecret,
					ObjectNew: sharedSecret,
				}
				result = bindablePredicates.UpdateFunc(updateEvent)
				if !result {
					t.Errorf("UpdateFunc should return true for object with OpbiTypeLabel")
				}

				deleteEvent := event.DeleteEvent{
					Object: sharedSecret,
				}
				result = bindablePredicates.DeleteFunc(deleteEvent)
				if !result {
					t.Errorf("DeleteFunc should return true for object with OpbiTypeLabel")
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for j := 0; j < numReaders*iterationsPerGoroutine; j++ {
			sharedSecret.Labels["modified-by"] = "goroutine"
			delete(sharedSecret.Labels, "modified-by")
		}
	}()

	wg.Wait()

	// If we reach here without panic or race detection, the test passes
	// The defensive copies prevent "fatal error: concurrent map read and map write"
}

// Made with Bob
