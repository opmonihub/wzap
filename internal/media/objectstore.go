package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Objects is the object-store contract the media storage depends on. The
// concrete implementation talks S3/MinIO; tests substitute an in-memory
// fake. All errors wrap ErrNotFound for absent keys.
type Objects interface {
	// Bucket returns the configured bucket name stored on every media row.
	Bucket() string
	// EnsureBucket creates the bucket when absent; an existing bucket is a
	// success and any other failure must abort the boot.
	EnsureBucket(ctx context.Context) error
	// Put uploads data under objectKey, overwriting any previous object.
	Put(ctx context.Context, objectKey string, data []byte, mimeType string) error
	// Get streams the object; a missing key wraps ErrNotFound.
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
	// Delete removes the object; deleting an absent key is a success.
	Delete(ctx context.Context, objectKey string) error
	// Exists reports whether objectKey is present in the bucket.
	Exists(ctx context.Context, objectKey string) (bool, error)
}

// ObjectStore keeps media bytes in an S3-compatible object store (MinIO). The
// SQL row stays the metadata authority; bucket and object_key are stable
// identifiers, never presigned URLs — the public download keeps flowing
// through the authenticated HTTP route.
type ObjectStore struct {
	client *s3.Client
	bucket string
}

// S3Config carries the WZAP_S3_* settings the store needs.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	UseTLS    bool
}

var _ Objects = (*ObjectStore)(nil)

// NewObjectStore returns an ObjectStore pointed at cfg. The bucket is not
// created here: the bootstrap calls EnsureBucket so a misconfigured store
// fails the service at boot instead of on the first write.
func NewObjectStore(cfg S3Config) *ObjectStore {
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if cfg.UseTLS {
			scheme = "https"
		}
		endpoint = scheme + "://" + endpoint
	}
	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.Credentials = credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")
		},
	}
	return &ObjectStore{
		client: s3.New(s3.Options{Region: cfg.Region}, opts...),
		bucket: cfg.Bucket,
	}
}

// Bucket returns the configured bucket name (stored on every media row).
func (s *ObjectStore) Bucket() string {
	return s.bucket
}

// EnsureBucket creates the configured bucket when absent and returns when it
// already exists (BucketAlreadyOwnedByYou/409 conflict = success). Any other
// failure means the store cannot persist media and must abort the boot.
func (s *ObjectStore) EnsureBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if err == nil {
		return nil
	}
	var bne *s3types.BucketAlreadyExists
	var baoby *s3types.BucketAlreadyOwnedByYou
	var apiErr smithy.APIError
	if errors.As(err, &bne) || errors.As(err, &baoby) ||
		(errors.As(err, &apiErr) && apiErr.ErrorCode() == "BucketAlreadyOwnedByYou") {
		return nil
	}
	return fmt.Errorf("ensure media bucket %q: %w", s.bucket, err)
}

// Put uploads data under objectKey. A re-put overwrites the object, which is
// safe because object keys embed a fresh media UUID.
func (s *ObjectStore) Put(ctx context.Context, objectKey string, data []byte, mimeType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(data),
	}
	if mimeType != "" {
		input.ContentType = aws.String(mimeType)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put media object %q: %w", objectKey, err)
	}
	return nil
}

// Get streams the object. A missing key reports ErrNotFound so callers map it
// the same way a missing file did.
func (s *ObjectStore) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var nsk *s3types.NoSuchKey
		var apiErr smithy.APIError
		if errors.As(err, &nsk) ||
			(errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound")) {
			return nil, fmt.Errorf("get media object %q: %w", objectKey, ErrNotFound)
		}
		return nil, fmt.Errorf("get media object %q: %w", objectKey, err)
	}
	return out.Body, nil
}

// Delete removes the object. S3 delete is idempotent: a missing key is a
// success, matching the "already gone file is not an error" policy.
func (s *ObjectStore) Delete(ctx context.Context, objectKey string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	}); err != nil {
		return fmt.Errorf("delete media object %q: %w", objectKey, err)
	}
	return nil
}

// Exists reports whether objectKey is present in the bucket.
func (s *ObjectStore) Exists(ctx context.Context, objectKey string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var nf *s3types.NotFound
		var apiErr smithy.APIError
		if errors.As(err, &nf) ||
			(errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey")) {
			return false, nil
		}
		return false, fmt.Errorf("head media object %q: %w", objectKey, err)
	}
	return true, nil
}
