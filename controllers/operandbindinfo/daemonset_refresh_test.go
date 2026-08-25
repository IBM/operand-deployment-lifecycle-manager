//
// Copyright 2026 IBM Corporation
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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

const (
	refreshTestNamespace    = "test-namespace"
	refreshTestResourceName = "shared-secret"
	refreshTestResourceType = "secret"
)

func TestRefreshPodsSkipsDaemonSetsWithoutPermission(t *testing.T) {
	manageDaemonSets := true
	objects := refreshTestWorkloads()
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, &manageDaemonSets, func(context.Context, string) (bool, string) {
		return false, "update permission was denied"
	})

	if err := reconciler.refreshPods(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPods returned an error when DaemonSet permission was denied: %v", err)
	}

	assertRestarted(t, k8sClient, &appsv1.Deployment{}, "deployment", true)
	assertRestarted(t, k8sClient, &appsv1.StatefulSet{}, "statefulset", true)
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", false)
}

func TestRefreshPodsFromDaemonSetHonorsOptOut(t *testing.T) {
	manageDaemonSets := false
	permissionCheckCalled := false
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, &manageDaemonSets, func(context.Context, string) (bool, string) {
		permissionCheckCalled = true
		return true, ""
	})

	if err := reconciler.refreshPodsFromDaemonSet(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPodsFromDaemonSet returned an error when management was disabled: %v", err)
	}
	if permissionCheckCalled {
		t.Fatal("permission checker was called even though DaemonSet management was disabled")
	}
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", false)
}

func TestRefreshPodsFromDaemonSetWithPermission(t *testing.T) {
	manageDaemonSets := true
	objects := []client.Object{refreshTestDaemonSet()}
	reconciler, k8sClient := newRefreshTestReconciler(t, objects, &manageDaemonSets, func(context.Context, string) (bool, string) {
		return true, ""
	})

	if err := reconciler.refreshPodsFromDaemonSet(refreshTestNamespace, refreshTestResourceName, refreshTestResourceType); err != nil {
		t.Fatalf("refreshPodsFromDaemonSet returned an error with permission: %v", err)
	}
	assertRestarted(t, k8sClient, &appsv1.DaemonSet{}, "daemonset", true)
}

func newRefreshTestReconciler(t *testing.T, objects []client.Object, manageDaemonSets *bool, permissionChecker func(context.Context, string) (bool, string)) (*Reconciler, client.Client) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add apps/v1 to the test scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	return &Reconciler{
		ODLMOperator: &deploy.ODLMOperator{
			Client: k8sClient,
			Reader: k8sClient,
		},
		ManageDaemonSets:           manageDaemonSets,
		daemonSetPermissionChecker: permissionChecker,
	}, k8sClient
}

func refreshTestWorkloads() []client.Object {
	objectMeta := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{
			Name:      name,
			Namespace: refreshTestNamespace,
			Labels: map[string]string{
				constant.BindInfoRefreshLabel: "enabled",
			},
			Annotations: map[string]string{
				"bindinfoRefresh/" + refreshTestResourceType: refreshTestResourceName,
			},
		}
	}

	return []client.Object{
		&appsv1.Deployment{ObjectMeta: objectMeta("deployment")},
		&appsv1.StatefulSet{ObjectMeta: objectMeta("statefulset")},
		&appsv1.DaemonSet{ObjectMeta: objectMeta("daemonset")},
	}
}

func refreshTestDaemonSet() *appsv1.DaemonSet {
	return refreshTestWorkloads()[2].(*appsv1.DaemonSet)
}

func assertRestarted(t *testing.T, k8sClient client.Client, workload client.Object, name string, expected bool) {
	t.Helper()

	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: refreshTestNamespace}, workload); err != nil {
		t.Fatalf("failed to get %T %s: %v", workload, name, err)
	}
	_, restarted := workload.GetAnnotations()["bindinfo/restartTime"]
	if restarted != expected {
		t.Fatalf("expected %T %s restarted=%t, got %t", workload, name, expected, restarted)
	}
}
