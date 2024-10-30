package controller

import (
	"fmt"
	"time"
)

const (
	enabledssbucketeerAnnotation       = "dapla.ssb.no/enable-ssbucketeer"
	impersonateGroupAnnotation         = "dapla.ssb.no/impersonate-group"
	mountBucketsAnnotation             = "dapla.ssb.no/mount-buckets"
	mountStandardBucketsAnnotation     = "dapla.ssb.no/mount-standard-buckets"
	serviceContainerAnnotation         = "dapla.ssb.no/service-container-name"
	requestedServiceDurationAnnotation = "dapla.ssb.no/requested-service-duration"

	probeJobStatefulsetAnnotation = "dapla.ssb.no/statefulset-probe-name"
	iamProbeStatus                = "dapla.ssb.no/iam-probe-completed"
	iamProbeRunningPrefix         = "running-replicas-"
	iamProbeDone                  = "done"

	istioExcludedIpRangesAnnotation = "traffic.sidecar.istio.io/excludeOutboundIPRanges"
	gcsfuseOutboundIPRange          = "169.254.169.254/32"

	gkeWIAnnotation = "iam.gke.io/gcp-service-account"

	wiRole = "roles/iam.workloadIdentityUser"

	finalizerName = "dapla.ssb.no/ssbucketeer"

	userNamespacePrefix = "user-ssb-"

	iamConditionKey = "ssbucketeer.namespacedName"
)

var (
	errIamConcurrencyError = fmt.Errorf("concurrency error when updating IAM policy")
)

type Auther interface {
	UserIsMemberOf(username, group string) (bool, error)
}

type AccessGroupConfig struct {
	Name            string        `yaml:"name"`
	ProjectTemplate string        `yaml:"projectTemplate"`
	MaxDuration     time.Duration `yaml:"maxDuration"`
	ReasonRequired  bool          `yaml:"reasonRequired"`
}

type ProjectTemplateData struct {
	TeamName string
	Stage    string
}
