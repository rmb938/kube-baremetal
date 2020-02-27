// +build !ignore_autogenerated

/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscovery) DeepCopyInto(out *BareMetalDiscovery) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscovery.
func (in *BareMetalDiscovery) DeepCopy() *BareMetalDiscovery {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscovery)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalDiscovery) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryHardware) DeepCopyInto(out *BareMetalDiscoveryHardware) {
	*out = *in
	in.CPU.DeepCopyInto(&out.CPU)
	out.Ram = in.Ram.DeepCopy()
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = make([]BareMetalDiscoveryHardwareStorage, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NICS != nil {
		in, out := &in.NICS, &out.NICS
		*out = make([]BareMetalDiscoveryHardwareNIC, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryHardware.
func (in *BareMetalDiscoveryHardware) DeepCopy() *BareMetalDiscoveryHardware {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryHardware)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryHardwareCPU) DeepCopyInto(out *BareMetalDiscoveryHardwareCPU) {
	*out = *in
	out.CPUS = in.CPUS.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryHardwareCPU.
func (in *BareMetalDiscoveryHardwareCPU) DeepCopy() *BareMetalDiscoveryHardwareCPU {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryHardwareCPU)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryHardwareNIC) DeepCopyInto(out *BareMetalDiscoveryHardwareNIC) {
	*out = *in
	out.Speed = in.Speed.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryHardwareNIC.
func (in *BareMetalDiscoveryHardwareNIC) DeepCopy() *BareMetalDiscoveryHardwareNIC {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryHardwareNIC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryHardwareStorage) DeepCopyInto(out *BareMetalDiscoveryHardwareStorage) {
	*out = *in
	out.Size = in.Size.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryHardwareStorage.
func (in *BareMetalDiscoveryHardwareStorage) DeepCopy() *BareMetalDiscoveryHardwareStorage {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryHardwareStorage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryList) DeepCopyInto(out *BareMetalDiscoveryList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BareMetalDiscovery, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryList.
func (in *BareMetalDiscoveryList) DeepCopy() *BareMetalDiscoveryList {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalDiscoveryList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoverySpec) DeepCopyInto(out *BareMetalDiscoverySpec) {
	*out = *in
	if in.Hardware != nil {
		in, out := &in.Hardware, &out.Hardware
		*out = new(BareMetalDiscoveryHardware)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoverySpec.
func (in *BareMetalDiscoverySpec) DeepCopy() *BareMetalDiscoverySpec {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoverySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalDiscoveryStatus) DeepCopyInto(out *BareMetalDiscoveryStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalDiscoveryStatus.
func (in *BareMetalDiscoveryStatus) DeepCopy() *BareMetalDiscoveryStatus {
	if in == nil {
		return nil
	}
	out := new(BareMetalDiscoveryStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalEndpoint) DeepCopyInto(out *BareMetalEndpoint) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalEndpoint.
func (in *BareMetalEndpoint) DeepCopy() *BareMetalEndpoint {
	if in == nil {
		return nil
	}
	out := new(BareMetalEndpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalEndpoint) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalEndpointList) DeepCopyInto(out *BareMetalEndpointList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BareMetalEndpoint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalEndpointList.
func (in *BareMetalEndpointList) DeepCopy() *BareMetalEndpointList {
	if in == nil {
		return nil
	}
	out := new(BareMetalEndpointList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalEndpointList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalEndpointSpec) DeepCopyInto(out *BareMetalEndpointSpec) {
	*out = *in
	if in.MACS != nil {
		in, out := &in.MACS, &out.MACS
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.NetworkRef.DeepCopyInto(&out.NetworkRef)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalEndpointSpec.
func (in *BareMetalEndpointSpec) DeepCopy() *BareMetalEndpointSpec {
	if in == nil {
		return nil
	}
	out := new(BareMetalEndpointSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalEndpointStatus) DeepCopyInto(out *BareMetalEndpointStatus) {
	*out = *in
	in.StatusConditions.DeepCopyInto(&out.StatusConditions)
	if in.Address != nil {
		in, out := &in.Address, &out.Address
		*out = new(BareMetalEndpointStatusAddress)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalEndpointStatus.
func (in *BareMetalEndpointStatus) DeepCopy() *BareMetalEndpointStatus {
	if in == nil {
		return nil
	}
	out := new(BareMetalEndpointStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalEndpointStatusAddress) DeepCopyInto(out *BareMetalEndpointStatusAddress) {
	*out = *in
	if in.Nameservers != nil {
		in, out := &in.Nameservers, &out.Nameservers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Search != nil {
		in, out := &in.Search, &out.Search
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalEndpointStatusAddress.
func (in *BareMetalEndpointStatusAddress) DeepCopy() *BareMetalEndpointStatusAddress {
	if in == nil {
		return nil
	}
	out := new(BareMetalEndpointStatusAddress)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardware) DeepCopyInto(out *BareMetalHardware) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardware.
func (in *BareMetalHardware) DeepCopy() *BareMetalHardware {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardware)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalHardware) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareList) DeepCopyInto(out *BareMetalHardwareList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BareMetalHardware, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareList.
func (in *BareMetalHardwareList) DeepCopy() *BareMetalHardwareList {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalHardwareList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareNIC) DeepCopyInto(out *BareMetalHardwareNIC) {
	*out = *in
	if in.Bond != nil {
		in, out := &in.Bond, &out.Bond
		*out = new(BareMetalHardwareNICBond)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareNIC.
func (in *BareMetalHardwareNIC) DeepCopy() *BareMetalHardwareNIC {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareNIC)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareNICBond) DeepCopyInto(out *BareMetalHardwareNICBond) {
	*out = *in
	if in.Interfaces != nil {
		in, out := &in.Interfaces, &out.Interfaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareNICBond.
func (in *BareMetalHardwareNICBond) DeepCopy() *BareMetalHardwareNICBond {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareNICBond)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareSpec) DeepCopyInto(out *BareMetalHardwareSpec) {
	*out = *in
	if in.NICS != nil {
		in, out := &in.NICS, &out.NICS
		*out = make([]BareMetalHardwareNIC, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Taints != nil {
		in, out := &in.Taints, &out.Taints
		*out = make([]v1.Taint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareSpec.
func (in *BareMetalHardwareSpec) DeepCopy() *BareMetalHardwareSpec {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareStatus) DeepCopyInto(out *BareMetalHardwareStatus) {
	*out = *in
	in.StatusConditions.DeepCopyInto(&out.StatusConditions)
	if in.InstanceRef != nil {
		in, out := &in.InstanceRef, &out.InstanceRef
		*out = new(BareMetalHardwareStatusInstanceRef)
		**out = **in
	}
	if in.Hardware != nil {
		in, out := &in.Hardware, &out.Hardware
		*out = new(BareMetalDiscoveryHardware)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareStatus.
func (in *BareMetalHardwareStatus) DeepCopy() *BareMetalHardwareStatus {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalHardwareStatusInstanceRef) DeepCopyInto(out *BareMetalHardwareStatusInstanceRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalHardwareStatusInstanceRef.
func (in *BareMetalHardwareStatusInstanceRef) DeepCopy() *BareMetalHardwareStatusInstanceRef {
	if in == nil {
		return nil
	}
	out := new(BareMetalHardwareStatusInstanceRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalInstance) DeepCopyInto(out *BareMetalInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalInstance.
func (in *BareMetalInstance) DeepCopy() *BareMetalInstance {
	if in == nil {
		return nil
	}
	out := new(BareMetalInstance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalInstanceList) DeepCopyInto(out *BareMetalInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]BareMetalInstance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalInstanceList.
func (in *BareMetalInstanceList) DeepCopy() *BareMetalInstanceList {
	if in == nil {
		return nil
	}
	out := new(BareMetalInstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BareMetalInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalInstanceSpec) DeepCopyInto(out *BareMetalInstanceSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalInstanceSpec.
func (in *BareMetalInstanceSpec) DeepCopy() *BareMetalInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(BareMetalInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalInstanceStatus) DeepCopyInto(out *BareMetalInstanceStatus) {
	*out = *in
	in.StatusConditions.DeepCopyInto(&out.StatusConditions)
	if in.AgentInfo != nil {
		in, out := &in.AgentInfo, &out.AgentInfo
		*out = new(BareMetalInstanceStatusAgentInfo)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalInstanceStatus.
func (in *BareMetalInstanceStatus) DeepCopy() *BareMetalInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(BareMetalInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BareMetalInstanceStatusAgentInfo) DeepCopyInto(out *BareMetalInstanceStatusAgentInfo) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BareMetalInstanceStatusAgentInfo.
func (in *BareMetalInstanceStatusAgentInfo) DeepCopy() *BareMetalInstanceStatusAgentInfo {
	if in == nil {
		return nil
	}
	out := new(BareMetalInstanceStatusAgentInfo)
	in.DeepCopyInto(out)
	return out
}
