package audit

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/logging"
)

var _ Sink = (*CloudLogging)(nil)

type CloudLogging struct {
	Logger *logging.Logger
}

func NewCloudLoggingSink(ctx context.Context, config map[string]string) (*CloudLogging, error) {
	projectId, ok := config["projectId"]
	if !ok {
		return nil, fmt.Errorf("missing `projectId` in config: %v", config)
	}

	logName, ok := config["logName"]
	if !ok {
		logName = "ssbucketeer"
	}

	client, err := logging.NewClient(ctx, fmt.Sprintf("projects/%s", projectId))
	if err != nil {
		return nil, err
	}

	return &CloudLogging{Logger: client.Logger(logName)}, nil
}

func (r *CloudLogging) Record(p Payload) error {
	idString, err := json.Marshal(p)
	if err != nil {
		return err
	}
	h := sha256.New()
	h.Write([]byte(idString))
	r.Logger.Log(logging.Entry{
		Payload:   p,
		Timestamp: p.StartTime,
		InsertID:  string(h.Sum(nil)),
	})
	return nil
}

func (r *CloudLogging) Flush() error {
	return r.Logger.Flush()
}
