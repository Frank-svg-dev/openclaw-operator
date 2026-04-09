// Package v1 contains API Schema definitions for the openclaw v1 API group
// +kubebuilder:object:generate=true
// +groupName=openclaw.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupName    = "openclaw.io"
	GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}

	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)
