package spec

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Plugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              PluginSpec `json:"spec"`
}

type PluginSpec struct {
	Sidecar Sidecar `json:"sidecar"`
}

type Sidecar struct {
	Address   string          `json:"address"`
	Container apiv1.Container `json:"container"`
}
