package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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

func (s *S3Store) appFileKey(appName string) string {
	return s.key("apps", appName, "app.yaml")
}

func (s *S3Store) stackFileKey(appName, stackName string) string {
	return s.key("apps", appName, fmt.Sprintf("%s.yaml", stackName))
}

func (s *S3Store) stackConfigKey(appName, stackName string) string {
	return s.stackFileKey(appName, stackName)
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
	ctx := context.Background()
	prefix := s.key("apps") + "/"

	out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("listing apps from S3: %w", err)
	}

	// Group objects by app name
	type objectEntry struct {
		key  string
		name string // filename within the app dir
	}
	appObjects := make(map[string][]objectEntry)
	for _, obj := range out.Contents {
		rel := strings.TrimPrefix(*obj.Key, prefix)
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) != 2 {
			continue
		}
		appObjects[parts[0]] = append(appObjects[parts[0]], objectEntry{key: *obj.Key, name: parts[1]})
	}

	var cfg config.Config
	for appName, objects := range appObjects {
		var app config.App
		app.Name = appName

		for _, obj := range objects {
			data, err := s.getObject(ctx, obj.key)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", obj.key, err)
			}
			if data == nil {
				continue
			}

			if obj.name == "app.yaml" {
				var af config.AppFile
				if err := yaml.Unmarshal(data, &af); err != nil {
					return nil, fmt.Errorf("parsing app %q: %w", appName, err)
				}
				app.Repo = af.Repo
				app.Path = af.Path
				if app.Path == "" {
					app.Path = "."
				}
			} else if strings.HasSuffix(obj.name, ".yaml") {
				stackName := strings.TrimSuffix(obj.name, ".yaml")
				var sf config.StackFile
				if err := yaml.Unmarshal(data, &sf); err != nil {
					return nil, fmt.Errorf("parsing stack %s/%s: %w", appName, stackName, err)
				}
				env := sf.Env
				if env == "" {
					env = "default"
				}
				app.Stacks = append(app.Stacks, config.Stack{
					Name:      stackName,
					Env:       env,
					Repo:      sf.Repo,
					Path:      sf.Path,
					Branch:    sf.Branch,
					Ref:       sf.Ref,
					DependsOn: sf.DependsOn,
					Org:       sf.Org,
					Project:   sf.Project,
					Bases:     sf.Bases,
				})
			}
		}

		cfg.Apps = append(cfg.Apps, app)
	}

	return &cfg, nil
}

func (s *S3Store) SaveConfig(cfg *config.Config) error {
	ctx := context.Background()

	for _, app := range cfg.Apps {
		// Write app.yaml
		af := config.AppFile{Repo: app.Repo, Path: app.Path}
		data, err := yaml.Marshal(af)
		if err != nil {
			return fmt.Errorf("marshaling app %q: %w", app.Name, err)
		}
		if err := s.putObject(ctx, s.appFileKey(app.Name), data); err != nil {
			return fmt.Errorf("writing app %q to S3: %w", app.Name, err)
		}

		// Write stack files, preserving existing config
		for _, stack := range app.Stacks {
			var existing config.StackFile
			if existingData, getErr := s.getObject(ctx, s.stackFileKey(app.Name, stack.Name)); getErr == nil && existingData != nil {
				// Unmarshal error intentionally ignored: if the existing file is
				// corrupt we simply start fresh with an empty StackFile.
				yaml.Unmarshal(existingData, &existing)
			}

			sf := config.StackFile{
				Env:       stack.Env,
				Repo:      stack.Repo,
				Path:      stack.Path,
				Branch:    stack.Branch,
				Ref:       stack.Ref,
				DependsOn: stack.DependsOn,
				Org:       stack.Org,
				Project:   stack.Project,
				Bases:     stack.Bases,
				Config:    existing.Config,
			}
			data, err := yaml.Marshal(sf)
			if err != nil {
				return fmt.Errorf("marshaling stack %s/%s: %w", app.Name, stack.Name, err)
			}
			if err := s.putObject(ctx, s.stackFileKey(app.Name, stack.Name), data); err != nil {
				return fmt.Errorf("writing stack %s/%s to S3: %w", app.Name, stack.Name, err)
			}
		}
	}

	return nil
}

func (s *S3Store) ReadStackConfig(appName, stackName string) ([]byte, error) {
	data, err := s.getObject(context.Background(), s.stackConfigKey(appName, stackName))
	if err != nil {
		return nil, fmt.Errorf("reading stack config from S3: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	var sf config.StackFile
	if err := yaml.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parsing stack file: %w", err)
	}

	if sf.Config == nil && sf.EncryptionSalt == "" {
		return nil, nil
	}

	pulumiCfg := make(map[string]any)
	if sf.Config != nil {
		pulumiCfg["config"] = sf.Config
	}
	if sf.EncryptionSalt != "" {
		pulumiCfg["encryptionsalt"] = sf.EncryptionSalt
	}
	return yaml.Marshal(pulumiCfg)
}

func (s *S3Store) WriteStackConfig(appName, stackName string, data []byte) error {
	ctx := context.Background()

	var incoming map[string]any
	if err := yaml.Unmarshal(data, &incoming); err != nil {
		return fmt.Errorf("parsing incoming config: %w", err)
	}

	// Read existing file to preserve definition fields
	var sf config.StackFile
	if existing, err := s.getObject(ctx, s.stackConfigKey(appName, stackName)); err == nil && existing != nil {
		yaml.Unmarshal(existing, &sf)
	}

	if cfgVal, ok := incoming["config"]; ok {
		if cfgMap, ok := cfgVal.(map[string]any); ok {
			sf.Config = cfgMap
		}
	} else {
		sf.Config = nil
	}
	if salt, ok := incoming["encryptionsalt"].(string); ok {
		sf.EncryptionSalt = salt
	}

	out, err := yaml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("marshaling stack file: %w", err)
	}

	if err := s.putObject(ctx, s.stackConfigKey(appName, stackName), out); err != nil {
		return fmt.Errorf("writing stack config to S3: %w", err)
	}
	return nil
}

func (s *S3Store) StackFilePath(appName, stackName string) (string, error) {
	return "", fmt.Errorf("stack edit is not supported with S3 backend; use stack show/import instead")
}

func (s *S3Store) baseConfigKey(name string) string {
	return s.key("bases", fmt.Sprintf("%s.yaml", name))
}

func (s *S3Store) ReadBaseConfig(name string) ([]byte, error) {
	data, err := s.getObject(context.Background(), s.baseConfigKey(name))
	if err != nil {
		return nil, fmt.Errorf("reading base config from S3: %w", err)
	}
	return data, nil
}

func (s *S3Store) WriteBaseConfig(name string, data []byte) error {
	if err := s.putObject(context.Background(), s.baseConfigKey(name), data); err != nil {
		return fmt.Errorf("writing base config to S3: %w", err)
	}
	return nil
}

func (s *S3Store) ListBases() ([]string, error) {
	prefix := s.key("bases") + "/"
	out, err := s.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("listing bases from S3: %w", err)
	}
	var names []string
	for _, obj := range out.Contents {
		key := strings.TrimPrefix(*obj.Key, prefix)
		if strings.HasSuffix(key, ".yaml") {
			names = append(names, strings.TrimSuffix(key, ".yaml"))
		}
	}
	return names, nil
}

func (s *S3Store) ReadActiveEnv() (string, error) {
	data, err := s.getObject(context.Background(), s.key("active-env"))
	if err != nil {
		return "", fmt.Errorf("reading active env from S3: %w", err)
	}
	if data == nil {
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *S3Store) WriteActiveEnv(env string) error {
	if err := s.putObject(context.Background(), s.key("active-env"), []byte(env+"\n")); err != nil {
		return fmt.Errorf("writing active env to S3: %w", err)
	}
	return nil
}

func (s *S3Store) DeleteBaseConfig(name string) error {
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.baseConfigKey(name)),
	})
	if err != nil {
		return fmt.Errorf("deleting base config from S3: %w", err)
	}
	return nil
}
