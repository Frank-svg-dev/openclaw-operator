package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenClawAgentDefaultsPhase string

const (
	OpenClawAgentDefaultsPhaseReady OpenClawAgentDefaultsPhase = "Ready"
	OpenClawAgentDefaultsPhaseError OpenClawAgentDefaultsPhase = "Error"
)

type OpenClawAgentDefaults struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawAgentDefaultsSpec   `json:"spec,omitempty"`
	Status OpenClawAgentDefaultsStatus `json:"status,omitempty"`
}

type OpenClawAgentDefaultsSpec struct {
	OpenclawRef  OpenclawRef `json:"openclawRef"`
	PrimaryModel string      `json:"primaryModel"`
	Fallbacks    []string    `json:"fallbacks,omitempty"`
	Workspace    string      `json:"workspace"`
}

type OpenClawAgentDefaultsStatus struct {
	Phase   OpenClawAgentDefaultsPhase `json:"phase,omitempty"`
	Message string                     `json:"message,omitempty"`
}

func (in *OpenClawAgentDefaults) DeepCopy() *OpenClawAgentDefaults {
	if in == nil {
		return nil
	}
	out := new(OpenClawAgentDefaults)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAgentDefaults) DeepCopyInto(out *OpenClawAgentDefaults) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *OpenClawAgentDefaults) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type OpenClawAgentDefaultsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawAgentDefaults `json:"items"`
}

func (in *OpenClawAgentDefaultsList) DeepCopy() *OpenClawAgentDefaultsList {
	if in == nil {
		return nil
	}
	out := new(OpenClawAgentDefaultsList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAgentDefaultsList) DeepCopyInto(out *OpenClawAgentDefaultsList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenClawAgentDefaults, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenClawAgentDefaultsList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&OpenClawAgentDefaults{}, &OpenClawAgentDefaultsList{})
}
