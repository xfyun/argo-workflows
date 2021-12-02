package spec

import (
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Plugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              PluginSpec `json:"spec"`
}

func (p Plugin) Validate() error {
	c := p.Spec.Sidecar.Container
	if c.Resources.Requests == nil {
		return fmt.Errorf("resources requests are mandatory")
	}
	if c.Resources.Limits == nil {
		return fmt.Errorf("resources limits are mandatory")
	}
	if c.SecurityContext == nil {
		return fmt.Errorf("security context is mandatory")
	}
	return nil
}

type PluginSpec struct {
	Sidecar Sidecar `json:"sidecar"`
}

type Sidecar struct {
	Address   string          `json:"address"`
	Container apiv1.Container `json:"container"`
}
