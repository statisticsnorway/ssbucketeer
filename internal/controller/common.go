package controller

import (
	"fmt"
	"strings"
	"text/template"
	"time"
)

const (
	enabledssbucketeerAnnotation       = "dapla.ssb.no/enable-ssbucketeer"
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

// projectTemplate wraps a *template.Template and implements yaml.Unmarshaler interface
// so we can unmarshal a template string into a template.
type projectTemplate struct {
	template *template.Template
}

// Execute wraps template.Template.Execute to provide an easier-to-use interface
// and only allow ProjectTemplateData to be passed as data.
func (t projectTemplate) Execute(data ProjectTemplateData) (string, error) {
	if t.template == nil {
		return "", fmt.Errorf("template is nil, trying to execute for data %q", data)
	}
	sb := strings.Builder{}
	if err := t.template.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (t *projectTemplate) UnmarshalYAML(unmarshal func(any) error) error {
	var templateString string
	if err := unmarshal(&templateString); err != nil {
		return fmt.Errorf("unmarshal template string: %w", err)
	}
	t.template = template.New(templateString)
	if _, err := t.template.Parse(templateString); err != nil {
		return fmt.Errorf("parse template string: %w", err)
	}
	return nil
}

type AccessGroupConfig struct {
	Name            string          `yaml:"name"`
	ProjectTemplate projectTemplate `yaml:"projectTemplate"`
	MaxDuration     time.Duration   `yaml:"maxDuration"`
	ReasonRequired  bool            `yaml:"reasonRequired"`
}

type ProjectTemplateData struct {
	TeamName string
	Stage    string
}
