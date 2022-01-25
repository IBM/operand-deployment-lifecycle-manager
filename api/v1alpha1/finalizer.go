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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureFinalizer ensures that the object's finalizer is included
// in the ObjectMeta Finalizers slice. If it already exists, no state change occurs.
// If it doesn't, the finalizer is appended to the slice.
func EnsureFinalizer(objectMeta *metav1.ObjectMeta, expectedFinalizer string) bool {
	// First check if the finalizer is already included in the object.
	for _, finalizer := range objectMeta.Finalizers {
		if finalizer == expectedFinalizer {
			return false
		}
	}

	objectMeta.Finalizers = append(objectMeta.Finalizers, expectedFinalizer)
	return true
}

// RemoveFinalizer removes the finalizer from the object's ObjectMeta.
func RemoveFinalizer(objectMeta *metav1.ObjectMeta, deletingFinalizer string) bool {
	outFinalizers := make([]string, 0)
	var changed bool
	for _, finalizer := range objectMeta.Finalizers {
		if finalizer == deletingFinalizer {
			changed = true
			continue
		}
		outFinalizers = append(outFinalizers, finalizer)
	}

	objectMeta.Finalizers = outFinalizers
	return changed
}
