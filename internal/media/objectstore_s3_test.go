package media

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Opt-in integration test against a real S3-compatible server (MinIO). Set
// WZAP_TEST_S3_ENDPOINT to run it; credentials default to the compose dev
// values and WZAP_TEST_S3_ACCESS_KEY/WZAP_TEST_S3_SECRET_KEY/
// WZAP_TEST_S3_BUCKET override them. The test uses a dedicated bucket plus a
// unique object key, so runs are isolated and re-runnable.
func TestObjectStoreS3Integration(t *testing.T) {
	endpoint := os.Getenv("WZAP_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set WZAP_TEST_S3_ENDPOINT to run the MinIO integration test")
	}
	accessKey := os.Getenv("WZAP_TEST_S3_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "wzap"
	}
	secretKey := os.Getenv("WZAP_TEST_S3_SECRET_KEY")
	if secretKey == "" {
		secretKey = "wzap-secret-key"
	}
	bucket := os.Getenv("WZAP_TEST_S3_BUCKET")
	if bucket == "" {
		bucket = "wzap-media-test"
	}

	store := NewObjectStore(S3Config{
		Endpoint:  endpoint,
		Bucket:    bucket,
		Region:    "us-east-1",
		AccessKey: accessKey,
		SecretKey: secretKey,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatalf("EnsureBucket: %v", err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatalf("EnsureBucket on existing bucket: %v", err)
	}

	key := "test/" + uuid.NewString() + ".bin"
	payload := []byte("wzap integration payload \x00\x01\x02")
	if err := store.Put(ctx, key, payload, "application/octet-stream"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := store.Delete(cleanupCtx, "", key); err != nil {
			t.Errorf("cleanup Delete: %v", err)
		}
	})

	exists, err := store.Exists(ctx, "", key)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatalf("Exists = false after Put")
	}

	body, err := store.Get(ctx, "", key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("body = %q, want %q", got, payload)
	}

	if err := store.Delete(ctx, "", key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	exists, err = store.Exists(ctx, "", key)
	if err != nil {
		t.Fatalf("Exists after Delete: %v", err)
	}
	if exists {
		t.Fatalf("Exists = true after Delete")
	}
	if _, err := store.Get(ctx, "", key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete err = %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, "", key); err != nil {
		t.Fatalf("Delete absent key: %v", err)
	}
	if _, err := store.Get(ctx, "", "test/"+uuid.NewString()+".bin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing key err = %v, want ErrNotFound", err)
	}
}
