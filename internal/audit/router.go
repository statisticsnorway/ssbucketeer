package audit

import (
	"context"
	"errors"
	"time"
)

type Payload struct {
	UserPrincipalName string
	TeamName          string
	AccessGroup       string
	GroupType         string
	Reason            string
	StartTime         time.Time
	EndTime           time.Time
	Duration          string
	Service           Service
}

type Service struct {
	Chart    ChartMeta
	Instance InstanceMeta
}

type ChartMeta struct {
	Name    string
	Version string
}

type InstanceMeta struct {
	Name      string
	Namespace string
}

type Sink interface {
	Record(context.Context, Payload) error
	Flush() error
}

type Router []Sink

func (rs Router) RecordAll(ctx context.Context, p Payload) error {
	var err error
	for _, r := range rs {
		err = errors.Join(err, r.Record(ctx, p))
	}
	return err
}

func (rs Router) FlushAll() error {
	var err error
	for _, r := range rs {
		err = errors.Join(err, r.Flush())
	}
	return err
}
