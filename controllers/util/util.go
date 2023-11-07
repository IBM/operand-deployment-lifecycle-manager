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
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ocproute "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/jsonpath"
)

type TemplateValueRef struct {
	Required        bool              `json:"required,omitempty"`
	Default         *DefaultObjectRef `json:"default,omitempty"`
	ConfigMapKeyRef *ConfigMapRef     `json:"configMapKeyRef,omitempty"`
	SecretRef       *SecretRef        `json:"secretKeyRef,omitempty"`
	// RouteRef        *RouteRef         `json:"routePathRef,omitempty"`
	ObjectRef *ObjectRef `json:"objectRef,omitempty"`
}

type DefaultObjectRef struct {
	Required        bool          `json:"required,omitempty"`
	ConfigMapKeyRef *ConfigMapRef `json:"configMapKeyRef,omitempty"`
	SecretRef       *SecretRef    `json:"secretKeyRef,omitempty"`
	// RouteRef        *RouteRef     `json:"routePathRef,omitempty"`
	ObjectRef    *ObjectRef `json:"objectRef,omitempty"`
	DefaultValue string     `json:"defaultValue,omitempty"`
}

type ConfigMapRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

type SecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

type ObjectRef struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Path       string `json:"path"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

// GetOperatorNamespace returns the Namespace of the operator
func GetOperatorNamespace() string {
	ns, found := os.LookupEnv("OPERATOR_NAMESPACE")
	if !found {
		return ""
	}
	return ns
}

// GetWatchNamespace returns the Namespace of the operator
func GetWatchNamespace() string {
	ns, found := os.LookupEnv("WATCH_NAMESPACE")
	if !found {
		return GetOperatorNamespace()
	}
	return ns
}

// GetInstallScope returns the scope of the installation
func GetInstallScope() string {
	ns, found := os.LookupEnv("INSTALL_SCOPE")
	if !found {
		return "cluster"
	}
	return ns
}

func GetIsolatedMode() bool {
	isEnable, found := os.LookupEnv("ISOLATED_MODE")
	if !found || isEnable != "true" {
		return false
	}
	return true
}

func GetoperatorCheckerMode() bool {
	isEnable, found := os.LookupEnv("OPERATORCHECKER_MODE")
	if found && isEnable == "false" {
		return true
	}
	return false
}

// ResourceExists returns true if the given resource kind exists
// in the given api groupversion
func ResourceExists(dc discovery.DiscoveryInterface, apiGroupVersion, kind string) (bool, error) {
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}
	for _, apiList := range apiLists {
		if apiList.GroupVersion == apiGroupVersion {
			for _, r := range apiList.APIResources {
				if r.Kind == kind {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// StringSliceContentEqual checks if the contant from two string slice are the same
func StringSliceContentEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// WaitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

// ResourceNamespaced returns true if the given resource is namespaced
func ResourceNamespaced(dc discovery.DiscoveryInterface, apiGroupVersion, kind string) (bool, error) {
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return false, err
	}
	for _, apiList := range apiLists {
		if apiList.GroupVersion == apiGroupVersion {
			for _, r := range apiList.APIResources {
				if r.Kind == kind {
					return r.Namespaced, nil
				}
			}
		}
	}
	return false, nil
}

func CompareChannelVersion(v1, v2 string) (v1IsLarger bool, err error) {
	_, v1Cut, isExist := strings.Cut(v1, "v")
	if !isExist {
		v1Cut = "0.0"
	}
	v1Slice := strings.Split(v1Cut, ".")
	if len(v1Slice) == 1 {
		v1Cut = v1Cut + ".0"
	}

	_, v2Cut, isExist := strings.Cut(v2, "v")
	if !isExist {
		v1Cut = "0.0"
	}
	v2Slice := strings.Split(v2Cut, ".")
	if len(v2Slice) == 1 {
		v2Cut = v2Cut + ".0"
	}

	v1Slice = strings.Split(v1Cut, ".")
	v2Slice = strings.Split(v2Cut, ".")
	for index := range v1Slice {
		v1SplitInt, e1 := strconv.Atoi(v1Slice[index])
		if e1 != nil {
			return false, e1
		}
		v2SplitInt, e2 := strconv.Atoi(v2Slice[index])
		if e2 != nil {
			return false, e2
		}

		if v1SplitInt > v2SplitInt {
			return true, nil
		} else if v1SplitInt == v2SplitInt {
			continue
		} else {
			return false, nil
		}
	}
	return false, nil
}

func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func ObjectToNewUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("could not convert Object to Unstructured resource: %v", err)
	}
	newUnstr := &unstructured.Unstructured{}
	newUnstr.SetUnstructuredContent(content)
	return newUnstr, nil
}

func EnsureLabelsForSecret(secret *corev1.Secret, labels map[string]string) {
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	for k, v := range labels {
		secret.Labels[k] = v
	}
}

func EnsureLabelsForConfigMap(cm *corev1.ConfigMap, labels map[string]string) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	for k, v := range labels {
		cm.Labels[k] = v
	}
}

func EnsureLabelsForRoute(r *ocproute.Route, labels map[string]string) {
	if r.Labels == nil {
		r.Labels = make(map[string]string)
	}
	for k, v := range labels {
		r.Labels[k] = v
	}
}

func EnsureLabelsForService(s *corev1.Service, labels map[string]string) {
	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	for k, v := range labels {
		s.Labels[k] = v
	}
}

func CompareSecret(secret *corev1.Secret, existingSecret *corev1.Secret) (needUpdate bool) {
	return !equality.Semantic.DeepEqual(secret.GetLabels(), existingSecret.GetLabels()) || !equality.Semantic.DeepEqual(secret.Type, existingSecret.Type) || !equality.Semantic.DeepEqual(secret.Data, existingSecret.Data) || !equality.Semantic.DeepEqual(secret.StringData, existingSecret.StringData)
}

func CompareConfigMap(configMap *corev1.ConfigMap, existingConfigMap *corev1.ConfigMap) (needUpdate bool) {
	return !equality.Semantic.DeepEqual(configMap.GetLabels(), existingConfigMap.GetLabels()) || !equality.Semantic.DeepEqual(configMap.Data, existingConfigMap.Data) || !equality.Semantic.DeepEqual(configMap.BinaryData, existingConfigMap.BinaryData)
}

// SanitizeObjectString takes a string, i.e. .metadata.namespace, and a K8s object
// and returns a string got from K8s object. The required string
// is sanitized because the values are YAML fields in a K8s object.
// Ensures that:
//  1. the field actually exists, otherwise returns an error
//  2. extracts the value from the K8s Service's field, the value will be
//     stringified
func SanitizeObjectString(jsonPath string, data interface{}) (string, error) {
	jpath := jsonpath.New("sanitizeObjectData")
	stringParts := strings.Split(jsonPath, "+")
	sanitized := ""
	for _, s := range stringParts {
		actual := s
		if strings.HasPrefix(s, ".") {
			if len(s) > 1 {
				if err := jpath.Parse("{" + s + "}"); err != nil {
					return "", err
				}
				buf := new(bytes.Buffer)
				if err := jpath.Execute(buf, data); err != nil {
					return "", err
				}
				actual = buf.String()
			}
		}
		sanitized += actual
	}
	return sanitized, nil
}
