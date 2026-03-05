package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"net"

	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
)

// Step is a single enrichment transformation. Steps are pure — they receive
// the event map and return it modified. Order matters; steps run sequentially.
type Step interface {
	Name() string
	Enrich(ctx context.Context, event map[string]any) error
}

// Pipeline runs a slice of Steps in order. A step failure is logged but does
// not abort the pipeline — partial enrichment is better than no enrichment.
type Pipeline struct {
	steps []Step
}

func NewPipeline(steps ...Step) *Pipeline {
	return &Pipeline{steps: steps}
}

func (p *Pipeline) Run(ctx context.Context, event map[string]any) map[string]any {
	if _, ok := event["enrichment"]; !ok {
		event["enrichment"] = map[string]any{}
	}
	for _, step := range p.steps {
		if err := step.Enrich(ctx, event); err != nil {
			// non-fatal: log and continue
			enrichment := event["enrichment"].(map[string]any)
			enrichment[step.Name()+"Error"] = err.Error()
		}
	}
	return event
}

// ─── GeoIP Enricher ────────────────────────────────────────────────────────

type GeoIPEnricher struct {
	db     *geoip2.Reader
	logger *zap.Logger
}

func NewGeoIPEnricher(dbPath string, logger *zap.Logger) *GeoIPEnricher {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		// GeoIP is optional — log warning, return no-op enricher
		logger.Warn("GeoIP database not found, geo enrichment disabled", zap.String("path", dbPath))
		return &GeoIPEnricher{logger: logger}
	}
	return &GeoIPEnricher{db: db, logger: logger}
}

func (g *GeoIPEnricher) Name() string { return "geoIp" }

func (g *GeoIPEnricher) Enrich(_ context.Context, event map[string]any) error {
	if g.db == nil {
		return nil // gracefully disabled
	}

	// Look for IP in payload tags
	payload, ok := event["payload"].(map[string]any)
	if !ok {
		return nil
	}
	tags, ok := payload["tags"].(map[string]any)
	if !ok {
		return nil
	}
	ipStr, ok := tags["clientIp"].(string)
	if !ok || ipStr == "" {
		return nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP: %s", ipStr)
	}

	record, err := g.db.City(ip)
	if err != nil {
		return fmt.Errorf("geoip lookup: %w", err)
	}

	enrichment, ok := event["enrichment"].(map[string]any)
	if !ok {
		enrichment = map[string]any{}
		event["enrichment"] = enrichment
	}

	geo := map[string]any{
		"country":   record.Country.IsoCode,
		"city":      record.City.Names["en"],
		"latitude":  record.Location.Latitude,
		"longitude": record.Location.Longitude,
	}

	if len(record.Subdivisions) > 0 {
		geo["region"] = record.Subdivisions[0].Names
	}

	enrichment["geoIp"] = geo

	return nil
}

// ─── Service Meta Enricher ─────────────────────────────────────────────────

type serviceRegistry struct {
	Services map[string]map[string]any `json:"services"`
}

type ServiceMetaEnricher struct {
	registry serviceRegistry
	logger   *zap.Logger
}

func NewServiceMetaEnricher(registryPath string, logger *zap.Logger) *ServiceMetaEnricher {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		logger.Warn("service registry not found, service meta enrichment disabled",
			zap.String("path", registryPath))
		return &ServiceMetaEnricher{logger: logger}
	}

	var reg serviceRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		logger.Warn("failed to parse service registry", zap.Error(err))
		return &ServiceMetaEnricher{logger: logger}
	}

	logger.Info("service registry loaded", zap.Int("services", len(reg.Services)))
	return &ServiceMetaEnricher{registry: reg, logger: logger}
}

func (s *ServiceMetaEnricher) Name() string { return "serviceMeta" }

func (s *ServiceMetaEnricher) Enrich(_ context.Context, event map[string]any) error {
	if s.registry.Services == nil {
		return nil
	}

	src, ok := event["source"].(map[string]any)
	if !ok {
		return nil
	}
	serviceName, ok := src["service"].(string)
	if !ok || serviceName == "" {
		return nil
	}

	meta, found := s.registry.Services[serviceName]
	if !found {
		// Unknown service — still useful to record that we looked it up
		meta = map[string]any{"tier": "unknown", "team": "unknown"}
	}

	enrichment := event["enrichment"].(map[string]any)
	enrichment["serviceMeta"] = meta
	return nil
}

// ─── Timestamp Enricher ────────────────────────────────────────────────────

type TimestampEnricher struct{}

func NewTimestampEnricher() *TimestampEnricher { return &TimestampEnricher{} }
func (t *TimestampEnricher) Name() string      { return "timestamp" }

func (t *TimestampEnricher) Enrich(_ context.Context, event map[string]any) error {
	enrichment := event["enrichment"].(map[string]any)
	enrichment["enrichedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	enrichment["enricherVersion"] = "1.0.0"

	// Calculate ingest lag if original timestamp is present
	if ts, ok := event["timestamp"].(string); ok {
		if original, err := time.Parse(time.RFC3339, ts); err == nil {
			enrichment["ingestLagMs"] = time.Since(original).Milliseconds()
		}
	}

	return nil
}
