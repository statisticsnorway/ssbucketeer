package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"text/template"

	"cloud.google.com/go/storage"
	"golang.org/x/exp/rand"
)

// CloudStorage implements the Sink interface for routing logs to a Google Cloud Storage bucket.
type CloudStorage struct {
	Bucket         *storage.BucketHandle
	FilenameFormat *template.Template
}

var _ Sink = (*CloudStorage)(nil)

func NewCloudStorageSink(ctx context.Context, config map[string]string) (*CloudStorage, error) {
	bucket, ok := config["bucket"]
	if !ok {
		return nil, errors.New("config is missing `bucket`")
	}
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	bucketHandle := storageClient.Bucket(bucket)

	// Default format for the filename of the access log entry. If the time format looks confusing,
	// Google "golang crazy time format"
	filenameFormat := `access-grant_{{ .UserPrincipalName }}_{{ .AccessGroup }}_{{ .StartTime.UTC.Format "2006-01-02_15:04:05" }}_{{ randomString 6 }}.json`
	if tplStr, ok := config["filenameFormat"]; ok {
		filenameFormat = tplStr
	}
	tpl := template.New("")
	// We add a convenience template function for generating random strings
	tpl = tpl.Funcs(template.FuncMap{"randomString": func(length int) string {
		var chars = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
		randomChars := make([]rune, length)
		for i := range randomChars {
			randomChars[i] = chars[rand.Intn(len(chars))]
		}
		return string(randomChars)
	}})
	tpl, err = tpl.Parse(filenameFormat)
	if err != nil {
		return nil, err
	}

	return &CloudStorage{
		Bucket:         bucketHandle,
		FilenameFormat: tpl,
	}, nil
}

func (s *CloudStorage) Record(ctx context.Context, p Payload) error {
	var filenameBuilder strings.Builder
	if err := s.FilenameFormat.Execute(&filenameBuilder, p); err != nil {
		return err
	}
	objectHandle := s.Bucket.Object(filenameBuilder.String())
	objectWriter := objectHandle.NewWriter(ctx)
	objectEncoder := json.NewEncoder(objectWriter)
	return objectEncoder.Encode(p)
}

func (s *CloudStorage) Flush() error {
	return nil
}
