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
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

// TestBindablePredicatesConcurrentAccess verifies predicate functions can be
// called concurrently without mutating shared event objects.
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

	wg.Wait()
}

func TestToOpbiRequestUsesEventLabelsWhenObjectDeleted(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add core scheme: %v", err)
	}
	reconciler := &Reconciler{
		ODLMOperator: &deploy.ODLMOperator{
			Reader: fake.NewClientBuilder().WithScheme(scheme).Build(),
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deleted-secret",
			Namespace: "request-ns",
			Labels: map[string]string{
				constant.OpbiTypeLabel:       "copy",
				"bind-ns.bind-name/bindinfo": "true",
			},
		},
	}

	requests := reconciler.toOpbiRequest(context.Background(), secret)

	if len(requests) != 1 {
		t.Fatalf("expected one reconcile request, got %d", len(requests))
	}
	if requests[0].Name != "bind-name" || requests[0].Namespace != "bind-ns" {
		t.Fatalf("unexpected reconcile request: %#v", requests[0])
	}
}
