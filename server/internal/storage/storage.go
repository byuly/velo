package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage defines the contract for blob operations needed by reel generation.
type Storage interface {
	// Download fetches an S3 object and writes it to localPath.
	Download(ctx context.Context, bucket, key, localPath string) error

	// Upload writes a local file to the given bucket+key.
	Upload(ctx context.Context, bucket, key, localPath string) error

	// ReelURL returns the public CDN URL for a reel.
	ReelURL(key string) string
}

// Compile-time interface check.
var _ Storage = (*S3Client)(nil)

// S3Client implements Storage using AWS S3.
type S3Client struct {
	client    *s3.Client
	cdnDomain string
}

// NewS3Client creates an S3-backed Storage.
// If accessKeyID is provided, static credentials are used.
// If empty, the default credential chain is used (ECS task role, env, shared config).
func NewS3Client(ctx context.Context, region, accessKeyID, secretAccessKey, cdnDomain string) (*S3Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if accessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &S3Client{
		client:    s3.NewFromConfig(cfg),
		cdnDomain: cdnDomain,
	}, nil
}

// Download fetches an S3 object by key and writes it to localPath.
func (c *S3Client) Download(ctx context.Context, bucket, key, localPath string) error {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 get %s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", localPath, err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", localPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("write %s: %w", localPath, err)
	}
	return nil
}

// Upload writes a local file to the given S3 bucket and key.
func (c *S3Client) Upload(ctx context.Context, bucket, key, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer f.Close()

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		return fmt.Errorf("s3 put %s/%s: %w", bucket, key, err)
	}
	return nil
}

// ReelURL returns the public CloudFront URL for a reel key.
func (c *S3Client) ReelURL(key string) string {
	return fmt.Sprintf("https://%s/%s", c.cdnDomain, key)
}
