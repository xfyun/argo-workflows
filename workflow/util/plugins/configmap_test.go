package plugin

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-workflows/v3/pkg/plugins/spec"
	"github.com/argoproj/argo-workflows/v3/workflow/common"
)

func TestToConfigMap(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := ToConfigMap(&spec.Plugin{})
		assert.EqualError(t, err, "sidecar is invalid: address is invalid: parse \"\": empty url")
	})
	t.Run("Valid", func(t *testing.T) {
		cm, err := ToConfigMap(&spec.Plugin{
			TypeMeta: metav1.TypeMeta{
				Kind: common.LabelValueTypeConfigMapExecutorPlugin,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-plug",
				Annotations: map[string]string{
					"my-anno": "my-value",
				},
				Labels: map[string]string{
					"my-label": "my-value",
				},
			},
			Spec: spec.PluginSpec{
				Sidecar: spec.Sidecar{
					Address: "http://localhost:1234",
					Container: apiv1.Container{
						Resources: apiv1.ResourceRequirements{
							Limits:   map[apiv1.ResourceName]resource.Quantity{},
							Requests: map[apiv1.ResourceName]resource.Quantity{},
						},
						SecurityContext: &apiv1.SecurityContext{},
					},
				},
			},
		})
		if assert.NoError(t, err) {
			assert.Equal(t, "my-plug-executor-plugin", cm.Name)
			assert.Len(t, cm.Annotations, 1)
			assert.Equal(t, map[string]string{
				"my-label":                             "my-value",
				"workflows.argoproj.io/configmap-type": "ExecutorPlugin",
			}, cm.Labels)
			assert.Equal(t, map[string]string{
				"sidecar.address":   "http://localhost:1234",
				"sidecar.container": "name: \"\"\nresources: {}\nsecurityContext: {}\n",
			}, cm.Data)
		}
	})
}

func TestFromConfigMap(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := FromConfigMap(&apiv1.ConfigMap{})
		assert.EqualError(t, err, "sidecar is invalid: address is invalid: parse \"\": empty url")
	})
	t.Run("Valid", func(t *testing.T) {
		p, err := FromConfigMap(&apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-plug-executor-plugin",
				Annotations: map[string]string{
					"my-anno": "my-value",
				},
				Labels: map[string]string{
					common.LabelKeyConfigMapType: common.LabelValueTypeConfigMapExecutorPlugin,
					"my-label":                   "my-value",
				},
			},
			Data: map[string]string{
				"sidecar.address":   "http://my-addr",
				"sidecar.container": "{'name': 'my-name', 'resources': {'requests': {}, 'limits': {}}, 'securityContext': {}}",
			},
		})
		if assert.NoError(t, err) {
			assert.Equal(t, "ExecutorPlugin", p.Kind)
			assert.Equal(t, "my-plug", p.Name)
			assert.Len(t, p.Annotations, 1)
			assert.Len(t, p.Labels, 1)
			assert.Equal(t, "http://my-addr", p.Spec.Sidecar.Address)
			assert.Equal(t, apiv1.Container{
				Name: "my-name",
				Resources: apiv1.ResourceRequirements{
					Limits:   map[apiv1.ResourceName]resource.Quantity{},
					Requests: map[apiv1.ResourceName]resource.Quantity{},
				},
				SecurityContext: &apiv1.SecurityContext{},
			}, p.Spec.Sidecar.Container)
		}
	})
}
