package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/model"
	"wzap/internal/storage"
)

const (
	// idempotencyKeyHeader carries the caller supplied key. It is optional:
	// without it a send is processed independently.
	IdempotencyKeyHeader = "Idempotency-Key"
	// idempotentReplayHeader marks a response replayed from the idempotency
	// store instead of freshly produced.
	IdempotentReplayHeader = "X-Idempotent-Replay"
	// idempotencyTTL is how long a stored response stays replayable.
	IdempotencyTTL = 24 * time.Hour
	// idempotencyKeyMaxLen caps the key size accepted from clients.
	IdempotencyKeyMaxLen = 255
	// fingerprintBodyLimit bounds JSON buffering and rejects oversized requests
	// before the idempotency store is consulted.
	FingerprintBodyLimit = MaxJSONBodyBytes
	// fingerprintMultipartOverhead is the slack above the configured upload
	// limit that still gets an exact multipart fingerprint: the multipart
	// boundaries, part headers and text fields around the file content.
	FingerprintMultipartOverhead = 1 << 20
	// fingerprintMultipartFallback bounds the multipart body the middleware
	// spools when no upload limit was configured. A bigger body falls back to
	// a method+route fingerprint.
	FingerprintMultipartFallback = 64 << 20
)

// Idempotency makes a retried send safe: the first request stores its response
// under the caller's key and a repeat replays it instead of enqueueing the same
// message twice. It scopes every key to the instance of the route and is meant
// to wrap explicitly idempotent POST, PUT and PATCH writes.
//
// Without the header the request goes straight through. While the original
// request runs the key is in flight and a repeat gets 409; a key reused with
// different content gets 422. A 4xx releases the key so the caller can fix the
// payload and retry; a 5xx is stored and replayed because the send may have
// reached WhatsApp before failing, so re-running it could duplicate the
// message. A 503 is the exception: the send path answers it before reaching
// WhatsApp, so the key is released and the retry the body asks for is possible.
// A panic or a handler that writes nothing also releases it.
//
// maxUploadBytes is the largest multipart upload accepted by the routes the
// middleware wraps; it lets a media upload be fingerprinted exactly instead of
// falling back to the route. A non-positive value uses a generous fallback.
func Idempotency(
	repo storage.IdempotencyRepository, log zerolog.Logger, maxUploadBytes int64,
) func(http.Handler) http.Handler {
	multipartLimit := maxUploadBytes + FingerprintMultipartOverhead
	if maxUploadBytes <= 0 {
		multipartLimit = FingerprintMultipartFallback
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			key := strings.TrimSpace(r.Header.Get(IdempotencyKeyHeader))
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			if repo == nil {
				log.Error().
					Str("request_id", RequestIDFromContext(r.Context())).
					Msg("idempotency repository is not configured")
				Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}
			if len(key) > IdempotencyKeyMaxLen {
				Error(w, r, http.StatusBadRequest, "invalid_request", "Idempotency-Key exceeds 255 characters")
				return
			}

			targetID, err := uuid.Parse(PathParam(r, "id"))
			if target, ok := TargetInstance(r); ok {
				targetID = target.ID
				err = nil
			}
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			fingerprint, cleanup, err := FingerprintRequest(r, multipartLimit)
			if cleanup != nil {

				defer cleanup()
			}
			if err != nil {
				WriteJSONBodyError(w, r, err)
				return
			}

			record, acquired, err := repo.Acquire(r.Context(), targetID, key, fingerprint, time.Now().Add(IdempotencyTTL))
			switch {
			case errors.Is(err, storage.ErrInProgress):
				Error(w, r, http.StatusConflict, "conflict", "request with this Idempotency-Key is already in progress")
				return
			case errors.Is(err, storage.ErrFingerprintMismatch):
				Error(w, r, http.StatusUnprocessableEntity, "unprocessable_entity", "Idempotency-Key was already used with different content")
				return
			case err != nil:

				log.Warn().
					Str("request_id", RequestIDFromContext(r.Context())).
					Err(err).
					Msg("idempotency store unavailable, proceeding without it")
				next.ServeHTTP(w, r)
				return
			}

			if !acquired {
				Replay(w, r, record)
				return
			}

			capture := &ResponseCapture{ResponseWriter: w}
			defer func() {

				ctx := context.WithoutCancel(r.Context())
				if !capture.wrote {
					ReleaseKey(ctx, repo, log, targetID, key)
					return
				}

				status := capture.status
				if (status >= http.StatusBadRequest && status < http.StatusInternalServerError) ||
					status == http.StatusServiceUnavailable {
					ReleaseKey(ctx, repo, log, targetID, key)
					return
				}

				if err := repo.Complete(ctx, targetID, key, status, capture.body.Bytes()); err != nil {
					log.Warn().
						Str("request_id", RequestIDFromContext(ctx)).
						Err(err).
						Msg("store idempotent response")
				}
			}()

			next.ServeHTTP(capture, r)
		})
	}
}

// Replay answers with the stored HTTP status and body for a completed
// Idempotency key without transforming the cached bytes.
func Replay(w http.ResponseWriter, r *http.Request, record *model.IdempotencyRecord) {
	if !validReplay(record) {
		Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set(IdempotentReplayHeader, "true")
	w.WriteHeader(record.ResponseStatus)
	_, _ = w.Write(record.ResponseBody)
}

func validReplay(record *model.IdempotencyRecord) bool {
	if record == nil || record.ResponseStatus < http.StatusOK || record.ResponseStatus > 599 {
		return false
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(record.ResponseBody, &envelope); err != nil {
		return false
	}
	dataBody, hasData := envelope["data"]
	errorBody, hasError := envelope["error"]
	if record.ResponseStatus < http.StatusMultipleChoices {
		var data map[string]json.RawMessage
		return hasData && !hasError && json.Unmarshal(dataBody, &data) == nil && data != nil
	}
	if record.ResponseStatus < http.StatusBadRequest || hasData || !hasError {
		return false
	}
	var body ErrorBody
	return json.Unmarshal(errorBody, &body) == nil && body.Code != "" && body.Message != ""
}

// ReleaseKey frees an idempotency key, logging a failure to free it.
func ReleaseKey(ctx context.Context, repo storage.IdempotencyRepository, log zerolog.Logger, instanceID uuid.UUID, key string) {
	if err := repo.Release(ctx, instanceID, key); err != nil {
		log.Warn().
			Str("request_id", RequestIDFromContext(ctx)).
			Err(err).
			Msg("release idempotency key")
	}
}

// ResponseCapture records the status and body written by the wrapped handler so
// the middleware can store them under the idempotency key.
type ResponseCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
	wrote  bool
}

// WriteHeader records the first status written and forwards it.
func (c *ResponseCapture) WriteHeader(status int) {
	if c.wrote {
		return
	}
	c.wrote = true
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

// Write records the body bytes and forwards them, defaulting to 200.
func (c *ResponseCapture) Write(b []byte) (int, error) {
	if !c.wrote {
		c.wrote = true
		c.status = http.StatusOK
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (c *ResponseCapture) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// FingerprintRequest returns a stable hash of a send: method, route and body,
// plus a cleanup function the caller must run once the handler is done with the
// request body (nil when nothing has to be released).
//
// A multipart body is parsed as it streams: its text fields and each file's
// name, content type and content hash take part in the fingerprint, so a
// same-key retry is only a replay when the upload is really the same. The body
// is spooled to a temporary file while it is parsed and handed back to the
// handler, so file content is never held in memory. A body above the
// configured upload limit falls back to a method+route fingerprint, which is
// harmless because the handler rejects it before anything is enqueued.
//
// A non-multipart body larger than fingerprintBodyLimit is rejected before
// acquiring or replaying an idempotency key.
func FingerprintRequest(r *http.Request, multipartLimit int64) (string, func(), error) {
	contentType := r.Header.Get("Content-Type")
	if mediaType, params, err := mime.ParseMediaType(contentType); err == nil && strings.HasPrefix(mediaType, "multipart/") {
		return FingerprintMultipart(r, params["boundary"], multipartLimit)
	}

	body, complete, err := ReadBodyPrefix(r)
	if err != nil {
		return "", nil, err
	}
	if !complete {
		return "", nil, &http.MaxBytesError{Limit: FingerprintBodyLimit}
	}

	sum := sha256.New()
	WriteRoute(sum, r)
	sum.Write(body)
	return hex.EncodeToString(sum.Sum(nil)), nil, nil
}

// FingerprintMultipart spools the multipart body of r while hashing its parts,
// then restores it for the handler. The cleanup closes and removes the spool.
func FingerprintMultipart(r *http.Request, boundary string, limit int64) (string, func(), error) {
	if boundary == "" {
		return "", nil, errors.New("multipart body without a boundary")
	}

	spool, err := os.CreateTemp("", ".wzap-fingerprint-*")
	if err != nil {

		return RouteFingerprint(r), nil, nil
	}
	cleanup := func() {
		_ = spool.Close()
		_ = os.Remove(spool.Name())
	}

	written, err := io.Copy(spool, io.LimitReader(r.Body, limit+1))
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if written > limit {

		if _, err := spool.Seek(0, io.SeekStart); err != nil {
			cleanup()
			return "", nil, err
		}
		r.Body = io.NopCloser(io.MultiReader(spool, r.Body))
		return RouteFingerprint(r), cleanup, nil
	}

	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return "", nil, err
	}
	parts, err := MultipartParts(boundary, spool)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return "", nil, err
	}
	r.Body = spool
	return PartsFingerprint(r, parts), cleanup, nil
}

// RouteFingerprint hashes the method and route of r.
func RouteFingerprint(r *http.Request) string {
	sum := sha256.New()
	WriteRoute(sum, r)
	return hex.EncodeToString(sum.Sum(nil))
}

// PartsFingerprint hashes the method, route and canonical multipart parts. Each
// part is length-prefixed, so a value containing the part separator cannot be
// re-segmented into a different set of parts.
func PartsFingerprint(r *http.Request, parts []string) string {
	sum := sha256.New()
	WriteRoute(sum, r)
	for _, part := range parts {
		_, _ = fmt.Fprintf(sum, "%d:", len(part))
		sum.Write([]byte(part))
		sum.Write([]byte{'\n'})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// WriteRoute writes the method and route pattern of r into h. Instance aliases
// share the pattern; concrete resource parameters still identify their target.
func WriteRoute(h io.Writer, r *http.Request) {
	route := r.Pattern
	if route == "" {
		route = r.URL.Path
	}
	_, _ = fmt.Fprintf(h, "%s\n%s\n", r.Method, route)
	_, instanceSuffix, _ := strings.Cut(r.Pattern, "/instances/{")
	instanceParameter, _, _ := strings.Cut(instanceSuffix, "}")
	remaining := r.Pattern
	for {
		_, after, found := strings.Cut(remaining, "{")
		if !found {
			return
		}
		name, rest, found := strings.Cut(after, "}")
		if !found {
			return
		}
		remaining = rest
		if name == instanceParameter {
			continue
		}
		value := PathParam(r, name)
		_, _ = fmt.Fprintf(h, "%d:%s%d:%s\n", len(name), name, len(value), value)
	}
}

// ReadBodyPrefix reads at most fingerprintBodyLimit+1 bytes of the request body
// and restores the body for the handler. The second result reports whether the
// whole body fit in the limit.
func ReadBodyPrefix(r *http.Request) ([]byte, bool, error) {
	if r.Body == nil {
		return nil, true, nil
	}

	prefix, err := io.ReadAll(io.LimitReader(r.Body, FingerprintBodyLimit+1))
	if err != nil {
		return nil, false, err
	}
	if len(prefix) > FingerprintBodyLimit {
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), r.Body))
		return prefix, false, nil
	}
	r.Body = io.NopCloser(bytes.NewReader(prefix))
	return prefix, true, nil
}

// MultipartParts sorts distinct multipart fields by kind and name, preserving
// the order of repeated values/files because handlers consume the first one.
// A text field is "field\x00name\x00value"; a file part is
// "file\x00name\x00filename\x00content-type\x00sha256".
func MultipartParts(boundary string, body io.Reader) ([]string, error) {
	type canonicalPart struct{ key, value string }
	reader := multipart.NewReader(body, boundary)
	parts := []canonicalPart{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		if part.FileName() != "" {
			sum := sha256.New()
			if _, err := io.Copy(sum, part); err != nil {
				return nil, err
			}
			key := "file\x00" + part.FormName()
			parts = append(parts, canonicalPart{key: key, value: key + "\x00" + part.FileName() +
				"\x00" + part.Header.Get("Content-Type") + "\x00" + hex.EncodeToString(sum.Sum(nil))})
			continue
		}

		value, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		key := "field\x00" + part.FormName()
		parts = append(parts, canonicalPart{key: key, value: key + "\x00" + string(value)})
	}
	slices.SortStableFunc(parts, func(a, b canonicalPart) int { return strings.Compare(a.key, b.key) })
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = part.value
	}
	return result, nil
}
