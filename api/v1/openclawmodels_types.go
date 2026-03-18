package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type OpenClawModelsPhase string

const (
	OpenClawModelsPhaseReady OpenClawModelsPhase = "Ready"
	OpenClawModelsPhaseError OpenClawModelsPhase = "Error"
)

type OpenClawModels struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawModelsSpec   `json:"spec,omitempty"`
	Status OpenClawModelsStatus `json:"status,omitempty"`
}

type OpenClawModelsSpec struct {
	OpenclawRef OpenclawRef `json:"openclawRef"`
	Mode       string      `json:"mode"`
	Providers  []Provider  `json:"providers"`
}

type Provider struct {
	Name     string   `json:"name"`
	API      string   `json:"api"`
	APIKey   string   `json:"apiKey"`
	BaseURL  string   `json:"baseUrl"`
	Models    []Model  `json:"models"`
}

type Model struct {
	ContextWindow int          `json:"contextWindow"`
	Cost         Cost         `json:"cost"`
	ID           string       `json:"id"`
	Input        []string     `json:"input"`
	MaxTokens    int          `json:"maxTokens"`
	Name         string       `json:"name"`
	Reasoning    bool         `json:"reasoning"`
}

type Cost struct {
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
	Input      int `json:"input"`
	Output     int `json:"output"`
}

type OpenClawModelsStatus struct {
	Phase   OpenClawModelsPhase `json:"phase,omitempty"`
	Message string               `json:"message,omitempty"`
}

func (in *OpenClawModels) DeepCopy() *OpenClawModels {
	if in == nil {
		return nil
	}
	out := new(OpenClawModels)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawModels) DeepCopyInto(out *OpenClawModels) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

func (in *OpenClawModels) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type OpenClawModelsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawModels `json:"items"`
}

func (in *OpenClawModelsList) DeepCopy() *OpenClawModelsList {
	if in == nil {
		return nil
	}
	out := new(OpenClawModelsList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenClawModelsList) DeepCopyInto(out *OpenClawModelsList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OpenClawModels, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenClawModelsList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func init() {
	SchemeBuilder.Register(&OpenClawModels{}, &OpenClawModelsList{})
}
