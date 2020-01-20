package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// InitInstance creates resource from a yaml file
func InitInstance(yamlPath string, mgr manager.Manager) error {
	name := "common-service"
	namespace := os.Getenv("WATCH_NAMESPACE")
	logger := log.WithValues("Namespace", namespace, "Name", name)

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
	obj.SetNamespace(namespace)

	err = kubeClient.Create(context.TODO(), obj)

	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("could not Create resource: %v", err)
	}

	logger.Info("CR was created successfully")

	return nil
}
