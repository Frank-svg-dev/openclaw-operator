package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type SecretRefs struct {
	AppId     SecretRef `json:"appId"`
	AppSecret SecretRef `json:"appSecret"`
}

type OpenClawChannelSpec struct {
	OpenclawRef    OpenclawRef            `json:"openclawRef"`
	Type           string                 `json:"type"`
	Enabled        bool                   `json:"enabled"`
	AccountName    string                 `json:"accountName"`
	DmPolicy       string                 `json:"dmPolicy"`
	BotName        string                 `json:"botName"`
	SecretRefs     SecretRefs             `json:"secretRefs"`
	RequireMention string                 `json:"requireMention,omitempty"`
	GroupPolicy    string                 `json:"groupPolicy,omitempty"`
	Groups         map[string]GroupConfig `json:"groups,omitempty"`
}

type GroupConfig struct {
	RequireMention bool `json:"requireMention,omitempty"`
}

type OpenClawChannelStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type OpenClawChannelPhase string

const (
	OpenClawChannelPhasePending OpenClawChannelPhase = "Pending"
	OpenClawChannelPhaseReady   OpenClawChannelPhase = "Ready"
	OpenClawChannelPhaseError   OpenClawChannelPhase = "Error"
)

type OpenClawChannel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawChannelSpec   `json:"spec,omitempty"`
	Status OpenClawChannelStatus `json:"status,omitempty"`
}

func (in *OpenClawChannel) DeepCopy() *OpenClawChannel {
	if in == nil {
		return nil
	}
	out := new(OpenClawChannel)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawChannel) DeepCopyInto(out *OpenClawChannel) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *OpenClawChannel) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type OpenClawChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawChannel `json:"items"`
}

func (in *OpenClawChannelList) DeepCopy() *OpenClawChannelList {
	if in == nil {
		return nil
	}
	out := new(OpenClawChannelList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawChannelList) DeepCopyInto(out *OpenClawChannelList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenClawChannel, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenClawChannelList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&OpenClawChannel{}, &OpenClawChannelList{})
}
