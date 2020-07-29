//
// Copyright 2020 IBM Corporation
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

package helpers

import (
	goctx "context"
	"fmt"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/IBM/operand-deployment-lifecycle-manager/test/config"
)

// CreateNamespace is used to create a new namespace for test
func CreateNamespace(f *framework.Framework, ctx *framework.TestCtx, name string) error {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := f.Client.Create(goctx.TODO(), obj, &framework.CleanupOptions{TestContext: ctx, Timeout: config.CleanupTimeout, RetryInterval: config.CleanupRetry})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// DeleteNamespace is delete the test namespace
func DeleteNamespace(f *framework.Framework, name string) error {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := f.Client.Delete(goctx.TODO(), obj); err != nil {
		return err
	}

	return nil
}

// RetrieveSecret is get a secret
func RetrieveSecret(f *framework.Framework, name, namespace string, delete bool) (*corev1.Secret, error) {
	obj := &corev1.Secret{}
	// Get Secret
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				if delete {
					return true, nil
				}
				fmt.Println("    --- Waiting for Secret copied ...")
				return false, nil
			}
			return false, err
		}
		if delete {
			fmt.Println("    --- Waiting for Secret deleted ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// RetrieveConfigmap is get a configmap
func RetrieveConfigmap(f *framework.Framework, name, namespace string, delete bool) (*corev1.ConfigMap, error) {
	obj := &corev1.ConfigMap{}
	// Get ConfigMap
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				if delete {
					return true, nil
				}
				fmt.Println("    --- Waiting for ConfigMap copied ...")
				return false, nil
			}
			return false, err
		}
		if delete {
			fmt.Println("    --- Waiting for ConfigMap deleted ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}

// RetrieveSubscription is get a Subscription
func RetrieveSubscription(f *framework.Framework, name, namespace string, delete bool) (*olmv1alpha1.Subscription, error) {
	obj := &olmv1alpha1.Subscription{}
	// Get ConfigMap
	if err := utilwait.PollImmediate(config.WaitForRetry, config.WaitForTimeout, func() (done bool, err error) {
		if err = f.Client.Get(goctx.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
			if errors.IsNotFound(err) {
				if delete {
					return true, nil
				}
				fmt.Println("    --- Waiting for Subscription created ...")
				return false, nil
			}
			return false, err
		}
		if delete {
			fmt.Println("    --- Waiting for Subscription deleted ...")
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, err
	}
	return obj, nil
}
