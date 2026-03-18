package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenClawAgentPhase string

const (
	OpenClawAgentPhaseReady OpenClawAgentPhase = "Ready"
	OpenClawAgentPhaseError OpenClawAgentPhase = "Error"
)

type OpenClawAgent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawAgentSpec   `json:"spec,omitempty"`
	Status OpenClawAgentStatus `json:"status,omitempty"`
}

type OpenClawAgentSpec struct {
	OpenclawRef OpenclawRef `json:"openclawRef"`
	ID          string      `json:"id"`
	Name        string      `json:"name,omitempty"`
	Model       string      `json:"model,omitempty"`
	Enabled     bool        `json:"enabled"`
	Default     bool        `json:"default,omitempty"`
}

type OpenClawAgentStatus struct {
	Phase   OpenClawAgentPhase `json:"phase,omitempty"`
	Message string             `json:"message,omitempty"`
}

func (in *OpenClawAgent) DeepCopy() *OpenClawAgent {
	if in == nil {
		return nil
	}
	out := new(OpenClawAgent)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAgent) DeepCopyInto(out *OpenClawAgent) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *OpenClawAgent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type OpenClawAgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawAgent `json:"items"`
}

func (in *OpenClawAgentList) DeepCopy() *OpenClawAgentList {
	if in == nil {
		return nil
	}
	out := new(OpenClawAgentList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAgentList) DeepCopyInto(out *OpenClawAgentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenClawAgent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenClawAgentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&OpenClawAgent{}, &OpenClawAgentList{})
}
