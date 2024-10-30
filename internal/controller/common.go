package controller

import (
	"errors"
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

	iamConditionKey = "ssbucketeer.namespacedName"
)

var (
	errIamConcurrencyError = errors.New("concurrency error when updating IAM policy")
)

type Auther interface {
	UserIsMemberOf(username, group string) (bool, error)
}
