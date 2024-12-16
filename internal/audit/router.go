package audit

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
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
	Stage             string
}

func (p Payload) Hash() string {
	pJson, _ := json.Marshal(p)
	hash := md5.Sum(pJson)
	return hex.EncodeToString(hash[:])
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

type Router struct {
	recentPayloadHashes []string
	currentHashIndex    int
	sinks               []Sink
}

func NewRouter(sinks ...Sink) *Router {
	return &Router{
		recentPayloadHashes: make([]string, 100),
		sinks:               sinks,
	}
}

func (rs *Router) RecordAll(ctx context.Context, p Payload) error {
	// Deduplication of audit entries
	hash := p.Hash()
	if slices.Contains(rs.recentPayloadHashes, hash) {
		// Payload has recently been recorded
		return nil
	}
	rs.recordHash(hash)

	var err error
	for _, r := range rs.sinks {
		err = errors.Join(err, r.Record(ctx, p))
	}
	return err
}

func (rs *Router) recordHash(hash string) {
	rs.recentPayloadHashes[rs.currentHashIndex] = hash
	rs.currentHashIndex = (rs.currentHashIndex + 1) % len(rs.recentPayloadHashes)
}

func (rs *Router) FlushAll() error {
	var err error
	for _, r := range rs.sinks {
		err = errors.Join(err, r.Flush())
	}
	return err
}
