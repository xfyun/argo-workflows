package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	apiv1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-workflows/v3/pkg/plugins/spec"
)

func loadPluginManifest(pluginDir string) (*spec.Plugin, error) {
	manifest, err := os.ReadFile(filepath.Join(pluginDir, "plugin.yaml"))
	if err != nil {
		return nil, err
	}
	p := &spec.Plugin{}
	err = yaml.UnmarshalStrict(manifest, p)
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(pluginDir, "server.*"))
	if err != nil {
		return nil, err
	}
	if len(files) == 1 {
		code, err := os.ReadFile(files[0])
		if err != nil {
			return nil, err
		}
		p.Spec.Sidecar.Container.Args = []string{string(code)}
	} else if len(files) > 1 {
		return nil, fmt.Errorf("plugin %s has more than one server.* file", p.Name)
	}
	return p, p.Validate()
}

func addHeader(x []byte, h string) []byte {
	return []byte(fmt.Sprintf("%s\n%s", h, string(x)))
}

func addCodegenHeader(x []byte) []byte {
	return addHeader(x, "# This is an auto-generated file. DO NOT EDIT")
}

func saveConfigMap(cm *apiv1.ConfigMap, pluginDir string) (string, error) {
	data, err := yaml.Marshal(cm)
	if err != nil {
		return "", err
	}
	cmPath := filepath.Join(pluginDir, fmt.Sprintf("%s-configmap.yaml", cm.Name))
	err = os.WriteFile(cmPath, addCodegenHeader(data), 0666)
	return cmPath, err
}
