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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ocproute "github.com/openshift/api/route/v1"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/util/jsonpath"

	constant "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
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

// GetNoOLM returns boolean NoOLM enabled
func GetNoOLM() string {
	enabled, found := os.LookupEnv("NO_OLM")
	if !found {
		return "false"
	}
	return enabled
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

// CalculateHash calculates the hash value for single resource
func CalculateHash(input []byte) string {
	if len(input) == 0 {
		return ""
	}
	hashedData := sha256.Sum256(input)
	return hex.EncodeToString(hashedData[:7])
}

// CalculateResHashes calculates the hash for the existing cluster resource and the new template resource
func CalculateResHashes(fromCluster *unstructured.Unstructured, fromTemplate []byte) (string, string) {
	templateHash := CalculateHash(fromTemplate)

	if fromCluster != nil {
		clusterAnnos := fromCluster.GetAnnotations()
		clusterHash := ""
		if clusterAnnos != nil {
			clusterHash = clusterAnnos[constant.K8sHashedData]
		}
		return clusterHash, templateHash
	}
	return "", templateHash
}

// SetHashAnnotation sets the hash annotation in the object
func AddHashAnnotation(obj *unstructured.Unstructured, key, hash string, newAnnotations map[string]string) map[string]string {
	if newAnnotations == nil {
		newAnnotations = make(map[string]string)
	}
	newAnnotations[key] = hash
	return newAnnotations
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

func Differs(list []string, s string) bool {
	for _, v := range list {
		if v != s {
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

func EnsureAnnotationsForSecret(secret *corev1.Secret, annotatinos map[string]string) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	for k, v := range annotatinos {
		secret.Annotations[k] = v
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

func EnsureAnnotationForConfigMap(cm *corev1.ConfigMap, annotations map[string]string) {
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		cm.Annotations[k] = v
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
	return !equality.Semantic.DeepEqual(secret.GetLabels(), existingSecret.GetLabels()) ||
		!equality.Semantic.DeepEqual(secret.Type, existingSecret.Type) ||
		!equality.Semantic.DeepEqual(secret.Data, existingSecret.Data) ||
		!equality.Semantic.DeepEqual(secret.StringData, existingSecret.StringData) ||
		!equality.Semantic.DeepEqual(secret.GetOwnerReferences(), existingSecret.GetOwnerReferences())
}

func CompareConfigMap(configMap *corev1.ConfigMap, existingConfigMap *corev1.ConfigMap) (needUpdate bool) {
	return !equality.Semantic.DeepEqual(configMap.GetLabels(), existingConfigMap.GetLabels()) ||
		!equality.Semantic.DeepEqual(configMap.Data, existingConfigMap.Data) ||
		!equality.Semantic.DeepEqual(configMap.BinaryData, existingConfigMap.BinaryData) ||
		!equality.Semantic.DeepEqual(configMap.GetOwnerReferences(), existingConfigMap.GetOwnerReferences())
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

// FindSemantic checks if a given string contains a substring which is a valid semantic version, and returns that substring
func FindSemantic(input string) string {
	// Define the regular expression pattern for flexible semantic versions
	semverPattern := `\bv?(\d+)(\.\d+)?(\.\d+)?\b`

	// Compile the regular expression
	re := regexp.MustCompile(semverPattern)

	// Find the first match in the input string
	match := re.FindString(input)

	// if no match is found, return default minimal version
	if match == "" {
		return "v0.0.0"
	}

	return match
}

// FindMinSemver returns the minimal semantic version by given channel and semver list
func FindMinSemver(curChannel string, semverlList []string, semVerChannelMappings map[string]string) string {
	if len(semverlList) == 0 {
		return ""
	} else if !Contains(semverlList, FindSemantic(curChannel)) || curChannel == "" { // if current channel is not in the list or empty
		// change channel to minimal version in the list
		sort.Sort(semver.ByVersion(semverlList))
		return semVerChannelMappings[semverlList[0]]
	}
	return curChannel
}

// FindMaxSemver returns the maximal semantic version by given channel and semver list
func FindMaxSemver(curChannel string, semverlList []string, semVerChannelMappings map[string]string) string {
	if len(semverlList) == 0 {
		return ""
	} else if !Contains(semverlList, FindSemantic(curChannel)) || curChannel == "" { // if current channel is not in the list or empty
		// change channel to maximal version in the list
		sort.Sort(semver.ByVersion(semverlList))
		return semVerChannelMappings[semverlList[len(semverlList)-1]]
	}
	return curChannel
}

func FindSemverFromAnnotations(annotations map[string]string) ([]string, map[string]string) {
	var semverlList []string
	var semVerChannelMappings = make(map[string]string)
	reg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)
	for anno, channel := range annotations {
		prunedChannel := FindSemantic(channel)
		if reg.MatchString(anno) && semver.IsValid(prunedChannel) {
			semverlList = append(semverlList, prunedChannel)
			semVerChannelMappings[prunedChannel] = channel
		}
	}
	return semverlList, semVerChannelMappings
}

func FindMinSemverFromAnnotations(annotations map[string]string, curChannel string) string {
	// check request annotation in subscription, get all available channels
	semverlList, semVerChannelMappings := FindSemverFromAnnotations(annotations)
	return FindMinSemver(curChannel, semverlList, semVerChannelMappings)
}

// RemoveObjectField removes the field from the object according to the jsonPath
// jsonPath is a string that represents the path to the field in the object, always starts with "."
func RemoveObjectField(obj interface{}, jsonPath string) {
	// Remove the first dot in the beginning of the jsonPath
	jsonPath = strings.TrimPrefix(jsonPath, ".")
	fields := strings.Split(jsonPath, ".")

	// Check if the object is a map
	if objMap, ok := obj.(map[string]interface{}); ok {
		removeField(objMap, fields)
	}
}

// removeField removes the field from the object according to the jsonPath
func removeField(obj map[string]interface{}, fields []string) {
	// Check if fields is in the format of one item in the list. For example "container[0]"
	if strings.Contains(fields[0], "[") {
		// Get the field name and the index
		field := strings.Split(fields[0], "[")[0]
		index, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(fields[0], field+"["), "]"))
		// Check if the field is a list
		if _, ok := obj[field].([]interface{}); !ok {
			return
		}
		// Check if the index is out of range
		if index >= len(obj[field].([]interface{})) {
			return
		}
		// Remove the value from the list
		removeField(obj[field].([]interface{})[index].(map[string]interface{}), fields[1:])
		return
	}
	// Check if the field is the last field in the list
	if len(fields) == 1 {
		delete(obj, fields[0])
		return
	}
	// Check if the field is a map
	if _, ok := obj[fields[0]].(map[string]interface{}); !ok {
		return
	}
	// Remove the value from the map
	removeField(obj[fields[0]].(map[string]interface{}), fields[1:])
}

// AddObjectField adds the field to the object according to the jsonPath
// jsonPath is a string that represents the path to the field in the object, always starts with "."
func AddObjectField(obj interface{}, jsonPath string, value interface{}) {
	// Remove the first dot in the beginning of the jsonPath if it exists
	jsonPath = strings.TrimPrefix(jsonPath, ".")
	fields := strings.Split(jsonPath, ".")

	// Check if the object is a map
	if objMap, ok := obj.(map[string]interface{}); ok {
		addField(objMap, fields, value)
	}
}

func addField(obj map[string]interface{}, fields []string, value interface{}) {
	// Check if fields is in the format of one item in the list. For example "container[0]"
	if strings.Contains(fields[0], "[") {
		// Get the field name and the index
		field := strings.Split(fields[0], "[")[0]
		index, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(fields[0], field+"["), "]"))
		// Check if the field is a list
		if _, ok := obj[field].([]interface{}); !ok {
			obj[field] = make([]interface{}, 0)
		}
		// Check if the index is out of range
		if index >= len(obj[field].([]interface{})) {
			obj[field] = append(obj[field].([]interface{}), make(map[string]interface{}))
		}
		// Add the value to the list
		addField(obj[field].([]interface{})[index].(map[string]interface{}), fields[1:], value)
		return
	}
	// Check if the field is the last field in the list
	if len(fields) == 1 {
		obj[fields[0]] = value
		return
	}
	// Check if the field is a map
	if _, ok := obj[fields[0]].(map[string]interface{}); !ok {
		obj[fields[0]] = make(map[string]interface{})
	}
	// Add the value to the map
	addField(obj[fields[0]].(map[string]interface{}), fields[1:], value)
}

func GetFirstNCharacter(str string, n int) string {
	if n >= len(str) {
		return str
	}
	return str[:n]
}
