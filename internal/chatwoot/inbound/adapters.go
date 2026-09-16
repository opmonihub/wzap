package inbound

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"wzap/internal/chatwoot/client"
)

// ChatwootAPI is the Chatwoot surface the inbound needs for private notes
// and operational confirmations.
type ChatwootAPI interface {
	CreateMessage(ctx context.Context, conversationID int64, req client.CreateMessageRequest) (*client.Message, error)
}

// APIChats adapts a Chatwoot client to Chats: notes and confirmations go as
// outgoing messages, private exactly for failure notes.
type APIChats struct {
	api ChatwootAPI
}

// NewAPIChats builds Chats over api.
func NewAPIChats(api ChatwootAPI) *APIChats {
	return &APIChats{api: api}
}

// CreateMessage posts content to conversationID, private for failure notes.
func (a *APIChats) CreateMessage(ctx context.Context, conversationID int64, content string, private bool) (int64, error) {
	if a.api == nil {
		return 0, fmt.Errorf("chatwoot api unavailable")
	}
	msg, err := a.api.CreateMessage(ctx, conversationID, client.CreateMessageRequest{
		Content:     content,
		MessageType: client.MessageTypeOutgoing,
		Private:     private,
	})
	if err != nil {
		return 0, err
	}
	if msg == nil {
		return 0, nil
	}
	return msg.ID, nil
}

// downloadTimeout pins the attachment fetch deadline.
const downloadTimeout = 15 * time.Second

// HTTPDownloader fetches attachment bytes from data_url with a size cap.
// Transports are cached per Chatwoot host so connections pool while every
// dial still goes through the pinned SSRF resolver.
type HTTPDownloader struct {
	maxBytes int64

	mu         sync.Mutex
	transports map[string]*http.Transport
}

// NewHTTPDownloader builds a Downloader with a 15s timeout and maxBytes cap.
// A non-positive maxBytes defers the cap to the media store.
func NewHTTPDownloader(maxBytes int64) *HTTPDownloader {
	return &HTTPDownloader{maxBytes: maxBytes}
}

// transportFor returns the cached transport pinning dials to the SSRF policy
// for allowHost (the instance Chatwoot url), building it once per host.
func (d *HTTPDownloader) transportFor(allowHost string) *http.Transport {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.transports == nil {
		d.transports = make(map[string]*http.Transport)
	}
	if t, ok := d.transports[allowHost]; ok {
		return t
	}
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           pinnedAttachmentDialContext(allowHost),
		ResponseHeaderTimeout: downloadTimeout,
	}
	d.transports[allowHost] = t
	return t
}

// Download GETs url and returns its bytes plus the response content type.
// The SSRF policy (ssrf.go) gates the initial URL and every redirect hop
// against allowHost (the instance Chatwoot url); at most
// maxAttachmentRedirects hops are followed. The transport dial re-resolves
// and re-validates through the same policy on the pinned address, so a DNS
// rebinding between validation and connect is refused at dial time.
func (d *HTTPDownloader) Download(ctx context.Context, url, allowHost string) ([]byte, string, error) {
	if err := validateAttachmentURL(url, allowHost); err != nil {
		return nil, "", err
	}
	client := &http.Client{
		Timeout:   downloadTimeout,
		Transport: d.transportFor(allowHost),
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxAttachmentRedirects {
			return fmt.Errorf("download %s: stopped after %d redirects", url, maxAttachmentRedirects)
		}
		if err := validateAttachmentURL(req.URL.String(), allowHost); err != nil {
			return err
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	limit := d.maxBytes + 1
	if limit <= 1 {
		limit = 1 << 26
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", err
	}
	mime := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	return data, mime, nil
}
