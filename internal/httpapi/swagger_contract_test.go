package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

type swaggerSchema struct {
	Ref        string                   `json:"$ref"`
	Type       string                   `json:"type"`
	Properties map[string]swaggerSchema `json:"properties"`
	Items      *swaggerSchema           `json:"items"`
	AllOf      []swaggerSchema          `json:"allOf"`
}

type swaggerOperation struct {
	Parameters []struct {
		Name     string         `json:"name"`
		In       string         `json:"in"`
		Required bool           `json:"required"`
		Schema   *swaggerSchema `json:"schema"`
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
	Paths       map[string]map[string]swaggerOperation `json:"paths"`
	Definitions map[string]swaggerSchema               `json:"definitions"`
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
	file, err := parser.ParseFile(token.NewFileSet(), "server.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Handle" && selector.Sel.Name != "HandleFunc") {
			return true
		}
		if len(call.Args) != 2 {
			t.Errorf("unsupported route registration: %s argument count %d", selector.Sel.Name, len(call.Args))
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Errorf("unsupported nonliteral route registration at byte %d", call.Pos())
			return true
		}
		pattern, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Error(err)
			return true
		}
		if strings.HasPrefix(pattern, "/") {
			return true // Methodless static mounts and API fallbacks are not operations.
		}
		method, path, ok := strings.Cut(pattern, " ")
		if !ok || !strings.HasPrefix(path, "/") || strings.Contains(path, " ") {
			t.Errorf("unsupported route pattern %q", pattern)
			return true
		}
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		default:
			t.Errorf("unsupported route method %q", method)
			return true
		}
		count++
		op, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			t.Errorf("Swagger missing registered operation %s", pattern)
			return true
		}
		for _, segment := range strings.Split(path, "/") {
			if !strings.HasPrefix(segment, "{") {
				continue
			}
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			found := false
			for _, parameter := range op.Parameters {
				if parameter.In == "path" && parameter.Name == name && parameter.Required {
					found = true
				}
			}
			if !found {
				t.Errorf("%s missing required path parameter %s", pattern, name)
			}
		}
		return true
	})
	if count == 0 {
		t.Fatal("no method/path registrations inspected")
	}
	t.Logf("checked %d registered operations", count)
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
						return // HTML, redirects and plain errors have their own contract below.
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
		path, method, status, field, fieldType string
	}{
		{"/healthz", "get", "200", "status", "string"},
		{"/readyz", "get", "503", "checks", "object"},
		{"/instances/{id}", "get", "200", "id", "string"},
		{"/instances/{id}/messages/text", "post", "202", "message_id", "string"},
		{"/instances/{id}/chatwoot/import", "post", "202", "imported", "integer"},
		{"/instances/{id}/chatwoot/command", "post", "200", "ok", "boolean"},
		{"/instances/{id}/chatwoot", "get", "200", "token", "string"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := doc.Paths[test.path][test.method].Responses[test.status]
			if response.Schema == nil {
				t.Fatal("missing response schema")
			}
			envelope := resolveSwaggerSchema(t, doc, *response.Schema)
			payload := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
			if payload.Properties[test.field].Type != test.fieldType {
				t.Errorf("data.%s type = %q, want %q", test.field, payload.Properties[test.field].Type, test.fieldType)
			}
		})
	}
	for _, test := range []struct{ path, itemField string }{
		{"/users", "email"},
		{"/instances", "id"},
		{"/instances/{id}/messages", "id"},
		{"/instances/{id}/groups", "jid"},
	} {
		t.Run("collection "+test.path, func(t *testing.T) {
			response := doc.Paths[test.path]["get"].Responses["200"]
			if response.Schema == nil {
				t.Fatal("missing response schema")
			}
			envelope := resolveSwaggerSchema(t, doc, *response.Schema)
			collection := resolveSwaggerSchema(t, doc, envelope.Properties["data"])
			if test.path != "/users" {
				collection = resolveSwaggerSchema(t, doc, collection.Properties["items"])
			}
			if collection.Type != "array" || collection.Items == nil {
				t.Fatal("missing typed collection")
			}
			item := resolveSwaggerSchema(t, doc, *collection.Items)
			if item.Properties[test.itemField].Type != "string" {
				t.Errorf("collection item missing string %s", test.itemField)
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
				if parameter.In == "header" && parameter.Name == "apikey" && parameter.Required {
					t.Errorf("%s %s apikey header must be optional for session-cookie callers", method, path)
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
				if schema.Properties["instance_quota"].Type != "integer" {
					t.Error("instance_quota must be documented as an integer")
				}
				return
			}
			t.Fatal("missing body parameter")
		})
	}
}
