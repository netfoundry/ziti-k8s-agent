//go:build !ignore_autogenerated

/*
Copyright 2025 NetFoundry.

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
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openziti/edge-api/rest_model"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ApiProxy) DeepCopyInto(out *ApiProxy) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ApiProxy.
func (in *ApiProxy) DeepCopy() *ApiProxy {
	if in == nil {
		return nil
	}
	out := new(ApiProxy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackoffParamters) DeepCopyInto(out *BackoffParamters) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackoffParamters.
func (in *BackoffParamters) DeepCopy() *BackoffParamters {
	if in == nil {
		return nil
	}
	out := new(BackoffParamters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CPU) DeepCopyInto(out *CPU) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CPU.
func (in *CPU) DeepCopy() *CPU {
	if in == nil {
		return nil
	}
	out := new(CPU)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertificateSpecs) DeepCopyInto(out *CertificateSpecs) {
	*out = *in
	if in.Organizations != nil {
		in, out := &in.Organizations, &out.Organizations
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertificateSpecs.
func (in *CertificateSpecs) DeepCopy() *CertificateSpecs {
	if in == nil {
		return nil
	}
	out := new(CertificateSpecs)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ChannelOptions) DeepCopyInto(out *ChannelOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ChannelOptions.
func (in *ChannelOptions) DeepCopy() *ChannelOptions {
	if in == nil {
		return nil
	}
	out := new(ChannelOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterRoleSpec) DeepCopyInto(out *ClusterRoleSpec) {
	*out = *in
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]rbacv1.PolicyRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterRoleSpec.
func (in *ClusterRoleSpec) DeepCopy() *ClusterRoleSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterRoleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Config) DeepCopyInto(out *Config) {
	*out = *in
	in.Identity.DeepCopyInto(&out.Identity)
	out.Forwarder = in.Forwarder
	out.Trace = in.Trace
	out.Profile = in.Profile
	in.Ctrl.DeepCopyInto(&out.Ctrl)
	in.Link.DeepCopyInto(&out.Link)
	if in.Dialers != nil {
		in, out := &in.Dialers, &out.Dialers
		*out = make([]EdgeDialer, len(*in))
		copy(*out, *in)
	}
	if in.Listeners != nil {
		in, out := &in.Listeners, &out.Listeners
		*out = make([]EdgeListener, len(*in))
		copy(*out, *in)
	}
	if in.Transport != nil {
		in, out := &in.Transport, &out.Transport
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Metrics = in.Metrics
	out.HealthChecks = in.HealthChecks
	out.ConnectEvents = in.ConnectEvents
	if in.Plugins != nil {
		in, out := &in.Plugins, &out.Plugins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Edge.DeepCopyInto(&out.Edge)
	in.Web.DeepCopyInto(&out.Web)
	if in.Proxy != nil {
		in, out := &in.Proxy, &out.Proxy
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Config.
func (in *Config) DeepCopy() *Config {
	if in == nil {
		return nil
	}
	out := new(Config)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConnectEventsConfig) DeepCopyInto(out *ConnectEventsConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConnectEventsConfig.
func (in *ConnectEventsConfig) DeepCopy() *ConnectEventsConfig {
	if in == nil {
		return nil
	}
	out := new(ConnectEventsConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Csr) DeepCopyInto(out *Csr) {
	*out = *in
	in.Sans.DeepCopyInto(&out.Sans)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Csr.
func (in *Csr) DeepCopy() *Csr {
	if in == nil {
		return nil
	}
	out := new(Csr)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Ctrl) DeepCopyInto(out *Ctrl) {
	*out = *in
	if in.Endpoints != nil {
		in, out := &in.Endpoints, &out.Endpoints
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.Options = in.Options
	out.Heartbeats = in.Heartbeats
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Ctrl.
func (in *Ctrl) DeepCopy() *Ctrl {
	if in == nil {
		return nil
	}
	out := new(Ctrl)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CtrlPingCheck) DeepCopyInto(out *CtrlPingCheck) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CtrlPingCheck.
func (in *CtrlPingCheck) DeepCopy() *CtrlPingCheck {
	if in == nil {
		return nil
	}
	out := new(CtrlPingCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentEnvVars) DeepCopyInto(out *DeploymentEnvVars) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentEnvVars.
func (in *DeploymentEnvVars) DeepCopy() *DeploymentEnvVars {
	if in == nil {
		return nil
	}
	out := new(DeploymentEnvVars)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeploymentSpec) DeepCopyInto(out *DeploymentSpec) {
	*out = *in
	out.Env = in.Env
	if in.ResourceRequest != nil {
		in, out := &in.ResourceRequest, &out.ResourceRequest
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
	if in.ResourceLimit != nil {
		in, out := &in.ResourceLimit, &out.ResourceLimit
		*out = make(corev1.ResourceList, len(*in))
		for key, val := range *in {
			(*out)[key] = val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeploymentSpec.
func (in *DeploymentSpec) DeepCopy() *DeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(DeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeConfig) DeepCopyInto(out *EdgeConfig) {
	*out = *in
	out.ApiProxy = in.ApiProxy
	in.Csr.DeepCopyInto(&out.Csr)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeConfig.
func (in *EdgeConfig) DeepCopy() *EdgeConfig {
	if in == nil {
		return nil
	}
	out := new(EdgeConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeDialer) DeepCopyInto(out *EdgeDialer) {
	*out = *in
	out.Options = in.Options
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeDialer.
func (in *EdgeDialer) DeepCopy() *EdgeDialer {
	if in == nil {
		return nil
	}
	out := new(EdgeDialer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeListener) DeepCopyInto(out *EdgeListener) {
	*out = *in
	out.Options = in.Options
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeListener.
func (in *EdgeListener) DeepCopy() *EdgeListener {
	if in == nil {
		return nil
	}
	out := new(EdgeListener)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeOptions) DeepCopyInto(out *EdgeOptions) {
	*out = *in
	out.Options = in.Options
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeOptions.
func (in *EdgeOptions) DeepCopy() *EdgeOptions {
	if in == nil {
		return nil
	}
	out := new(EdgeOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ForwarderOptions) DeepCopyInto(out *ForwarderOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ForwarderOptions.
func (in *ForwarderOptions) DeepCopy() *ForwarderOptions {
	if in == nil {
		return nil
	}
	out := new(ForwarderOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HealthChecks) DeepCopyInto(out *HealthChecks) {
	*out = *in
	out.CtrlPingCheck = in.CtrlPingCheck
	out.LinkCheck = in.LinkCheck
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HealthChecks.
func (in *HealthChecks) DeepCopy() *HealthChecks {
	if in == nil {
		return nil
	}
	out := new(HealthChecks)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HeartbeatOptions) DeepCopyInto(out *HeartbeatOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HeartbeatOptions.
func (in *HeartbeatOptions) DeepCopy() *HeartbeatOptions {
	if in == nil {
		return nil
	}
	out := new(HeartbeatOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Identity) DeepCopyInto(out *Identity) {
	*out = *in
	if in.AltServerCerts != nil {
		in, out := &in.AltServerCerts, &out.AltServerCerts
		*out = make([]ServerPair, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Identity.
func (in *Identity) DeepCopy() *Identity {
	if in == nil {
		return nil
	}
	out := new(Identity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Link) DeepCopyInto(out *Link) {
	*out = *in
	if in.Listeners != nil {
		in, out := &in.Listeners, &out.Listeners
		*out = make([]LinkListener, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Dialers != nil {
		in, out := &in.Dialers, &out.Dialers
		*out = make([]LinkDialer, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Link.
func (in *Link) DeepCopy() *Link {
	if in == nil {
		return nil
	}
	out := new(Link)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinkCheck) DeepCopyInto(out *LinkCheck) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinkCheck.
func (in *LinkCheck) DeepCopy() *LinkCheck {
	if in == nil {
		return nil
	}
	out := new(LinkCheck)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinkDialer) DeepCopyInto(out *LinkDialer) {
	*out = *in
	if in.Groups != nil {
		in, out := &in.Groups, &out.Groups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.Options = in.Options
	out.HealthyDialBackoff = in.HealthyDialBackoff
	out.UnhealthyDialBackoff = in.UnhealthyDialBackoff
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinkDialer.
func (in *LinkDialer) DeepCopy() *LinkDialer {
	if in == nil {
		return nil
	}
	out := new(LinkDialer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinkListener) DeepCopyInto(out *LinkListener) {
	*out = *in
	if in.Groups != nil {
		in, out := &in.Groups, &out.Groups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.Options = in.Options
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinkListener.
func (in *LinkListener) DeepCopy() *LinkListener {
	if in == nil {
		return nil
	}
	out := new(LinkListener)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Memory) DeepCopyInto(out *Memory) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Memory.
func (in *Memory) DeepCopy() *Memory {
	if in == nil {
		return nil
	}
	out := new(Memory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Metrics) DeepCopyInto(out *Metrics) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Metrics.
func (in *Metrics) DeepCopy() *Metrics {
	if in == nil {
		return nil
	}
	out := new(Metrics)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Profile) DeepCopyInto(out *Profile) {
	*out = *in
	out.Memory = in.Memory
	out.CPU = in.CPU
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Profile.
func (in *Profile) DeepCopy() *Profile {
	if in == nil {
		return nil
	}
	out := new(Profile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RouterCreateModel) DeepCopyInto(out *RouterCreateModel) {
	*out = *in
	in.AppData.DeepCopyInto(&out.AppData)
	if in.RoleAttributes != nil {
		in, out := &in.RoleAttributes, &out.RoleAttributes
		*out = make(rest_model.Attributes, len(*in))
		copy(*out, *in)
	}
	in.Tags.DeepCopyInto(&out.Tags)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RouterCreateModel.
func (in *RouterCreateModel) DeepCopy() *RouterCreateModel {
	if in == nil {
		return nil
	}
	out := new(RouterCreateModel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RouterDeploymentSpec) DeepCopyInto(out *RouterDeploymentSpec) {
	*out = *in
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Container.DeepCopyInto(&out.Container)
	if in.DNSConfig != nil {
		in, out := &in.DNSConfig, &out.DNSConfig
		*out = new(corev1.PodDNSConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(corev1.PodSecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.TerminationGracePeriodSeconds != nil {
		in, out := &in.TerminationGracePeriodSeconds, &out.TerminationGracePeriodSeconds
		*out = new(int64)
		**out = **in
	}
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.UpdateStrategy.DeepCopyInto(&out.UpdateStrategy)
	if in.RevisionHistoryLimit != nil {
		in, out := &in.RevisionHistoryLimit, &out.RevisionHistoryLimit
		*out = new(int32)
		**out = **in
	}
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
	if in.VolumeMode != nil {
		in, out := &in.VolumeMode, &out.VolumeMode
		*out = new(corev1.PersistentVolumeMode)
		**out = **in
	}
	if in.PersistentVolumeClaimRetentionPolicy != nil {
		in, out := &in.PersistentVolumeClaimRetentionPolicy, &out.PersistentVolumeClaimRetentionPolicy
		*out = new(appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy)
		**out = **in
	}
	if in.Ordinals != nil {
		in, out := &in.Ordinals, &out.Ordinals
		*out = new(appsv1.StatefulSetOrdinals)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RouterDeploymentSpec.
func (in *RouterDeploymentSpec) DeepCopy() *RouterDeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(RouterDeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Sans) DeepCopyInto(out *Sans) {
	*out = *in
	if in.DnsAddresses != nil {
		in, out := &in.DnsAddresses, &out.DnsAddresses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.IpAddresses != nil {
		in, out := &in.IpAddresses, &out.IpAddresses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.IpAddressesParsed != nil {
		in, out := &in.IpAddressesParsed, &out.IpAddressesParsed
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	if in.EmailAddresses != nil {
		in, out := &in.EmailAddresses, &out.EmailAddresses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.UriAddresses != nil {
		in, out := &in.UriAddresses, &out.UriAddresses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Sans.
func (in *Sans) DeepCopy() *Sans {
	if in == nil {
		return nil
	}
	out := new(Sans)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServerPair) DeepCopyInto(out *ServerPair) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServerPair.
func (in *ServerPair) DeepCopy() *ServerPair {
	if in == nil {
		return nil
	}
	out := new(ServerPair)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceAccountSpec) DeepCopyInto(out *ServiceAccountSpec) {
	*out = *in
	if in.Secrets != nil {
		in, out := &in.Secrets, &out.Secrets
		*out = make([]corev1.ObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.AutomountServiceAccountToken != nil {
		in, out := &in.AutomountServiceAccountToken, &out.AutomountServiceAccountToken
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceAccountSpec.
func (in *ServiceAccountSpec) DeepCopy() *ServiceAccountSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Tags) DeepCopyInto(out *Tags) {
	*out = *in
	if in.SubTags != nil {
		in, out := &in.SubTags, &out.SubTags
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tags.
func (in *Tags) DeepCopy() *Tags {
	if in == nil {
		return nil
	}
	out := new(Tags)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Trace) DeepCopyInto(out *Trace) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Trace.
func (in *Trace) DeepCopy() *Trace {
	if in == nil {
		return nil
	}
	out := new(Trace)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebApi) DeepCopyInto(out *WebApi) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebApi.
func (in *WebApi) DeepCopy() *WebApi {
	if in == nil {
		return nil
	}
	out := new(WebApi)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebBindpoint) DeepCopyInto(out *WebBindpoint) {
	*out = *in
	in.Identity.DeepCopyInto(&out.Identity)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebBindpoint.
func (in *WebBindpoint) DeepCopy() *WebBindpoint {
	if in == nil {
		return nil
	}
	out := new(WebBindpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebConfig) DeepCopyInto(out *WebConfig) {
	*out = *in
	if in.BindPoints != nil {
		in, out := &in.BindPoints, &out.BindPoints
		*out = make([]WebBindpoint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Apis != nil {
		in, out := &in.Apis, &out.Apis
		*out = make([]WebApi, len(*in))
		copy(*out, *in)
	}
	out.Options = in.Options
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebConfig.
func (in *WebConfig) DeepCopy() *WebConfig {
	if in == nil {
		return nil
	}
	out := new(WebConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebOptions) DeepCopyInto(out *WebOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebOptions.
func (in *WebOptions) DeepCopy() *WebOptions {
	if in == nil {
		return nil
	}
	out := new(WebOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *XgressOptions) DeepCopyInto(out *XgressOptions) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new XgressOptions.
func (in *XgressOptions) DeepCopy() *XgressOptions {
	if in == nil {
		return nil
	}
	out := new(XgressOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiController) DeepCopyInto(out *ZitiController) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiController.
func (in *ZitiController) DeepCopy() *ZitiController {
	if in == nil {
		return nil
	}
	out := new(ZitiController)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiController) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiControllerList) DeepCopyInto(out *ZitiControllerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ZitiController, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiControllerList.
func (in *ZitiControllerList) DeepCopy() *ZitiControllerList {
	if in == nil {
		return nil
	}
	out := new(ZitiControllerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiControllerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiControllerSpec) DeepCopyInto(out *ZitiControllerSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiControllerSpec.
func (in *ZitiControllerSpec) DeepCopy() *ZitiControllerSpec {
	if in == nil {
		return nil
	}
	out := new(ZitiControllerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiControllerStatus) DeepCopyInto(out *ZitiControllerStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiControllerStatus.
func (in *ZitiControllerStatus) DeepCopy() *ZitiControllerStatus {
	if in == nil {
		return nil
	}
	out := new(ZitiControllerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiRouter) DeepCopyInto(out *ZitiRouter) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiRouter.
func (in *ZitiRouter) DeepCopy() *ZitiRouter {
	if in == nil {
		return nil
	}
	out := new(ZitiRouter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiRouter) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiRouterList) DeepCopyInto(out *ZitiRouterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ZitiRouter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiRouterList.
func (in *ZitiRouterList) DeepCopy() *ZitiRouterList {
	if in == nil {
		return nil
	}
	out := new(ZitiRouterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiRouterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiRouterSpec) DeepCopyInto(out *ZitiRouterSpec) {
	*out = *in
	in.Model.DeepCopyInto(&out.Model)
	in.Config.DeepCopyInto(&out.Config)
	in.Deployment.DeepCopyInto(&out.Deployment)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiRouterSpec.
func (in *ZitiRouterSpec) DeepCopy() *ZitiRouterSpec {
	if in == nil {
		return nil
	}
	out := new(ZitiRouterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiRouterStatus) DeepCopyInto(out *ZitiRouterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DeploymentConditions != nil {
		in, out := &in.DeploymentConditions, &out.DeploymentConditions
		*out = make([]appsv1.StatefulSetCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiRouterStatus.
func (in *ZitiRouterStatus) DeepCopy() *ZitiRouterStatus {
	if in == nil {
		return nil
	}
	out := new(ZitiRouterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiWebhook) DeepCopyInto(out *ZitiWebhook) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiWebhook.
func (in *ZitiWebhook) DeepCopy() *ZitiWebhook {
	if in == nil {
		return nil
	}
	out := new(ZitiWebhook)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiWebhook) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiWebhookList) DeepCopyInto(out *ZitiWebhookList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ZitiWebhook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiWebhookList.
func (in *ZitiWebhookList) DeepCopy() *ZitiWebhookList {
	if in == nil {
		return nil
	}
	out := new(ZitiWebhookList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ZitiWebhookList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiWebhookSpec) DeepCopyInto(out *ZitiWebhookSpec) {
	*out = *in
	in.Cert.DeepCopyInto(&out.Cert)
	in.DeploymentSpec.DeepCopyInto(&out.DeploymentSpec)
	if in.MutatingWebhookSpec != nil {
		in, out := &in.MutatingWebhookSpec, &out.MutatingWebhookSpec
		*out = make([]admissionregistrationv1.MutatingWebhook, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.ClusterRoleSpec.DeepCopyInto(&out.ClusterRoleSpec)
	in.ServiceAccount.DeepCopyInto(&out.ServiceAccount)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiWebhookSpec.
func (in *ZitiWebhookSpec) DeepCopy() *ZitiWebhookSpec {
	if in == nil {
		return nil
	}
	out := new(ZitiWebhookSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ZitiWebhookStatus) DeepCopyInto(out *ZitiWebhookStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DeploymentConditions != nil {
		in, out := &in.DeploymentConditions, &out.DeploymentConditions
		*out = make([]appsv1.DeploymentCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.IssuerConditions != nil {
		in, out := &in.IssuerConditions, &out.IssuerConditions
		*out = make([]certmanagerv1.IssuerCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CertificateConditions != nil {
		in, out := &in.CertificateConditions, &out.CertificateConditions
		*out = make([]certmanagerv1.CertificateCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ZitiWebhookStatus.
func (in *ZitiWebhookStatus) DeepCopy() *ZitiWebhookStatus {
	if in == nil {
		return nil
	}
	out := new(ZitiWebhookStatus)
	in.DeepCopyInto(out)
	return out
}
