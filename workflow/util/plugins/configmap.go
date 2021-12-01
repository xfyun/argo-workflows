package plugin

import (
	"fmt"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-workflows/v3/pkg/plugins/spec"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

func ToConfigMap(p *spec.Plugin) (*apiv1.ConfigMap, error) {
	data, err := yaml.Marshal(p.Spec.Sidecar.Container)
	if err != nil {
		return nil, err
	}
	cm := &apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-executor-plugin", p.Name),
			Annotations: map[string]string{},
			Labels: map[string]string{
				common.LabelKeyConfigMapType: p.Kind,
			},
		},
		Data: map[string]string{
			"sidecar.address":   p.Spec.Sidecar.Address,
			"sidecar.container": string(data),
		},
	}
	for k, v := range p.Annotations {
		cm.Annotations[k] = v
	}
	for k, v := range p.Labels {
		cm.Labels[k] = v
	}
	return cm, nil
}

func FromConfigMap(cm *apiv1.ConfigMap) (*spec.Plugin, error) {
	p := &spec.Plugin{
		TypeMeta: metav1.TypeMeta{
			Kind: cm.Labels[common.LabelKeyConfigMapType],
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.TrimSuffix(cm.Name, "-executor-plugin"),
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Spec: spec.PluginSpec{
			Sidecar: spec.Sidecar{
				Address: cm.Data["sidecar.address"],
			},
		},
	}
	for k, v := range cm.Annotations {
		p.Annotations[k] = v
	}
	for k, v := range cm.Labels {
		p.Labels[k] = v
	}
	delete(p.Labels, common.LabelKeyConfigMapType)
	err := yaml.UnmarshalStrict([]byte(cm.Data["sidecar.container"]), &p.Spec.Sidecar.Container)
	return p, err
}
