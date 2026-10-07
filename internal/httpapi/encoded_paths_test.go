package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"wzap/internal/config"
	"wzap/internal/httpapi"
	"wzap/internal/instance"
)

func TestFinalEncodedChannel(t *testing.T) {
	svc := existingInstanceService()
	seen := ""
	svc.getNewsletterFn = func(_ context.Context, _ uuid.UUID, jid string) (instance.Newsletter, error) {
		seen = jid
		return instance.Newsletter{ChannelJID: jid}, nil
	}
	srv := httpapi.New(config.Config{APIKey: testToken}, zerolog.Nop(), httpapi.Deps{Instances: svc})
	rec := serveJSON(t, srv, http.MethodGet, "/instances/"+uuid.NewString()+"/newsletters/123%40newsletter", "")
	if rec.Code != 200 || seen != "123@newsletter" {
		t.Fatalf("status=%d jid=%q body=%s", rec.Code, seen, rec.Body.String())
	}
}
