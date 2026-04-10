package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type OpenclawSpec struct {
	Image               string               `json:"image,omitempty"`
	Replicas            *int32               `json:"replicas,omitempty"`
	ServiceType         string               `json:"serviceType,omitempty"`
	Resources           ResourceRequirements `json:"resources,omitempty"`
	Storage             StorageSpec          `json:"storage,omitempty"`
	GatewayToken        string               `json:"gatewayToken,omitempty"`
	CustomAPIKey        string               `json:"customApiKey,omitempty"`
	CustomBaseURL       string               `json:"customBaseUrl,omitempty"`
	CustomModelID       string               `json:"customModelId,omitempty"`
	CustomProviderID    string               `json:"customProviderId,omitempty"`
	CustomCompatibility string               `json:"customCompatibility,omitempty"`
	GatewayPort         int32                `json:"gatewayPort,omitempty"`
	GatewayBind         string               `json:"gatewayBind,omitempty"`
	Privacy             *bool                `json:"privacy,omitempty"`
	SLMAPIURL           string               `json:"slmApiUrl,omitempty"`
	SLMModelID          string               `json:"slmModelId,omitempty"`
	SLMAPIKey           string               `json:"slmApiKey,omitempty"`
	Redis               RedisSpec            `json:"redis,omitempty"`
}

type RedisSpec struct {
	Address  string `json:"address,omitempty"`
	Password string `json:"password,omitempty"`
	DB       int    `json:"db,omitempty"`
}

type StorageSpec struct {
	AccessModes []string `json:"accessModes,omitempty"`
	Storage     string   `json:"storage,omitempty"`
}

type ResourceRequirements struct {
	Requests *ResourceList `json:"requests,omitempty"`
	Limits   *ResourceList `json:"limits,omitempty"`
}

type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type OpenclawStatus struct {
	Replicas      int32  `json:"replicas,omitempty"`
	ReadyReplicas int32  `json:"readyReplicas,omitempty"`
	Phase         string `json:"phase,omitempty"`
	Message       string `json:"message,omitempty"`
}

type OpenclawPhase string

const (
	OpenclawPhasePending   OpenclawPhase = "Pending"
	OpenclawPhaseRunning   OpenclawPhase = "Running"
	OpenclawPhaseFailed    OpenclawPhase = "Failed"
	OpenclawPhaseSucceeded OpenclawPhase = "Succeeded"
)

func (in *Openclaw) DeepCopyInto(out *Openclaw) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

func (in *Openclaw) DeepCopy() *Openclaw {
	if in == nil {
		return nil
	}
	out := new(Openclaw)
	in.DeepCopyInto(out)
	return out
}

func (in *Openclaw) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *OpenclawList) DeepCopyInto(out *OpenclawList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Openclaw, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *OpenclawList) DeepCopy() *OpenclawList {
	if in == nil {
		return nil
	}
	out := new(OpenclawList)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenclawList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *OpenclawSpec) DeepCopyInto(out *OpenclawSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Resources.Requests != nil {
		in, out := &in.Resources.Requests, &out.Resources.Requests
		*out = new(ResourceList)
		**out = **in
	}
	if in.Resources.Limits != nil {
		in, out := &in.Resources.Limits, &out.Resources.Limits
		*out = new(ResourceList)
		**out = **in
	}
	if in.Storage.AccessModes != nil {
		in, out := &in.Storage.AccessModes, &out.Storage.AccessModes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *OpenclawSpec) DeepCopy() *OpenclawSpec {
	if in == nil {
		return nil
	}
	out := new(OpenclawSpec)
	in.DeepCopyInto(out)
	return out
}

func (in *OpenclawStatus) DeepCopyInto(out *OpenclawStatus) {
	*out = *in
}

func (in *OpenclawStatus) DeepCopy() *OpenclawStatus {
	if in == nil {
		return nil
	}
	out := new(OpenclawStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceRequirements) DeepCopyInto(out *ResourceRequirements) {
	*out = *in
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = new(ResourceList)
		**out = **in
	}
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = new(ResourceList)
		**out = **in
	}
}

func (in *ResourceRequirements) DeepCopy() *ResourceRequirements {
	if in == nil {
		return nil
	}
	out := new(ResourceRequirements)
	in.DeepCopyInto(out)
	return out
}

func (in *StorageSpec) DeepCopyInto(out *StorageSpec) {
	*out = *in
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *StorageSpec) DeepCopy() *StorageSpec {
	if in == nil {
		return nil
	}
	out := new(StorageSpec)
	in.DeepCopyInto(out)
	return out
}

var _ runtime.Object = &Openclaw{}
var _ runtime.Object = &OpenclawList{}

func (in *Openclaw) GetObjectKind() schema.ObjectKind {
	return &in.TypeMeta
}

func (in *OpenclawList) GetObjectKind() schema.ObjectKind {
	return &in.TypeMeta
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:path=openclaws,scope=Namespaced,shortName=oc
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Openclaw struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenclawSpec   `json:"spec,omitempty"`
	Status OpenclawStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type OpenclawList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Openclaw `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Openclaw{}, &OpenclawList{})
}
