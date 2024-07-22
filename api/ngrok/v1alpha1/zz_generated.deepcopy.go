//go:build !ignore_autogenerated

/*
MIT License

Copyright (c) 2024 ngrok, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"encoding/json"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NgrokTrafficPolicy) DeepCopyInto(out *NgrokTrafficPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NgrokTrafficPolicy.
func (in *NgrokTrafficPolicy) DeepCopy() *NgrokTrafficPolicy {
	if in == nil {
		return nil
	}
	out := new(NgrokTrafficPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NgrokTrafficPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NgrokTrafficPolicyList) DeepCopyInto(out *NgrokTrafficPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NgrokTrafficPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NgrokTrafficPolicyList.
func (in *NgrokTrafficPolicyList) DeepCopy() *NgrokTrafficPolicyList {
	if in == nil {
		return nil
	}
	out := new(NgrokTrafficPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NgrokTrafficPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NgrokTrafficPolicySpec) DeepCopyInto(out *NgrokTrafficPolicySpec) {
	*out = *in
	if in.TrafficPolicy != nil {
		in, out := &in.TrafficPolicy, &out.TrafficPolicy
		*out = make(json.RawMessage, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NgrokTrafficPolicySpec.
func (in *NgrokTrafficPolicySpec) DeepCopy() *NgrokTrafficPolicySpec {
	if in == nil {
		return nil
	}
	out := new(NgrokTrafficPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NgrokTrafficPolicyStatus) DeepCopyInto(out *NgrokTrafficPolicyStatus) {
	*out = *in
	if in.TrafficPolicy != nil {
		in, out := &in.TrafficPolicy, &out.TrafficPolicy
		*out = make(json.RawMessage, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NgrokTrafficPolicyStatus.
func (in *NgrokTrafficPolicyStatus) DeepCopy() *NgrokTrafficPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(NgrokTrafficPolicyStatus)
	in.DeepCopyInto(out)
	return out
}
