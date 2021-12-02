package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestPlugin_Validate(t *testing.T) {
	t.Run("ResourceRequests", func(t *testing.T) {
		assert.EqualError(t, Plugin{}.Validate(), "resources requests are mandatory")
	})
	t.Run("ResourceLimits", func(t *testing.T) {
		assert.EqualError(t, Plugin{
			Spec: PluginSpec{
				Sidecar: Sidecar{
					Container: apiv1.Container{Resources: apiv1.ResourceRequirements{
						Requests: map[apiv1.ResourceName]resource.Quantity{apiv1.ResourceCPU: resource.MustParse("1")},
					}},
				},
			},
		}.Validate(), "resources limits are mandatory")
	})
	t.Run("SecurityContext", func(t *testing.T) {
		assert.EqualError(t, Plugin{
			Spec: PluginSpec{
				Sidecar: Sidecar{
					Container: apiv1.Container{Resources: apiv1.ResourceRequirements{
						Requests: map[apiv1.ResourceName]resource.Quantity{apiv1.ResourceCPU: resource.MustParse("1")},
						Limits:   map[apiv1.ResourceName]resource.Quantity{apiv1.ResourceCPU: resource.MustParse("1")},
					}},
				},
			},
		}.Validate(), "security context is mandatory")
	})
}
