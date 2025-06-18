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

package v1alpha1

import (
	"fmt"

	"github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/internal/utils"
	"github.com/openziti/edge-api/rest_model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ZitiRouterSpec defines the desired state of ZitiRouter
type ZitiRouterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Router Name
	// +kubebuilder:validation:MinLength=10
	Name string `json:"name,omitempty"`

	// Controller CR Name
	ZitiControllerName string `json:"zitiControllerName,omitempty"`

	// Controller DNS name
	ZitiCtrlMgmtApi string `json:"zitiCtrlMgmtApi,omitempty"`

	Model RouterCreateModel `json:"model,omitempty"`

	// Router Configuration
	Config Config `json:"config,omitempty"`

	// Deployment Specs
	Deployment RouterDeploymentSpec `json:"deployment,omitempty"`
}

type RouterDeploymentSpec struct {
	// +kubebuilder:default=2
	Replicas *int32 `json:"replicas,omitempty"`
	// +kubebuilder:default={}
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// +kubebuilder:default={}
	Labels map[string]string `json:"labels,omitempty"`
	// +kubebuilder:default={}
	Annotations map[string]string `json:"annotations,omitempty"`
	Container   corev1.Container  `json:"container,omitempty"`
	// +kubebuilder:default=false
	// +kubebuilder:validation:Enum=true;false
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// +kubebuilder:default={}
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	// +kubebuilder:default=ClusterFirstWithHostNet
	// +kubebuilder:validation:Enum=ClusterFirst;ClusterFirstWithHostNet;Default
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
	// +kubebuilder:default=default-scheduler
	SchedulerName string `json:"schedulerName,omitempty"`
	// +kubebuilder:default=Always
	// +kubebuilder:validation:Enum=Always;OnFailure;Never
	RestartPolicy corev1.RestartPolicy `json:"restartPolicy,omitempty"`
	// +kubebuilder:default={}
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// +kubebuilder:default=30
	TerminationGracePeriodSeconds *int64                           `json:"terminationGracePeriodSeconds,omitempty"`
	Volumes                       []corev1.Volume                  `json:"volumes,omitempty"`
	UpdateStrategy                appsv1.StatefulSetUpdateStrategy `json:"strategy,omitempty"`
	// Minimum number of seconds for which a newly created pod should be ready
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Minimum=0
	MinReadySeconds int32 `json:"progressDeadlineSeconds,omitempty"`
	// Revision History Limit
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Minimum=0
	RevisionHistoryLimit                 *int32                                                  `json:"revisionHistoryLimit,omitempty"`
	StorageClassName                     *string                                                 `json:"storageClassName,omitempty"`
	VolumeMode                           *corev1.PersistentVolumeMode                            `json:"volumeMode,omitempty"`
	PersistentVolumeClaimRetentionPolicy *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy `json:"persistentVolumeClaimRetentionPolicy,omitempty"`
	Ordinals                             *appsv1.StatefulSetOrdinals                             `json:"ordinals,omitempty"`
	// Log Verbose Level
	// +kubebuilder:default:=2
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5
	LogLevel int32 `json:"logLevel,omitempty"`
}

type RouterCreateModel struct {

	// app data
	AppData Tags `json:"appData,omitempty"`
	// router cost
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=0
	Cost int64 `json:"cost,omitempty"`
	// disabled
	// +kubebuilder:default=false
	Disabled bool `json:"disabled,omitempty"`
	// Is tunneler enabled
	// +kubebuilder:default=false
	IsTunnelerEnabled bool `json:"isTunnelerEnabled,omitempty"`
	// name
	Name string `json:"name"`
	// no traversal
	// +kubebuilder:default=false
	NoTraversal bool `json:"noTraversal,omitempty"`
	// role attributes
	RoleAttributes rest_model.Attributes `json:"roleAttributes,omitempty"`
	// tag
	Tags Tags `json:"tags,omitempty"`
}

type Tags struct {
	SubTags map[string]string `json:"subTags,omitempty"`
}

type Config struct {
	Identity       Identity            `json:"identity,omitempty"`
	EnableDebugOps bool                `json:"enableDebugOps,omitempty"`
	Forwarder      ForwarderOptions    `json:"forwarder,omitempty"`
	Trace          Trace               `json:"trace,omitempty"`
	Profile        Profile             `json:"profile,omitempty"`
	Ctrl           Ctrl                `json:"ctrl,omitempty"`
	Link           Link                `json:"link,omitempty"`
	Dialers        []EdgeDialer        `json:"dialers,omitempty"`
	Listeners      []EdgeListener      `json:"listeners,omitempty"`
	Transport      map[string]string   `json:"transport,omitempty"`
	Metrics        Metrics             `json:"metrics,omitempty"`
	HealthChecks   HealthChecks        `json:"healthChecks,omitempty"`
	ConnectEvents  ConnectEventsConfig `json:"connectEvents,omitempty"`
	Plugins        []string            `json:"plugins,omitempty"`
	Edge           EdgeConfig          `json:"edge,omitempty"`
	Web            WebConfig           `json:"web,omitempty"`
	Proxy          map[string]string   `json:"proxy,omitempty"`
}

type Identity struct {
	Key            string       `json:"key,omitempty"`
	Cert           string       `json:"cert,omitempty"`
	ServerCert     string       `json:"server_cert,omitempty"`
	ServerKey      string       `json:"server_key,omitempty"`
	AltServerCerts []ServerPair `json:"alt_server_certs,omitempty"`
	CA             string       `json:"ca,omitempty"`
}

type ServerPair struct {
	ServerCert string `json:"server_cert,omitempty"`
	ServerKey  string `json:"server_key,omitempty"`
}

type Trace struct {
	Path string `json:"path,omitempty"`
}

type Metrics struct {
	ReportInterval        int64 `json:"reportInterval,omitempty"`
	IntervalAgeThreshold  int64 `json:"intervalAgeThreshold,omitempty"`
	MessageQueueSize      int   `json:"messageQueueSize,omitempty"`
	EventQueueSize        int   `json:"eventQueueSize,omitempty"`
	EnableDataDelayMetric bool  `json:"enableDataDelayMetric,omitempty"`
}

type HealthChecks struct {
	CtrlPingCheck CtrlPingCheck `json:"ctrlPingCheck,omitempty"`
	LinkCheck     LinkCheck     `json:"linkCheck,omitempty"`
}

type CtrlPingCheck struct {
	Interval     int64 `json:"interval,omitempty"`
	Timeout      int64 `json:"timeout,omitempty"`
	InitialDelay int64 `json:"initialDelay,omitempty"`
}

type LinkCheck struct {
	MinLinks     int   `json:"minLinks,omitempty"`
	Interval     int64 `json:"interval,omitempty"`
	InitialDelay int64 `json:"initialDelay,omitempty"`
}

type Profile struct {
	Memory Memory `json:"memory,omitempty"`
	CPU    CPU    `json:"cpu,omitempty"`
}

type Memory struct {
	Path       string `json:"path,omitempty"`
	Interval   int64  `json:"interval,omitempty"`
	IntervalMs int64  `json:"intervalMs,omitempty"`
}

type CPU struct {
	Path string `json:"path,omitempty"`
}

type Ctrl struct {
	Endpoint              string           `json:"endpoint,omitempty"`
	Endpoints             []string         `json:"endpoints,omitempty"`
	Bind                  string           `json:"bind,omitempty"`
	DefaultRequestTimeout int64            `json:"defaultRequestTimeout,omitempty"`
	Options               ChannelOptions   `json:"options,omitempty"`
	EndpointsFile         string           `json:"endpointsFile,omitempty"`
	Heartbeats            HeartbeatOptions `json:"heartbeats,omitempty"`
}

type HeartbeatOptions struct {
	SendInterval             int64 `json:"sendInterval,omitempty"`
	CheckInterval            int64 `json:"checkInterval,omitempty"`
	CloseUnresponsiveTimeout int64 `json:"closeUnresponsiveTimeout,omitempty"`
}

type ChannelOptions struct {
	OutQueueSize           int   `json:"outQueueSize,omitempty"`
	MaxQueuedConnects      int   `json:"maxQueuedConnects,omitempty"`
	MaxOutstandingConnects int   `json:"maxOutstandingConnects,omitempty"`
	ConnectTimeoutMs       int64 `json:"connectTimeoutMs,omitempty"`
	WriterTimeout          int64 `json:"writerTimeout,omitempty"`
}

type Link struct {
	Listeners []LinkListener `json:"listeners,omitempty"`
	Dialers   []LinkDialer   `json:"dialers,omitempty"`
}

type LinkDialer struct {
	// Indicates if a single connection should be made for all data or separate connections
	Split                bool             `json:"split,omitempty"`
	Binding              string           `json:"binding,omitempty"`
	Bind                 string           `json:"bind,omitempty"`
	Groups               []string         `json:"groups,omitempty"`
	Options              ChannelOptions   `json:"options,omitempty"`
	HealthyDialBackoff   BackoffParamters `json:"healthyDialBackoff,omitempty"`
	UnhealthyDialBackoff BackoffParamters `json:"unhealthyDialBackoff,omitempty"`
}

type LinkListener struct {
	Binding   string `json:"binding,omitempty"`
	Bind      string `json:"bind,omitempty"`
	Advertise string `json:"advertise,omitempty"`
	// Dialers will only attempt to dial listeners who have at least one group in common with them
	Groups  []string       `json:"groups,omitempty"`
	Options ChannelOptions `json:"options,omitempty"`
}

type XgressOptions struct {
	Mtu int32 `json:"mtu,omitempty"`
	// +kubebuilder:default=false
	RandomDrops bool `json:"randomDrops,omitempty"`
	// if randomDrops is enabled, will drop 1 in N payloads, used for testing only
	// +kubebuilder:default=100
	Drop1InN int32 `json:"drop1InN,omitempty"`
	// The number of transmit payload to queue
	// +kubebuilder:default=1
	TxQueueSize int32 `json:"txQueueSize,omitempty"`
	// optional integer that sets the starting window sizes
	// +kubebuilder:default=16384

	TxPortalStartSize int `json:"txPortalStartSize,omitempty"`
	// Optional integer that sets the minimum window size
	// +kubebuilder:default=16384
	TxPortalMinSize int32 `json:"txPortalMinSize,omitempty"`
	// Optional integer that sets the maximum window size
	// +kubebuilder:default=410241024
	TxPortalMaxSize int32 `json:"txPortalMaxSize,omitempty"`
	// Optional number of successful transmits that triggers the window size to be scaled by txPortalIncreaseScale
	// +kubebuilder:default=224
	TxPortalIncreaseThresh int32 `json:"txPortalIncreaseThresh,omitempty"`
	// Optional scale factor to increase the window size by
	// +kubebuilder:default="1.0"
	TxPortalIncreaseScale string `json:"txPortalIncreaseScale,omitempty"`
	// Optional number of retransmissions that triggers the window size to be scaled by txPortalRetxScale
	// +kubebuilder:default=64
	TxPortalRetxThresh int32 `json:"txPortalRetxThresh,omitempty"`
	// Optional factor used to scale the window size when txPortalRetxThresh is reached
	// +kubebuilder:default="0.75"
	TxPortalRetxScale string `json:"txPortalRetxScale,omitempty"`
	// Optional number of duplicate ACKs that triggers the window size to be scaled by txPortalDupAckScale
	// +kubebuilder:default=64
	TxPortalDupAckThresh int32 `json:"txPortalDupAckThresh,omitempty"`
	// Optional factor used to scale the window size when txPortalDupAckThresh is reached
	// +kubebuilder:default="0.9"
	TxPortalDupAckScale string `json:"txPortalDupAckScale,omitempty"`
	// Optional size of the receive buffer
	// +kubebuilder:default=410241024

	RxBufferSize int32 `json:"rxBufferSize,omitempty"`
	// Optional number of milliseconds to wait before attempting to retransmit
	// +kubebuilder:default=200
	RetxStartMs int32 `json:"retxStartMs,omitempty"`
	// Optional factor to scale retxStartMs based on RTT
	// +kubebuilder:default="1.5"
	RetxScale string `json:"retxScale,omitempty"`
	// Optional number of milliseconds to add to retxStartMs when calculating new retransmission thresholds
	// +kubebuilder:default=0
	RetxAddMs int32 `json:"retxAddMs,omitempty"`
	// Optional amount of time to wait for buffers to empty before closing a connection in seconds
	// +kubebuilder:default=30

	MaxCloseWaitMs int64 `json:"maxCloseWaitMs,omitempty"`
	// Optional amount of time to wait for circuit creation in seconds
	// +kubebuilder:default=30
	GetCircuitTimeout int64 `json:"getCircuitTimeout,omitempty"`
	// Optional amount of time to wait for a circuit to start in seconds
	// +kubebuilder:default=180
	CircuitStartTimeout int64 `json:"circuitStartTimeout,omitempty"`
	// Optional amount of time to wait for dialed connections to connect in seconds
	// +kubebuilder:default=0
	ConnectTimeout int64 `json:"connectTimeout,omitempty"`
}

type BackoffParamters struct {
	// Duration specifying the minimum time between dial attempts in ms
	// Default is 1m
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=8.64e+7
	MinRetryInterval int64 `json:"minRetryInterval,omitempty"`
	// Duration specifying the maximum time between dial attempts in ms
	// Default is 1h
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=8.64e+7
	MaxRetryInterval int64 `json:"maxRetryInterval,omitempty"`
	// Factor by which to increase the retry interval between failed dial attempts
	// Default is 10
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	RetryBackoffFactor int `json:"retryBackoffFactor,omitempty"`
}

type EdgeListener struct {
	Binding string      `json:"binding,omitempty"`
	Bind    string      `json:"address,omitempty"`
	Options EdgeOptions `json:"options,omitempty"`
}

type EdgeDialer struct {
	Binding string        `json:"binding,omitempty"`
	Options XgressOptions `json:"options,omitempty"`
}

type EdgeOptions struct {
	Advertise string        `json:"advertise,omitempty"`
	Options   XgressOptions `json:"options,omitempty"`
}

type ConnectEventsConfig struct {
	Enabled          bool  `json:"enabled,omitempty"`
	BatchInterval    int64 `json:"batchInterval,omitempty"`
	MaxQueuedEvents  int64 `json:"maxQueuedEvents,omitempty"`
	FullSyncInterval int64 `json:"fullSyncInterval,omitempty"`
}

type ForwarderOptions struct {
	FaultTxInterval          int64  `json:"faultTxInterval,omitempty"`
	IdleCircuitTimeout       int64  `json:"idleCircuitTimeout,omitempty"`
	IdleTxInterval           int64  `json:"idleTxInterval,omitempty"`
	LinkDialQueueLength      uint16 `json:"linkDialQueueLength,omitempty"`
	LinkDialWorkerCount      uint16 `json:"linkDialWorkerCount,omitempty"`
	RateLimitedQueueLength   uint16 `json:"rateLimitedQueueLength,omitempty"`
	RateLimitedWorkerCount   uint16 `json:"rateLimitedWorkerCount,omitempty"`
	UnresponsiveLinkTimeout  int64  `json:"unresponsiveLinkTimeout,omitempty"`
	XgressCloseCheckInterval int64  `json:"xgressCloseCheckInterval,omitempty"`
	XgressDialQueueLength    uint16 `json:"xgressDialQueueLength,omitempty"`
	XgressDialWorkerCount    uint16 `json:"xgressDialWorkerCount,omitempty"`
	XgressDialDwellTime      int64  `json:"xgressDialDwellTime,omitempty"`
}

type EdgeConfig struct {
	ApiProxy                   ApiProxy `json:"apiProxy,omitempty"`
	Csr                        Csr      `json:"csr,omitempty"`
	HeartbeatIntervalSeconds   int      `json:"heartbeatIntervalSeconds,omitempty"`
	SessionValidateChunkSize   uint32   `json:"sessionValidateChunkSize,omitempty"`
	SessionValidateMinInterval int64    `json:"sessionValidateMinInterval,omitempty"`
	SessionValidateMaxInterval int64    `json:"sessionValidateMaxInterval,omitempty"`
	ForceExtendEnrollment      bool     `json:"forceExtendEnrollment,omitempty"`
	Db                         string   `json:"db,omitempty"`
	DbSaveIntervalSeconds      int64    `json:"dbSaveIntervalSeconds,omitempty"`
}

type Csr struct {
	Sans               Sans   `json:"sans,omitempty"`
	Country            string `json:"country,omitempty"`
	Locality           string `json:"locality,omitempty"`
	Organization       string `json:"organization,omitempty"`
	OrganizationalUnit string `json:"organizationalUnit,omitempty"`
	Province           string `json:"province,omitempty"`
}

type ApiProxy struct {
	Listener string `json:"listener,omitempty"`
	Upstream string `json:"upstream,omitempty"`
}

type Sans struct {
	DnsAddresses      []string `json:"dnsAddresses,omitempty"`
	IpAddresses       []string `json:"ipAddresses,omitempty"`
	IpAddressesParsed []byte   `json:"ipAddressesParsed,omitempty"`
	EmailAddresses    []string `json:"emailAddresses,omitempty"`
	UriAddresses      []string `json:"uriAddresses,omitempty"`
}

type WebConfig struct {
	// +kubebuilder:default="health-check"
	Name string `json:"name,omitempty"`
	// +kubebuilder:default={{"interface":"0.0.0.0:8081","address":""}}
	BindPoints []WebBindpoint `json:"bindPoints,omitempty"`
	// +kubebuilder:default={{"binding":"health-checks"}}
	Apis    []WebApi   `json:"apis,omitempty"`
	Options WebOptions `json:"options,omitempty"`
}

type WebBindpoint struct {
	Interface  string   `json:"interface,omitempty"`
	Identity   Identity `json:"identity,omitempty"`
	Address    string   `json:"address,omitempty"`
	NewAddress string   `json:"newAddress,omitempty"`
}

type WebApi struct {
	Binding string `json:"binding,omitempty"`
}

type WebOptions struct {
	IdleTimeout   int64  `json:"idleTimeout,omitempty"`
	ReadTimeout   int64  `json:"readTimeout,omitempty"`
	WriteTimeout  int64  `json:"writeTimeout,omitempty"`
	MinTlsVersion string `json:"minTlsVersion,omitempty"`
	MaxTlsVersion string `json:"maxTlsVersion,omitempty"`
}

// ZitiRouterStatus defines the observed state of ZitiRouter
type ZitiRouterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Replicas int32 `json:"replicas,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.deployment.replicas,statuspath=.status.replicas,selectorpath=.status.selector
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.deployment.replicas",description="The number of desired replicas"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.replicas",description="The number of ready replicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// ZitiRouter is the Schema for the zitirouters API
type ZitiRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiRouterSpec   `json:"spec,omitempty"`
	Status ZitiRouterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiRouterList contains a list of ZitiRouter
type ZitiRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiRouter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiRouter{}, &ZitiRouterList{})
}

func (r *ZitiRouter) GetDefaults() *ZitiRouterSpec {
	return &ZitiRouterSpec{
		Name:               r.Name,
		ZitiControllerName: r.Spec.ZitiControllerName,
		ZitiCtrlMgmtApi:    r.Spec.ZitiCtrlMgmtApi,
		Deployment: RouterDeploymentSpec{
			Replicas:                      &[]int32{2}[0],
			Selector:                      r.GetDefaultSelector(),
			Labels:                        r.GetDefaultLabels(),
			Annotations:                   r.GetDefaultAnnotations(),
			Container:                     r.GetDefaultContainer(),
			HostNetwork:                   false,
			DNSConfig:                     nil,
			DNSPolicy:                     corev1.DNSClusterFirstWithHostNet,
			RestartPolicy:                 corev1.RestartPolicyAlways,
			SchedulerName:                 "default-scheduler",
			SecurityContext:               r.GetDefaultSecurityContext(),
			TerminationGracePeriodSeconds: &[]int64{30}[0],
			Volumes: []corev1.Volume{
				{
					Name: r.Name,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: r.Name,
						},
					},
				},
				{
					Name: "ziti-router-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: &[]int32{292}[0],
							LocalObjectReference: corev1.LocalObjectReference{
								Name: r.Name + "-config",
							},
						},
					},
				},
			},
			UpdateStrategy:                       r.GetDefaultStrategy(),
			MinReadySeconds:                      10,
			RevisionHistoryLimit:                 &[]int32{10}[0],
			StorageClassName:                     &[]string{"standard"}[0],
			VolumeMode:                           &[]corev1.PersistentVolumeMode{"Filesystem"}[0],
			PersistentVolumeClaimRetentionPolicy: r.GetDefaultPersistentVolumeClaimRetentionPolicy(),
			Ordinals:                             r.GetDefaultOrdinals(),
			LogLevel:                             2,
		},
	}
}

func (r *ZitiRouter) GetDefaultContainer() corev1.Container {
	return corev1.Container{
		Name:            "ziti-router",
		Image:           "docker.io/openziti/ziti-router:latest",
		ImagePullPolicy: "Always",
		Args: []string{
			"run",
			"/etc/ziti/config/ziti-router.yaml",
		},
		Command: []string{
			"/entrypoint.bash",
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "edge",
				ContainerPort: 9443,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ZITI_ENROLL_TOKEN",
				Value: "",
			},
			{
				Name:  "ZITI_BOOTSTRAP",
				Value: "true",
			},
			{
				Name:  "ZITI_BOOTSTRAP_ENROLLMENT",
				Value: "true",
			},
			{
				Name:  "ZITI_BOOTSTRAP_CONFIG",
				Value: "false",
			},
			{
				Name:  "ZITI_AUTO_RENEW_CERTS",
				Value: "true",
			},
			{
				Name:  "ZITI_HOME",
				Value: "/etc/ziti/config",
			},
			{
				Name:  "ZITI_ROUTER_NAME",
				Value: r.ObjectMeta.Name,
			},
			{
				Name:  "ZITI_ROUTER_IDENTITY_NAME",
				Value: "",
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 10,
			TimeoutSeconds:      1,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"ziti agent stats",
					},
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: 10,
			TimeoutSeconds:      1,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"ziti agent stats",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("400m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      r.Name,
				MountPath: "/etc/ziti/config",
			},
			{
				Name:      "ziti-router-config",
				MountPath: "/etc/ziti/config/" + r.GetDefaultConfigMapKey(),
				SubPath:   r.GetDefaultConfigMapKey(),
			},
		},
	}
}

func (r *ZitiRouter) GetDefaultSelector() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: utils.FilterLabels(r.GetDefaultLabels()),
	}
}

func (r *ZitiRouter) GetDefaultSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		FSGroup: &[]int64{2171}[0],
	}
}

func (r *ZitiRouter) GetDefaultStrategy() appsv1.StatefulSetUpdateStrategy {
	return appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
		RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{
			Partition: &[]int32{0}[0],
		},
	}
}

func (r *ZitiRouter) GetDefaultPersistentVolumeClaimRetentionPolicy() *appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy {
	return &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
		WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
	}
}

func (r *ZitiRouter) GetDefaultOrdinals() *appsv1.StatefulSetOrdinals {
	return &appsv1.StatefulSetOrdinals{
		Start: 0,
	}
}

// GetDefaultLabels returns the default labels for the ZitiRouter resource.
func (r *ZitiRouter) GetDefaultLabels() map[string]string {
	return map[string]string{
		"app":                          r.Name,
		"app.kubernetes.io/name":       r.Name + "-" + r.Namespace,
		"app.kubernetes.io/part-of":    r.Name + "-operator",
		"app.kubernetes.io/managed-by": r.Name + "-controller",
		"app.kubernetes.io/component":  "router",
		"router.openziti.io/enabled":   "true",
	}
}

func (r *ZitiRouter) GetDefaultAnnotations() map[string]string {
	return map[string]string{}
}

func (r *ZitiRouter) GetDefaultFinalizer() []string {
	return []string{
		"kubernetes.openziti.io/zitirouter",
	}
}

func (r *ZitiRouter) GetDefaultDeploymentName() string {
	return r.Name + "-deployment"
}

func (r *ZitiRouter) GetDefaultServiceAccountName() string {
	return r.Name + "-service-account"
}

func (r *ZitiRouter) GetDefaultServiceName() string {
	return r.Name + "-service"
}

func (r *ZitiRouter) GetDefaultServicePort() int32 {
	return 443
}

func (r *ZitiRouter) GetDefaultClusterDomain() string {
	return "cluster.local"
}

func (r *ZitiRouter) GetDefaultConfigMapName() string {
	return r.Name + "-config"
}

func (r *ZitiRouter) GetDefaultConfigMapKey() string {
	return "ziti-router.yaml"
}

func (r *ZitiRouter) GetDefaultConfigMapData() map[string]string {

	edgeAddress := fmt.Sprintf("%s-service.%s.svc.%s", r.ObjectMeta.Name, r.Namespace, r.GetDefaultClusterDomain())
	configTemplate := `v: 3
identity:
  cert: /etc/ziti/config/%s.cert
  server_cert: /etc/ziti/config/%s.server.chain.cert
  key: /etc/ziti/config/%s.key
  ca: /etc/ziti/config/%s.cas

ctrl:
  endpoint: %s

link:
  dialers:
    - binding: transport

listeners:
  - binding: edge
    address: tls:0.0.0.0:%d
    options:
      advertise: %s:%d
      connectTimeoutMs: 5000
      getSessionTimeout: 60s

edge:
  csr:
    country: US
    province: NC
    locality: Charlotte
    organization: NetFoundry
    organizationalUnit: Ziti
    sans:
      dns:
        - localhost
        - %s
      ip:
        - 127.0.0.1
      email:
      uri:

web:
  - name: health-check
    bindPoints:
      - interface: 0.0.0.0:8081
        address: 0.0.0.0:8081 	
    apis:
      - binding: health-checks
`
	configTemplate = fmt.Sprintf(
		configTemplate,
		r.ObjectMeta.Name,
		r.ObjectMeta.Name,
		r.ObjectMeta.Name,
		r.ObjectMeta.Name,
		r.Spec.Config.Ctrl.Endpoint,
		r.Spec.Deployment.Container.Ports[0].ContainerPort,
		edgeAddress,
		r.GetDefaultServicePort(),
		edgeAddress,
	)
	return map[string]string{
		r.GetDefaultConfigMapKey(): configTemplate,
	}
}
