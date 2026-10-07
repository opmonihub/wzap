package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type swaggerSchema struct {
	Required   []string                 `json:"required"`
	Ref        string                   `json:"$ref"`
	Type       string                   `json:"type"`
	Properties map[string]swaggerSchema `json:"properties"`
	Items      *swaggerSchema           `json:"items"`
	AllOf      []swaggerSchema          `json:"allOf"`
}
type swaggerOperation struct {
	Parameters []struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		In          string         `json:"in"`
		Required    bool           `json:"required"`
		Schema      *swaggerSchema `json:"schema"`
	} `json:"parameters"`
	Responses map[string]struct {
		Schema  *swaggerSchema `json:"schema"`
		Headers map[string]struct {
			Type string `json:"type"`
		} `json:"headers"`
	} `json:"responses"`
	Security []map[string][]string `json:"security"`
	Produces []string              `json:"produces"`
	Consumes []string              `json:"consumes"`
}
type swaggerDocument struct {
	Paths               map[string]map[string]swaggerOperation `json:"paths"`
	Definitions         map[string]swaggerSchema               `json:"definitions"`
	SecurityDefinitions map[string]struct {
		Type string `json:"type"`
		In   string `json:"in"`
		Name string `json:"name"`
	} `json:"securityDefinitions"`
}

func servedSwagger(t *testing.T) swaggerDocument {
	t.Helper()
	rec := serve(t, newTestServer(t), http.MethodGet, "/swagger/doc.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("Swagger status = %d", rec.Code)
	}
	var doc swaggerDocument
	decodeJSON(t, rec.Body.Bytes(), &doc)
	return doc
}

// Missing an annotation for either method on a shared path must fail coverage.
func TestSwaggerDocumentsEveryRegisteredOperation(t *testing.T) {
	doc := servedSwagger(t)
	routes, ok := newTestServer(t).Handler.(chi.Routes)
	if !ok {
		t.Fatal("server must expose Chi routes")
	}
	count := 0
	err := chi.Walk(routes, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(path, "*") {
			return nil
		}
		count++
		op, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			t.Errorf("Swagger missing %s %s", method, path)
			return nil
		}
		for _, segment := range strings.Split(path, "/") {
			if !strings.HasPrefix(segment, "{") {
				continue
			}
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			found := false
			for _, param := range op.Parameters {
				if param.In == "path" && param.Name == name && param.Required {
					found = true
				}
			}
			if !found {
				t.Errorf("%s %s missing path param %s", method, path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 88 {
		t.Fatalf("operations=%d want88 explicit operations plus Manager static surface", count)
	}
}
func resolveSwaggerSchema(t *testing.T, doc swaggerDocument, schema swaggerSchema) swaggerSchema {
	t.Helper()
	if schema.Ref != "" {
		name, ok := strings.CutPrefix(schema.Ref, "#/definitions/")
		definition, found := doc.Definitions[name]
		if !ok || !found {
			t.Fatalf("unresolved Swagger schema %q", schema.Ref)
		}
		return resolveSwaggerSchema(t, doc, definition)
	}
	if len(schema.AllOf) > 0 {
		properties := make(map[string]swaggerSchema)
		for _, part := range schema.AllOf {
			resolved := resolveSwaggerSchema(t, doc, part)
			schema.Required = append(schema.Required, resolved.Required...)
			for name, property := range resolved.Properties {
				properties[name] = property
			}
		}
		schema.Properties = properties
	}
	return schema
}

// Removing data/error composition must fail even when descriptions mention envelopes.
func TestSwaggerResponseEnvelopes(t *testing.T) {
	doc := servedSwagger(t)
	for path, methods := range doc.Paths {
		for method, op := range methods {
			for status, response := range op.Responses {
				t.Run(method+" "+path+" "+status, func(t *testing.T) {
					if strings.HasPrefix(path, "/manager") {
						return
					}
					if status == "204" {
						if response.Schema != nil {
							t.Error("204 response must not declare a body")
						}
						return
					}
					if path == "/media/{id}" && status == "200" {
						if response.Schema == nil || response.Schema.Type != "file" {
							t.Error("media success must be binary")
						}
						return
					}
					if path == "/chatwoot/webhook/{id}" && status == "200" {
						return
					}
					if response.Schema == nil {
						t.Fatal("missing response schema")
					}
					key := "error"
					if strings.HasPrefix(status, "2") || path == "/readyz" {
						key = "data"
					}
					schema := resolveSwaggerSchema(t, doc, *response.Schema)
					property, ok := schema.Properties[key]
					if !ok {
						t.Fatalf("missing %s envelope", key)
					}
					if key == "error" {
						body := resolveSwaggerSchema(t, doc, property)
						for _, field := range []string{"code", "message"} {
							if body.Properties[field].Type != "string" {
								t.Errorf("error.%s must be a string", field)
							}
						}
					}
				})
			}
		}
	}
}

// Independently chosen payloads guard against replacing every data schema with any.
func TestSwaggerTypedPayloadsAndExceptions(t *testing.T) {
	doc := servedSwagger(t)
	for _, test := range []struct {
		path, method, status, via, field, fieldType string
	}{
		{"/healthz", "get", "200", "", "status", "string"},
		{"/readyz", "get", "503", "", "checks", "object"},
		{"/instances/{id}", "get", "200", "instance", "id", "string"},
		{"/instances/{id}", "get", "200", "instance", "integration", "object"},
		{"/instances/{id}", "get", "200", "instance", "settings", "object"},
		{"/instances/{instance}/messages/text", "post", "202", "message", "id", "string"},
		{"/instances/{id}/chatwoot/import", "post", "202", "", "imported", "integer"},
		{"/instances/{id}/chatwoot/command", "post", "200", "", "ok", "boolean"},
		{"/instances/{id}/chatwoot", "get", "200", "chatwoot_config", "webhook_url", "string"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := doc.Paths[test.path][test.method].Responses[test.status]
			if response.Schema == nil {
				t.Fatal("missing response schema")
			}
			envelope := resolveSwaggerSchema(t, doc, *response.Schema)
			payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
			if test.via != "" {
				payload = resolveSwaggerSchema(t, doc, payload.Properties[test.via])
			}

			fieldSchema := resolveSwaggerSchema(t, doc, payload.Properties[test.field])
			if fieldSchema.Type != test.fieldType {
				t.Errorf("data.%s.%s type = %q, want %q", test.via, test.field, fieldSchema.Type, test.fieldType)
			}
		})
	}

	t.Run("chatwoot token hidden", func(t *testing.T) {
		response := doc.Paths["/instances/{id}/chatwoot"]["get"].Responses["200"]
		if response.Schema == nil {
			t.Fatal("missing response schema")
		}
		envelope := resolveSwaggerSchema(t, doc, *response.Schema)
		payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
		config := resolveSwaggerSchema(t, doc, payload.Properties["chatwoot_config"])
		if _, ok := config.Properties["token"]; ok {
			t.Error("GET chatwoot must not expose the token")
		}
	})

	t.Run("instance aggregated blocks", func(t *testing.T) {
		response := doc.Paths["/instances/{id}"]["get"].Responses["200"]
		if response.Schema == nil {
			t.Fatal("missing response schema")
		}
		envelope := resolveSwaggerSchema(t, doc, *response.Schema)
		data := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
		object := resolveSwaggerSchema(t, doc, data.Properties["instance"])
		if _, ok := object.Properties["webhook"]; ok {
			t.Error("webhook must move under integration (BREAKING), not the instance root")
		}
		integration := resolveSwaggerSchema(t, doc, object.Properties["integration"])
		if _, ok := integration.Properties["webhook"]; !ok {
			t.Error("integration.webhook must be documented")
		}
		nested := resolveSwaggerSchema(t, doc, integration.Properties["chatwoot_config"])
		if _, ok := nested.Properties["token"]; ok {
			t.Error("nested chatwoot_config must not expose the token")
		}
		settings := resolveSwaggerSchema(t, doc, object.Properties["settings"])
		if settings.Properties["default_disappearing"].Type != "string" {
			t.Errorf("settings.default_disappearing type = %q, want string", settings.Properties["default_disappearing"].Type)
		}
		for _, field := range []string{"profile", "privacy", "status_privacy"} {
			block := resolveSwaggerSchema(t, doc, settings.Properties[field])
			if len(block.Properties) == 0 {
				t.Errorf("settings.%s must be a documented object", field)
			}
		}
	})
	for _, test := range []struct{ path, entity, itemField string }{
		{"/users", "user", "email"},
		{"/instances", "instance", "id"},
		{"/instances/{instance}/messages", "message", "id"},
		{"/instances/{id}/groups", "group", "jid"},
	} {
		t.Run("collection "+test.path, func(t *testing.T) {
			response := doc.Paths[test.path]["get"].Responses["200"]
			if response.Schema == nil {
				t.Fatal("missing response schema")
			}
			envelope := resolveSwaggerSchema(t, doc, *response.Schema)
			payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
			collection := resolveSwaggerSchema(t, doc, payload.Properties[map[string]string{"instance": "instances", "group": "groups", "user": "users", "message": "messages", "channel": "channels"}[test.entity]])
			if collection.Type != "array" || collection.Items == nil {
				t.Fatal("missing typed collection")
			}
			wrapper := resolveSwaggerSchema(t, doc, *collection.Items)
			item := wrapper
			if item.Properties[test.itemField].Type != "string" {
				t.Errorf("collection item missing string %s.%s", test.entity, test.itemField)
			}
		})
	}
	t.Run("public webhook", func(t *testing.T) {
		op := doc.Paths["/chatwoot/webhook/{id}"]["post"]
		response := op.Responses["200"]
		if response.Schema == nil {
			t.Fatal("missing webhook schema")
		}
		schema := resolveSwaggerSchema(t, doc, *response.Schema)
		if schema.Properties["content"].Type != "string" || len(schema.Properties) != 1 {
			t.Error("webhook must return only raw content string")
		}
	})
	t.Run("manager HTML", func(t *testing.T) {
		op := doc.Paths["/manager/"]["get"]
		if response := op.Responses["200"]; response.Schema == nil || response.Schema.Type != "string" {
			t.Error("manager 200 must be HTML text")
		}
		found := false
		for _, contentType := range op.Produces {
			if contentType == "text/html" {
				found = true
			}
		}
		if !found {
			t.Error("manager entry must produce text/html")
		}
	})
	t.Run("manager redirect", func(t *testing.T) {
		op := doc.Paths["/manager"]["get"]
		if op.Responses["301"].Headers["Location"].Type != "string" {
			t.Error("missing 301 Location header")
		}
		if response := op.Responses["503"]; response.Schema == nil || response.Schema.Type != "string" {
			t.Error("manager 503 must be plain text")
		}
	})
}

// Public operations must stay usable without an API key; cookies suffice for dual auth.
func TestSwaggerCredentialAndRequestContracts(t *testing.T) {
	doc := servedSwagger(t)
	credential := doc.SecurityDefinitions["apikey"]
	if credential.Type != "apiKey" || credential.In != "header" || credential.Name != "apikey" {
		t.Error("Authorize must use the apikey header security definition")
	}
	for path, methods := range doc.Paths {
		for method, op := range methods {
			public := path == "/healthz" || path == "/readyz" || strings.HasPrefix(path, "/manager") || strings.HasPrefix(path, "/auth/") || path == "/chatwoot/webhook/{id}"
			if public && len(op.Security) != 0 {
				t.Errorf("%s %s must not require apikey security", method, path)
			}
			if !public {
				found := false
				for _, requirement := range op.Security {
					if _, ok := requirement["apikey"]; ok {
						found = true
					}
				}
				if !found {
					t.Errorf("%s %s missing machine apikey security", method, path)
				}
			}
			for _, parameter := range op.Parameters {
				if parameter.In == "header" && (strings.EqualFold(parameter.Name, "apikey") || strings.EqualFold(parameter.Name, "X-Request-Id")) {
					t.Errorf("%s %s must not expose manual %s header input", method, path, parameter.Name)
				}
			}
			for status, response := range op.Responses {
				if response.Headers["X-Request-Id"].Type != "string" {
					t.Errorf("%s %s %s missing response X-Request-Id header", method, path, status)
				}
			}
		}
	}
	for _, path := range []string{"/instances/{id}/chatwoot/import", "/instances/{id}/connect", "/instances/{id}/disconnect"} {
		if len(doc.Paths[path]["post"].Consumes) != 0 {
			t.Errorf("body-less POST %s must not require a content type", path)
		}
	}
	op := doc.Paths["/auth/login"]["post"]
	for _, status := range []string{"413", "429"} {
		if _, ok := op.Responses[status]; !ok {
			t.Errorf("login missing %s response", status)
		}
	}
	for _, path := range []string{"/auth/login", "/auth/logout"} {
		if doc.Paths[path]["post"].Responses["200"].Headers["Set-Cookie"].Type != "string" {
			t.Errorf("%s missing Set-Cookie header", path)
		}
	}
}

// Instance listing exposes the complete collection without pagination controls.
func TestSwaggerInstanceListingWithoutPagination(t *testing.T) {
	doc := servedSwagger(t)
	op, ok := doc.Paths["/instances"]["get"]
	if !ok {
		t.Fatal("missing GET /instances operation")
	}
	for _, parameter := range op.Parameters {
		if parameter.In == "query" && (parameter.Name == "limit" || parameter.Name == "cursor") {
			t.Errorf("GET /instances must not expose pagination input %s", parameter.Name)
		}
	}
	response := op.Responses["200"]
	if response.Schema == nil {
		t.Fatal("missing instance list response schema")
	}
	envelope := resolveSwaggerSchema(t, doc, *response.Schema)
	payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
	if _, ok := payload.Properties["next_cursor"]; ok {
		t.Error("GET /instances data must not include next_cursor")
	}
	items := resolveSwaggerSchema(t, doc, payload.Properties["instances"])
	if items.Type != "array" || items.Items == nil {
		t.Fatal("GET /instances data.instances must be a typed array")
	}
	wrapper := resolveSwaggerSchema(t, doc, *items.Items)
	item := wrapper
	if item.Properties["id"].Type != "string" {
		t.Error("GET /instances data.instances must retain the instance schema")
	}
}

// Removing instance pagination must preserve the other collections' page contracts.
func TestSwaggerOtherCollectionPagination(t *testing.T) {
	doc := servedSwagger(t)
	for _, path := range []string{"/instances/{instance}/messages", "/instances/{id}/groups", "/instances/{id}/newsletters"} {
		t.Run(path, func(t *testing.T) {
			op := doc.Paths[path]["get"]
			for _, name := range []string{"limit", "cursor"} {
				found := false
				for _, parameter := range op.Parameters {
					if parameter.In == "query" && parameter.Name == name {
						found = true
					}
				}
				if !found {
					t.Errorf("missing pagination query %s", name)
				}
			}
			response := op.Responses["200"]
			if response.Schema == nil {
				t.Fatal("missing collection response schema")
			}
			envelope := resolveSwaggerSchema(t, doc, *response.Schema)
			payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
			if payload.Properties["next_cursor"].Type != "string" {
				t.Error("data.next_cursor must remain a string")
			}
		})
	}
}

// Quotas are JSON integers even though their decoders preserve raw validation input.
func TestSwaggerQuotaRequestSchemas(t *testing.T) {
	doc := servedSwagger(t)
	for _, test := range []struct{ path, method string }{{"/users", "post"}, {"/users/{id}", "patch"}} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			for _, parameter := range doc.Paths[test.path][test.method].Parameters {
				if parameter.In != "body" {
					continue
				}
				if parameter.Schema == nil {
					t.Fatal("missing body schema")
				}
				schema := resolveSwaggerSchema(t, doc, *parameter.Schema)
				if schema.Properties["instance_limit"].Type != "integer" {
					t.Error("instance_limit must be documented as an integer")
				}
				return
			}
			t.Fatal("missing body parameter")
		})
	}
}
func TestSwaggerInstanceReferenceContract(t *testing.T) {
	doc := servedSwagger(t)
	for path, methods := range doc.Paths {
		if !strings.Contains(path, "/instances/{id}") && path != "/chatwoot/webhook/{id}" {
			continue
		}
		for method, operation := range methods {
			found := false
			for _, param := range operation.Parameters {
				if param.Name == "id" && param.In == "path" {
					found = true
					if !strings.Contains(param.Description, "UUID or name") {
						t.Errorf("%s %s reference description=%q", method, path, param.Description)
					}
				}
			}
			if !found {
				t.Errorf("%s %s lacks instance reference", method, path)
			}
			if _, ok := operation.Responses["409"]; !ok {
				t.Errorf("%s %s lacks ambiguous-name response", method, path)
			}
		}
	}
	stats := doc.Paths["/instances/stats"]["get"]
	found := false
	for _, param := range stats.Parameters {
		if param.Name == "instance" && param.In == "query" && !param.Required && strings.Contains(param.Description, "UUID or name") {
			found = true
		}
	}
	if !found {
		t.Error("stats lacks optional UUID or name target")
	}
	for _, op := range []swaggerOperation{doc.Paths["/instances"]["post"], doc.Paths["/instances/{id}"]["patch"]} {
		for _, code := range []string{"422", "409"} {
			if _, ok := op.Responses[code]; !ok {
				t.Errorf("name write lacks %s response", code)
			}
		}
	}
}

func TestSwaggerSerializationPresence(t *testing.T) {
	doc := servedSwagger(t)
	for _, tc := range []struct{ path, method, key string }{
		{"/instances", "get", "instances"}, {"/users", "get", "users"}, {"/instances/{id}/groups", "get", "groups"}, {"/instances/{instance}/messages", "get", "messages"}, {"/instances/{id}/newsletters", "get", "channels"}, {"/instances/{id}/newsletters/{channel}/messages", "get", "messages"}, {"/instances/{id}/newsletters/{channel}/updates", "get", "messages"}, {"/instances/{id}/status/updates", "get", "statuses"}, {"/instances/{id}/contacts/check", "post", "contacts"}, {"/instances/{id}/blocklist", "get", "blocked_jids"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response := doc.Paths[tc.path][tc.method].Responses["200"]
			if response.Schema == nil {
				t.Fatal("missing schema")
			}
			env := resolveSwaggerSchema(t, doc, *response.Schema)
			data := resolveSwaggerSchema(t, doc, env.Properties["data"])
			collection := resolveSwaggerSchema(t, doc, data.Properties[tc.key])
			if collection.Type != "array" || collection.Items == nil || !slices.Contains(data.Required, tc.key) {
				t.Fatalf("%s must be a required typed array: %+v", tc.key, data)
			}
			if _, exists := data.Properties["items"]; exists {
				t.Error("items must be absent")
			}
			if slices.Contains(data.Required, "next_cursor") {
				t.Error("cursor must be optional")
			}
		})
	}
	for _, tc := range []struct {
		name               string
		required, optional []string
	}{
		{"representation.InstanceResponse", []string{"id", "name", "created_at", "updated_at", "connection", "integration"}, []string{"settings"}},
		{"representation.IntegrationResponse", []string{"webhook"}, []string{"chatwoot_config"}},
		{"representation.WebhookResponse", []string{"enabled", "events"}, []string{"url"}},
		{"representation.MessageResponse", []string{"retry_count", "created_at", "updated_at"}, []string{"wa_id", "media_id", "last_error", "next_attempt_at", "delivered_at", "read_at"}},
		{"representation.ChatwootConfigResponse", []string{"is_enabled", "import_days", "ignored_jids"}, []string{"url", "account_id", "organization", "logo"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema, exists := doc.Definitions[tc.name]
			if !exists {
				t.Fatal("missing definition")
			}
			for _, key := range tc.required {
				if !slices.Contains(schema.Required, key) {
					t.Errorf("%s must be required", key)
				}
			}
			for _, key := range tc.optional {
				if _, ok := schema.Properties[key]; !ok || slices.Contains(schema.Required, key) {
					t.Errorf("%s must exist and be optional", key)
				}
			}
		})
	}
	for path, methods := range doc.Paths {
		for method, op := range methods {
			for _, parameter := range op.Parameters {
				if parameter.In == "header" && strings.Contains(parameter.Name, "Idempotency") && parameter.Name != "Idempotency-Key" {
					t.Errorf("%s %s invalid header %s", method, path, parameter.Name)
				}
			}
		}
	}
}
