package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenClawAllowedOriginSpec struct {
	OpenclawRef OpenclawRef `json:"openclawRef"`
	Origin      string      `json:"origin"`
	Enabled     bool        `json:"enabled"`
	UseHTTP     bool        `json:"useHTTP,omitempty"`
}

type OpenclawRef struct {
	Name string `json:"name"`
}

type OpenClawAllowedOriginStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type OpenClawAllowedOriginPhase string

const (
	OpenClawAllowedOriginPhasePending OpenClawAllowedOriginPhase = "Pending"
	OpenClawAllowedOriginPhaseReady   OpenClawAllowedOriginPhase = "Ready"
	OpenClawAllowedOriginPhaseError   OpenClawAllowedOriginPhase = "Error"
)

type OpenClawAllowedOrigin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawAllowedOriginSpec   `json:"spec,omitempty"`
	Status OpenClawAllowedOriginStatus `json:"status,omitempty"`
}

func (in *OpenClawAllowedOrigin) DeepCopy() *OpenClawAllowedOrigin {
	if in == nil {
		return nil
	}
	out := new(OpenClawAllowedOrigin)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAllowedOrigin) DeepCopyInto(out *OpenClawAllowedOrigin) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *OpenClawAllowedOrigin) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type OpenClawAllowedOriginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawAllowedOrigin `json:"items"`
}

func (in *OpenClawAllowedOriginList) DeepCopy() *OpenClawAllowedOriginList {
	if in == nil {
		return nil
	}
	out := new(OpenClawAllowedOriginList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawAllowedOriginList) DeepCopyInto(out *OpenClawAllowedOriginList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenClawAllowedOrigin, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenClawAllowedOriginList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&OpenClawAllowedOrigin{}, &OpenClawAllowedOriginList{})
}
