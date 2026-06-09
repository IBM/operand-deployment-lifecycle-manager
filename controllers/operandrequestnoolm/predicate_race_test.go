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
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
)

// TestReferencePredicatesConcurrentAccess tests for race conditions in predicate functions
// This test is designed to catch the concurrent map access issue reported in #69503
func TestReferencePredicatesConcurrentAccess(t *testing.T) {
	// Use the shared predicate function to avoid code duplication
	ReferencePredicates := NewReferencePredicates()

	sharedCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-ns",
			Labels: map[string]string{
				constant.ODLMWatchedLabel: "true",
				"test-label":              "test-value",
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
					Object: sharedCM,
				}
				_ = ReferencePredicates.Create(createEvent)

				updateEvent := event.UpdateEvent{
					ObjectOld: sharedCM,
					ObjectNew: sharedCM,
				}
				_ = ReferencePredicates.Update(updateEvent)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for j := 0; j < numReaders*iterationsPerGoroutine; j++ {
			sharedCM.Labels["modified-by"] = "goroutine"
			delete(sharedCM.Labels, "modified-by")
		}
	}()

	wg.Wait()

	// If we reach here without panic or race detection, the test passes
	// The defensive copies prevent "fatal error: concurrent map read and map write"
}

// TestReferencePredicatesLogic tests the predicate logic
func TestReferencePredicatesLogic(t *testing.T) {
	ReferencePredicates := NewReferencePredicates()

	tests := []struct {
		name     string
		labels   map[string]string
		expected bool
	}{
		{
			name:     "nil labels",
			labels:   nil,
			expected: false,
		},
		{
			name:     "no ODLM label",
			labels:   map[string]string{"other": "value"},
			expected: false,
		},
		{
			name:     "ODLM label true",
			labels:   map[string]string{constant.ODLMWatchedLabel: "true"},
			expected: true,
		},
		{
			name: "ODLM label true with copy type",
			labels: map[string]string{
				constant.ODLMWatchedLabel: "true",
				constant.OpbiTypeLabel:    "copy",
			},
			expected: false,
		},
		{
			name: "ODLM label true with non-copy type",
			labels: map[string]string{
				constant.ODLMWatchedLabel: "true",
				constant.OpbiTypeLabel:    "original",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-ns",
					Labels:    tt.labels,
				},
			}

			createEvent := event.CreateEvent{
				Object: cm,
			}

			result := ReferencePredicates.Create(createEvent)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for labels %v", tt.expected, result, tt.labels)
			}
		})
	}
}

// Made with Bob
