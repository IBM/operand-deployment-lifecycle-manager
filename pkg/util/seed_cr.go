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

package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// InitInstance creates resource from a yaml file
func InitInstance(yamlPath string, mgr manager.Manager) error {
	yamlFile, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", yamlPath, err)
	}

	kubeClient := mgr.GetClient()

	obj := &unstructured.Unstructured{}
	jsonSpec, err := yaml.YAMLToJSON(yamlFile)
	if err != nil {
		return fmt.Errorf("could not convert yaml file to json: %v", err)
	}
	if err := obj.UnmarshalJSON(jsonSpec); err != nil {
		return fmt.Errorf("could not unmarshal resource: %v", err)
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "ibm-common-services"
		}
		obj.SetNamespace(namespace)
	}

	klog.V(4).Info("Generating", name, "in namespace", namespace)

	err = kubeClient.Create(context.TODO(), obj)

	if errors.IsAlreadyExists(err) {
		klog.V(2).Info("CR exists in the cluster")
	} else if err != nil {
		return fmt.Errorf("could not Create resource: %v", err)
	} else {
		klog.V(2).Info("CR was created successfully")
	}

	return nil
}
