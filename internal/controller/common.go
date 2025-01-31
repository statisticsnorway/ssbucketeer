package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/statisticsnorway/ssbucketeer/internal/template"
)

const (
	impersonateGroupAnnotation         = "dapla.ssb.no/impersonate-group"
	mountBucketsAnnotation             = "dapla.ssb.no/mount-buckets"
	mountStandardBucketsAnnotation     = "dapla.ssb.no/mount-standard-buckets"
	serviceContainerAnnotation         = "dapla.ssb.no/service-container-name"
	requestedServiceDurationAnnotation = "dapla.ssb.no/requested-service-duration"
	accessReasonAnnotation             = "dapla.ssb.no/access-reason"

	probeJobStatefulsetAnnotation = "dapla.ssb.no/statefulset-probe-name"
	iamProbeStatus                = "dapla.ssb.no/iam-probe-completed"
	iamProbeRunningPrefix         = "running-replicas-"
	iamProbeDone                  = "done"

	daplaTeamLabel  = "dapla.ssb.no/team"
	daplaGroupLabel = "dapla.ssb.no/group"

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
	Name            string                                          `yaml:"name"`
	ProjectTemplate template.AnonymousTemplate[ProjectTemplateData] `yaml:"projectTemplate"`
	MaxDuration     time.Duration                                   `yaml:"maxDuration"`
	ReasonRequired  bool                                            `yaml:"reasonRequired"`
}

func (c AccessGroupConfig) ToTeam(group string) string {
	return strings.TrimSuffix(strings.TrimSuffix(group, c.Name), "-")
}

type ProjectTemplateData struct {
	TeamName string
	Stage    string
}

type SharedBucketTemplateData struct {
	TeamName        string
	BucketShortName string
	Stage           string
}

type SharedBucketTemplate = template.AnonymousTemplate[SharedBucketTemplateData]

type AccessGroupConfigs []AccessGroupConfig

func (cs AccessGroupConfigs) GetConfig(group string) *AccessGroupConfig {
	for _, c := range cs {
		if team := strings.TrimSuffix(group, c.Name); team != group {
			return &c
		}
	}
	return nil
}
