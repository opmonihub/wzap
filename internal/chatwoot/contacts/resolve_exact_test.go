package contacts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"wzap/internal/chatwoot/client"
	"wzap/internal/model"
)

func TestResolveContainsDoesNotMergeUnrelatedPhone(t *testing.T) {
	valid := client.Contact{ID: 7, PhoneNumber: "+551188888888"}
	unrelated := client.Contact{ID: 20, PhoneNumber: "+49551188888888"}
	merges := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/contacts/filter"):
			var body struct {
				Payload []struct {
					Values []string `json:"values"`
				} `json:"payload"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			if len(body.Payload) != 1 || len(body.Payload[0].Values) != 1 {
				t.Error("bad query")
				return
			}
			needle := body.Payload[0].Values[0]
			found := []client.Contact{}
			for _, c := range []client.Contact{valid, unrelated} {
				if strings.Contains(c.PhoneNumber, needle) {
					found = append(found, c)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"payload": found})
		case strings.HasSuffix(r.URL.Path, "/actions/contact_merge"):
			merges++
			_ = json.NewEncoder(w).Encode(unrelated)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	resolver := New(newTestClient(srv.URL), model.ChatwootConfig{MergeBrazilContacts: true}, zerolog.Nop())
	got, err := resolver.Resolve(context.Background(), valid.PhoneNumber, false, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != valid.ID || merges != 0 {
		t.Fatal("substring match selected unrelated phone and merged the exact contact into it")
	}
}

func TestResolveIdentifierRequiresExactMatch(t *testing.T) {
	for _, isGroup := range []bool{false, true} {
		name, jid := "individual-conflict", "5511999887766@s.whatsapp.net"
		if isGroup {
			name, jid = "group", "120363000000000000@g.us"
		}
		for _, exact := range []bool{false, true} {
			t.Run(name+map[bool]string{false: "/contains-only", true: "/exact-after-contains"}[exact], func(t *testing.T) {
				var chosenIDs []string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case strings.HasSuffix(r.URL.Path, "/contacts/filter"):
						_, _ = w.Write([]byte(`{"payload":[]}`))
					case strings.HasSuffix(r.URL.Path, "/contacts/search"):
						matches := []client.Contact{{ID: 20, Identifier: "prefix-" + jid, Name: "Unrelated"}}
						if exact {
							matches = append(matches, client.Contact{ID: 7, Identifier: jid, Name: "Exact"})
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"payload": matches})
					case strings.HasSuffix(r.URL.Path, "/contacts") && r.Method == http.MethodPost:
						if !isGroup {
							w.WriteHeader(http.StatusUnprocessableEntity)
							_, _ = w.Write([]byte(`{"message":"phone taken"}`))
							return
						}
						_ = json.NewEncoder(w).Encode(client.Contact{ID: 9, Identifier: jid, Name: "New (GROUP)"})
					case r.Method == http.MethodPut:
						chosenIDs = append(chosenIDs, r.URL.Path)
						_ = json.NewEncoder(w).Encode(client.Contact{ID: 7, Identifier: jid, Name: "Fresh"})
					default:
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer srv.Close()
				got, err := New(newTestClient(srv.URL), model.ChatwootConfig{}, zerolog.Nop()).Resolve(context.Background(), "+5511999887766", isGroup, "Fresh", "", jid)
				if !exact && !isGroup {
					if err == nil || got != nil {
						t.Fatalf("contains-only conflict = %+v, %v, want unresolved", got, err)
					}
				} else {
					want := int64(7)
					if !exact {
						want = 9
					}
					if err != nil || got == nil || got.ID != want {
						t.Fatalf("identifier result = %+v, %v; want id %d", got, err, want)
					}
				}
				for _, path := range chosenIDs {
					if strings.HasSuffix(path, "/20") {
						t.Error("cosmetic update reached unrelated identifier")
					}
				}
			})
		}
	}
}
