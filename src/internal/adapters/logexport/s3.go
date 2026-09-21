package logexport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	domainlogexport "github.com/bzdvdn/maskchain/src/internal/domain/logexport"
	appconfig "github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// putObjectAPI is the subset of the S3 client used by the sink, so tests can
// substitute a fake without network access.
type putObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// @sk-task log-export#T3.1: S3-compatible object-store sink (AC-006)
//
// S3Sink writes one newline-delimited JSON object per tenant under a
// tenant + date-partitioned key.
type S3Sink struct {
	name   string
	bucket string
	prefix string
	client putObjectAPI
}

// NewS3Sink builds a sink over an S3-compatible client.
func NewS3Sink(name, bucket, prefix string, client putObjectAPI) *S3Sink {
	return &S3Sink{name: name, bucket: bucket, prefix: prefix, client: client}
}

// NewS3Client builds a real S3-compatible client from config. A custom endpoint
// (MinIO, GCS interoperability) switches to path-style addressing.
func NewS3Client(ctx context.Context, cfg appconfig.LogExportS3Config) (*s3.Client, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3 sink %s: load aws config: %w", cfg.Name, err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})
	return client, nil
}

func (s *S3Sink) Name() string { return s.name }

// Send writes one object per tenant present in the batch.
func (s *S3Sink) Send(ctx context.Context, records []domainlogexport.Record) error {
	if len(records) == 0 {
		return nil
	}
	byTenant := make(map[string][]domainlogexport.Record)
	order := make([]string, 0, len(records))
	for _, r := range records {
		if _, seen := byTenant[r.Tenant]; !seen {
			order = append(order, r.Tenant)
		}
		byTenant[r.Tenant] = append(byTenant[r.Tenant], r)
	}

	now := time.Now().UTC()
	for _, tenant := range order {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		for _, wr := range toWire(byTenant[tenant]) {
			if err := enc.Encode(wr); err != nil {
				return fmt.Errorf("s3 sink %s: encode: %w", s.name, err)
			}
		}
		key := s.objectKey(tenant, now, newEventID())
		if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(buf.Bytes()),
			ContentType: aws.String("application/x-ndjson"),
		}); err != nil {
			return fmt.Errorf("s3 sink %s: put %s: %w", s.name, key, err)
		}
	}
	return nil
}

// objectKey builds "<prefix>/<tenant>/<YYYY>/<MM>/<DD>/<event>.jsonl".
func (s *S3Sink) objectKey(tenant string, ts time.Time, eventID string) string {
	parts := make([]string, 0, 6)
	if p := strings.Trim(s.prefix, "/"); p != "" {
		parts = append(parts, p)
	}
	parts = append(parts, tenant, ts.Format("2006"), ts.Format("01"), ts.Format("02"), eventID+".jsonl")
	return strings.Join(parts, "/")
}
