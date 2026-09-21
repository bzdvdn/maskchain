package bootstrap

import (
	"context"
	"log/slog"

	adapterlogexport "github.com/bzdvdn/maskchain/src/internal/adapters/logexport"
	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// @sk-task log-export#T2.5: build configured export sinks (AC-001, AC-005)
// @sk-task log-export#T3.1: build the S3-compatible sink (AC-006)
// @sk-task log-export#T3.2: build the Langfuse sink (AC-007)
//
// BuildExportSinks constructs the export sinks from configuration. Incomplete
// definitions (missing name/URL/bucket/host) are skipped, and a sink that
// cannot be built (for example an S3 client error) is logged and skipped rather
// than failing startup.
func BuildExportSinks(ctx context.Context, cfg *config.LogExportConfig, log *slog.Logger) []domainlogexport.Sink {
	if cfg == nil {
		return nil
	}
	if log == nil {
		log = slog.Default()
	}
	sinks := make([]domainlogexport.Sink, 0, len(cfg.Webhooks)+len(cfg.S3)+len(cfg.Langfuse))

	for _, w := range cfg.Webhooks {
		if w.Name == "" || w.URL == "" {
			continue
		}
		sinks = append(sinks, adapterlogexport.NewWebhookSink(w.Name, w.URL, w.Secret, cfg.Timeout))
	}

	for _, s := range cfg.S3 {
		if s.Name == "" || s.Bucket == "" {
			continue
		}
		client, err := adapterlogexport.NewS3Client(ctx, s)
		if err != nil {
			log.Warn("log export: S3 sink skipped", slog.String("sink", s.Name), slog.String("error", err.Error()))
			continue
		}
		sinks = append(sinks, adapterlogexport.NewS3Sink(s.Name, s.Bucket, s.Prefix, client))
	}

	for _, l := range cfg.Langfuse {
		if l.Name == "" || l.Host == "" {
			continue
		}
		sinks = append(sinks, adapterlogexport.NewLangfuseSink(l.Name, l.Host, l.PublicKey, l.SecretKey, cfg.Timeout))
	}

	return sinks
}
