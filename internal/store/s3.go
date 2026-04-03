package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gfyrag/plr/internal/config"
	"gopkg.in/yaml.v3"
)

// S3Store stores configuration in an S3 bucket.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// S3StoreOptions configures the S3 store.
type S3StoreOptions struct {
	Bucket   string
	Region   string
	Prefix   string
	Endpoint string // optional, for testing with MinIO/LocalStack
}

// NewS3Store creates a store backed by an S3 bucket.
func NewS3Store(ctx context.Context, opts S3StoreOptions) (*S3Store, error) {
	var cfgOpts []func(*awsconfig.LoadOptions) error
	if opts.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(opts.Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	var s3Opts []func(*s3.Options)
	if opts.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(opts.Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, s3Opts...)

	return &S3Store{
		client: client,
		bucket: opts.Bucket,
		prefix: opts.Prefix,
	}, nil
}

func (s *S3Store) key(parts ...string) string {
	key := s.prefix
	for _, p := range parts {
		if key != "" && key[len(key)-1] != '/' {
			key += "/"
		}
		key += p
	}
	return key
}

func (s *S3Store) configKey() string {
	return s.key("config.yaml")
}

func (s *S3Store) stackConfigKey(appName, stackName string) string {
	return s.key("stacks", appName, fmt.Sprintf("Pulumi.%s.yaml", stackName))
}

func (s *S3Store) getObject(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, nil
		}
		// Also handle 404 via smithy API error
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3Store) putObject(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *S3Store) LoadConfig() (*config.Config, error) {
	data, err := s.getObject(context.Background(), s.configKey())
	if err != nil {
		return nil, fmt.Errorf("reading config from S3: %w", err)
	}
	if data == nil {
		return &config.Config{}, nil
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	for i := range cfg.Apps {
		if cfg.Apps[i].Path == "" {
			cfg.Apps[i].Path = "."
		}
	}

	return &cfg, nil
}

func (s *S3Store) SaveConfig(cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := s.putObject(context.Background(), s.configKey(), data); err != nil {
		return fmt.Errorf("writing config to S3: %w", err)
	}
	return nil
}

func (s *S3Store) ReadStackConfig(appName, stackName string) ([]byte, error) {
	data, err := s.getObject(context.Background(), s.stackConfigKey(appName, stackName))
	if err != nil {
		return nil, fmt.Errorf("reading stack config from S3: %w", err)
	}
	return data, nil
}

func (s *S3Store) WriteStackConfig(appName, stackName string, data []byte) error {
	if err := s.putObject(context.Background(), s.stackConfigKey(appName, stackName), data); err != nil {
		return fmt.Errorf("writing stack config to S3: %w", err)
	}
	return nil
}
