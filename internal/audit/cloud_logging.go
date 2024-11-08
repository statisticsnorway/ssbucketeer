package audit

import "cloud.google.com/go/logging"

var _ Sink = (*CloudLogging)(nil)

type CloudLogging struct {
	Logger *logging.Logger
}

func (r *CloudLogging) Record(p Payload) error {
	r.Logger.Log(logging.Entry{Payload: p})
	return nil
}

func (r *CloudLogging) Flush() error {
	return r.Logger.Flush()
}
