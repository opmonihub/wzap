package config

import (
	"os"
	"strings"
	"testing"
)

var allEnvKeys = []string{
	"WZAP_HTTP_ADDR",
	"WZAP_PUBLIC_URL",
	"WZAP_MANAGER_DEV_URL",
	"WZAP_API_KEY",
	"WZAP_DATABASE_URL",
	"WZAP_NATS_URL",
	"WZAP_NATS_STREAM",
	"WZAP_EVENT_RETENTION_DAYS",
	"WZAP_DATA_DIR",
	"WZAP_MEDIA_TTL_SECONDS",
	"WZAP_MAX_MEDIA_BYTES",
	"WZAP_OUTBOX_WORKERS",
	"WZAP_HUMANIZE",
	"WZAP_LOG_LEVEL",
	"WZAP_LOG_FORMAT",
	"WZAP_AUTO_MIGRATE",
	"WZAP_ADMIN_EMAIL",
	"WZAP_ADMIN_PASSWORD",
	"WZAP_JWT_SECRET",
	"WZAP_MAX_INSTANCES",
	"WZAP_DEFAULT_USER_INSTANCE_QUOTA",
	"WZAP_CHATWOOT_ENABLED",
	"WZAP_CHATWOOT_BOT_CONTACT",
	"WZAP_CHATWOOT_MESSAGE_READ",
	"WZAP_CHATWOOT_MESSAGE_DELETE",
	"WZAP_CHATWOOT_IMPORT_DB_URL",
	"WZAP_CHATWOOT_IMPORT_PLACEHOLDER",
	"WZAP_CHATWOOT_TOKEN_KEY",
}

func TestLoadRejectsInvalidManagerDevURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"relative", "manager-dev:3000"},
		{"scheme_relative", "//manager-dev:3000"},
		{"unsupported_scheme", "ftp://manager-dev:3000"},
		{"missing_host", "http:///"},
		{"missing_hostname", "http://:3000"},
		{"opaque", "http:manager-dev"},
		{"credentials", "http://sensitive-user:sensitive-password@manager-dev:3000"},
		{"username", "http://sensitive-user@manager-dev:3000"},
		{"query", "http://manager-dev:3000?sensitive-token=secret"},
		{"empty_query", "http://manager-dev:3000?"},
		{"fragment", "http://manager-dev:3000#sensitive-fragment"},
		{"empty_fragment", "http://manager-dev:3000#"},
		{"path_prefix", "http://manager-dev:3000/manager/"},
		{"encoded_path", "http://manager-dev:3000/%2F"},
		{"invalid_escape", "http://manager-dev:3000/%sensitive"},
		{"invalid_port", "http://manager-dev:sensitive-port"},
		{"whitespace", " http://manager-dev:3000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv("WZAP_MANAGER_DEV_URL", tt.url)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want invalid manager dev URL rejected")
			}
			if !strings.Contains(err.Error(), "WZAP_MANAGER_DEV_URL") {
				t.Errorf("Load() error = %q, want it to name WZAP_MANAGER_DEV_URL", err)
			}
			if strings.Contains(err.Error(), tt.url) || strings.Contains(err.Error(), "sensitive") {
				t.Errorf("Load() error = %q, want error without supplied URL or sensitive values", err)
			}
		})
	}
}

func TestLoadManagerDevURL(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		absent bool
	}{
		{name: "absent", absent: true},
		{name: "empty"},
		{name: "http", value: "http://manager-dev:3000"},
		{name: "http_root", value: "http://manager-dev:3000/"},
		{name: "https", value: "https://manager.example.com"},
		{name: "https_root", value: "https://manager.example.com/"},
		{name: "ipv4", value: "http://127.0.0.1:3000"},
		{name: "ipv6", value: "http://[::1]:3000/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv("WZAP_MANAGER_DEV_URL", tt.value)
			if tt.absent {
				if err := os.Unsetenv("WZAP_MANAGER_DEV_URL"); err != nil {
					t.Fatal(err)
				}
			}

			got, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.ManagerDevURL != tt.value {
				t.Errorf("Load().ManagerDevURL = %q, want %q", got.ManagerDevURL, tt.value)
			}
		})
	}
}

// clearWZAPEnv clears every WZAP_* variable for the test, overriding and later
// restoring any ambient value. Empty is treated as unset by config.Load.
func clearWZAPEnv(t *testing.T) {
	t.Helper()
	for _, key := range allEnvKeys {
		t.Setenv(key, "")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WZAP_API_KEY", "test-api-key")
	t.Setenv("WZAP_JWT_SECRET", "test-jwt-secret-0123456789abcdef01")
	t.Setenv("WZAP_DATABASE_URL", "postgres://wzap:secret@127.0.0.1:5432/wzap")
	t.Setenv("WZAP_NATS_URL", "nats://127.0.0.1:4222")
}

func TestLoadFullConfig(t *testing.T) {
	clearWZAPEnv(t)
	for key, value := range map[string]string{
		"WZAP_HTTP_ADDR":                   "127.0.0.1:9999",
		"WZAP_PUBLIC_URL":                  "https://wzap.example.com",
		"WZAP_API_KEY":                     "api-key",
		"WZAP_ADMIN_EMAIL":                 "admin@example.com",
		"WZAP_ADMIN_PASSWORD":              "s3cret",
		"WZAP_JWT_SECRET":                  "full-config-jwt-secret-0123456789",
		"WZAP_MAX_INSTANCES":               "10",
		"WZAP_DEFAULT_USER_INSTANCE_QUOTA": "3",
		"WZAP_DATABASE_URL":                "postgres://wzap:secret@db:5432/wzap",
		"WZAP_NATS_URL":                    "nats://nats:4222",
		"WZAP_NATS_STREAM":                 "WZAP_TEST",
		"WZAP_EVENT_RETENTION_DAYS":        "30",
		"WZAP_DATA_DIR":                    "/tmp/wzap",
		"WZAP_MEDIA_TTL_SECONDS":           "60",
		"WZAP_MAX_MEDIA_BYTES":             "1024",
		"WZAP_OUTBOX_WORKERS":              "2",
		"WZAP_HUMANIZE":                    "true",
		"WZAP_LOG_LEVEL":                   "debug",
		"WZAP_LOG_FORMAT":                  "console",
		"WZAP_AUTO_MIGRATE":                "false",
		"WZAP_CHATWOOT_ENABLED":            "true",
		"WZAP_CHATWOOT_TOKEN_KEY":          "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		"WZAP_CHATWOOT_BOT_CONTACT":        "123456",
		"WZAP_CHATWOOT_MESSAGE_READ":       "true",
		"WZAP_CHATWOOT_MESSAGE_DELETE":     "true",
		"WZAP_CHATWOOT_IMPORT_DB_URL":      "postgres://chatwoot:secret@db:5432/chatwoot",
		"WZAP_CHATWOOT_IMPORT_PLACEHOLDER": "true",
		"WZAP_S3_ENDPOINT":                 "http://minio:9000",
		"WZAP_S3_BUCKET":                   "media-bucket",
		"WZAP_S3_REGION":                   "br-east-1",
		"WZAP_S3_ACCESS_KEY":               "minio-user",
		"WZAP_S3_SECRET_KEY":               "minio-pass",
		"WZAP_S3_USE_TLS":                  "true",
	} {
		t.Setenv(key, value)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		HTTPAddr:           "127.0.0.1:9999",
		PublicURL:          "https://wzap.example.com",
		APIKey:             "api-key",
		AdminEmail:         "admin@example.com",
		AdminPassword:      "s3cret",
		JWTSecret:          "full-config-jwt-secret-0123456789",
		MaxInstances:       10,
		DefaultUserQuota:   3,
		DatabaseURL:        "postgres://wzap:secret@db:5432/wzap",
		NATSURL:            "nats://nats:4222",
		NATSStream:         "WZAP_TEST",
		EventRetentionDays: 30,
		DataDir:            "/tmp/wzap",
		MediaTTLSeconds:    60,
		MaxMediaBytes:      1024,
		OutboxWorkers:      2,
		Humanize:           true,
		LogLevel:           "debug",
		LogFormat:          "console",
		AutoMigrate:        false,
		Chatwoot: Chatwoot{
			Enabled:           true,
			BotContact:        "123456",
			MessageRead:       true,
			MessageDelete:     true,
			ImportDBURL:       "postgres://chatwoot:secret@db:5432/chatwoot",
			ImportPlaceholder: true,
			TokenKey:          "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
		},
		S3: S3{
			Endpoint:  "http://minio:9000",
			Bucket:    "media-bucket",
			Region:    "br-east-1",
			AccessKey: "minio-user",
			SecretKey: "minio-pass",
			UseTLS:    true,
		},
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadDefaults(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		HTTPAddr:           ":8080",
		PublicURL:          "",
		APIKey:             "test-api-key",
		AdminEmail:         "",
		AdminPassword:      "",
		JWTSecret:          "test-jwt-secret-0123456789abcdef01",
		MaxInstances:       0,
		DefaultUserQuota:   0,
		DatabaseURL:        "postgres://wzap:secret@127.0.0.1:5432/wzap",
		NATSURL:            "nats://127.0.0.1:4222",
		NATSStream:         "WZAP",
		EventRetentionDays: 7,
		DataDir:            "/data",
		MediaTTLSeconds:    7200,
		MaxMediaBytes:      16777216,
		OutboxWorkers:      4,
		Humanize:           false,
		LogLevel:           "info",
		LogFormat:          "json",
		AutoMigrate:        true,
		Chatwoot:           Chatwoot{},
		S3:                 S3{Bucket: "wzap-media", Region: "us-east-1"},
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	t.Setenv("WZAP_JWT_SECRET", "too-short")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error naming WZAP_JWT_SECRET")
	}
	if !strings.Contains(err.Error(), "WZAP_JWT_SECRET") {
		t.Errorf("Load() error = %q, want it to mention WZAP_JWT_SECRET", err)
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	clearWZAPEnv(t)
	t.Setenv("WZAP_JWT_SECRET", "missing-api-key-jwt-secret-012345")
	t.Setenv("WZAP_DATABASE_URL", "postgres://wzap:secret@127.0.0.1:5432/wzap")
	t.Setenv("WZAP_NATS_URL", "nats://127.0.0.1:4222")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error naming WZAP_API_KEY")
	}
	if !strings.Contains(err.Error(), "WZAP_API_KEY") {
		t.Errorf("Load() error = %q, want it to mention WZAP_API_KEY", err)
	}
}

func TestLoadMissingJWTSecret(t *testing.T) {
	clearWZAPEnv(t)
	t.Setenv("WZAP_API_KEY", "api-key")
	t.Setenv("WZAP_DATABASE_URL", "postgres://wzap:secret@127.0.0.1:5432/wzap")
	t.Setenv("WZAP_NATS_URL", "nats://127.0.0.1:4222")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error naming WZAP_JWT_SECRET")
	}
	if !strings.Contains(err.Error(), "WZAP_JWT_SECRET") {
		t.Errorf("Load() error = %q, want it to mention WZAP_JWT_SECRET", err)
	}
}

func TestLoadEmptyRequiredTreatedAsMissing(t *testing.T) {
	for _, key := range []string{"WZAP_API_KEY", "WZAP_JWT_SECRET"} {
		t.Run(key, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(key, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, key)
			}
		})
	}
}

func TestLoadRejectsInvalidLogFormat(t *testing.T) {
	for _, format := range []string{"text", "xml", "pretty"} {
		t.Run(format, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv("WZAP_LOG_FORMAT", format)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil with format %q, want error naming WZAP_LOG_FORMAT", format)
			}
			if !strings.Contains(err.Error(), "WZAP_LOG_FORMAT") {
				t.Errorf("Load() error = %q, want it to mention WZAP_LOG_FORMAT", err)
			}
		})
	}
}

func TestLoadExplicitZeroQuotasMeanUnlimited(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	t.Setenv("WZAP_MAX_INSTANCES", "0")
	t.Setenv("WZAP_DEFAULT_USER_INSTANCE_QUOTA", "0")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.MaxInstances != 0 {
		t.Errorf("Load().MaxInstances = %d, want 0 (unlimited)", got.MaxInstances)
	}
	if got.DefaultUserQuota != 0 {
		t.Errorf("Load().DefaultUserQuota = %d, want 0 (unlimited)", got.DefaultUserQuota)
	}
}

func TestLoadRejectsNegativeQuotas(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"WZAP_MAX_INSTANCES", "-1"},
		{"WZAP_DEFAULT_USER_INSTANCE_QUOTA", "-2"},
	}

	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.value, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, tt.key)
			}
		})
	}
}

func TestLoadRejectsNonNumericQuotas(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"WZAP_MAX_INSTANCES", "many"},
		{"WZAP_DEFAULT_USER_INSTANCE_QUOTA", "lots"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, tt.key)
			}
		})
	}
}

func TestLoadMissingDatabaseAndNATS(t *testing.T) {
	clearWZAPEnv(t)
	t.Setenv("WZAP_API_KEY", "api-key")
	t.Setenv("WZAP_JWT_SECRET", "missing-db-nats-jwt-secret-01234567")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error naming both missing variables")
	}
	for _, key := range []string{"WZAP_DATABASE_URL", "WZAP_NATS_URL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("Load() error = %q, want it to mention %s", err, key)
		}
	}
}

func TestLoadRejectsNonPositiveNumbers(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"WZAP_EVENT_RETENTION_DAYS", "0"},
		{"WZAP_MEDIA_TTL_SECONDS", "-1"},
		{"WZAP_MAX_MEDIA_BYTES", "0"},
		{"WZAP_OUTBOX_WORKERS", "-2"},
	}

	for _, tt := range tests {
		t.Run(tt.key+"/"+tt.value, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, tt.key)
			}
			if !strings.Contains(err.Error(), "positive") {
				t.Errorf("Load() error = %q, want it to say the value must be positive", err)
			}
		})
	}
}

func TestLoadAggregatesProblems(t *testing.T) {
	clearWZAPEnv(t)
	t.Setenv("WZAP_EVENT_RETENTION_DAYS", "0")
	t.Setenv("WZAP_MEDIA_TTL_SECONDS", "0")
	t.Setenv("WZAP_MAX_MEDIA_BYTES", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want the aggregated error")
	}
	for _, key := range []string{
		"WZAP_API_KEY", "WZAP_JWT_SECRET", "WZAP_DATABASE_URL", "WZAP_NATS_URL",
		"WZAP_EVENT_RETENTION_DAYS", "WZAP_MEDIA_TTL_SECONDS", "WZAP_MAX_MEDIA_BYTES",
	} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("Load() error = %q, want it to mention %s", err, key)
		}
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"WZAP_EVENT_RETENTION_DAYS", "many"},
		{"WZAP_MEDIA_TTL_SECONDS", "soon"},
		{"WZAP_MAX_MEDIA_BYTES", "huge"},
		{"WZAP_OUTBOX_WORKERS", "all"},
		{"WZAP_HUMANIZE", "maybe"},
		{"WZAP_AUTO_MIGRATE", "perhaps"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, tt.key)
			}
		})
	}
}

func TestLoadChatwootDefaultsWhenAbsent(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Chatwoot != (Chatwoot{}) {
		t.Errorf("Load().Chatwoot = %+v, want zero value", got.Chatwoot)
	}
}

func TestLoadChatwootValid(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	t.Setenv("WZAP_CHATWOOT_ENABLED", "true")
	// base64 of 32 0x01 bytes; an enabled connector requires the seal key.
	t.Setenv("WZAP_CHATWOOT_TOKEN_KEY", "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")
	t.Setenv("WZAP_CHATWOOT_BOT_CONTACT", "123456")
	t.Setenv("WZAP_CHATWOOT_MESSAGE_READ", "true")
	t.Setenv("WZAP_CHATWOOT_MESSAGE_DELETE", "1")
	t.Setenv("WZAP_CHATWOOT_IMPORT_PLACEHOLDER", "true")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Chatwoot{
		Enabled: true, BotContact: "123456", MessageRead: true, MessageDelete: true,
		ImportPlaceholder: true, TokenKey: "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=",
	}
	if got.Chatwoot != want {
		t.Errorf("Load().Chatwoot = %+v, want %+v", got.Chatwoot, want)
	}
}

func TestLoadChatwootRejectsInvalidBool(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"WZAP_CHATWOOT_ENABLED", "banana"},
		{"WZAP_CHATWOOT_MESSAGE_READ", "banana"},
		{"WZAP_CHATWOOT_MESSAGE_DELETE", "banana"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want error naming %s", tt.key)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("Load() error = %q, want it to mention %s", err, tt.key)
			}
		})
	}
}

func TestLoadChatwootTokenKeyValid(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	// base64 of 32 0x01 bytes.
	t.Setenv("WZAP_CHATWOOT_TOKEN_KEY", "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Chatwoot.TokenKey != "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=" {
		t.Errorf("Load().Chatwoot.TokenKey = %q, want the configured key", got.Chatwoot.TokenKey)
	}
}

func TestLoadChatwootTokenKeyRejectsMalformed(t *testing.T) {
	for _, raw := range []string{"not-base64!!", "dG9vLXNob3J0", "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="[:40]} {
		t.Run(raw, func(t *testing.T) {
			clearWZAPEnv(t)
			setRequiredEnv(t)
			t.Setenv("WZAP_CHATWOOT_TOKEN_KEY", raw)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want error naming WZAP_CHATWOOT_TOKEN_KEY")
			}
			if !strings.Contains(err.Error(), "WZAP_CHATWOOT_TOKEN_KEY") {
				t.Errorf("Load() error = %q, want it to mention WZAP_CHATWOOT_TOKEN_KEY", err)
			}
		})
	}
}

func TestLoadChatwootEnabledRequiresTokenKey(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	t.Setenv("WZAP_CHATWOOT_ENABLED", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil with chatwoot enabled, want error naming WZAP_CHATWOOT_TOKEN_KEY")
	}
	if !strings.Contains(err.Error(), "WZAP_CHATWOOT_TOKEN_KEY") {
		t.Errorf("Load() error = %q, want it to mention WZAP_CHATWOOT_TOKEN_KEY", err)
	}
}

func TestLoadChatwootDisabledWithoutTokenKeyLoads(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want disabled chatwoot to load without WZAP_CHATWOOT_TOKEN_KEY", err)
	}
	if got.Chatwoot.TokenKey != "" {
		t.Errorf("Load().Chatwoot.TokenKey = %q, want empty when WZAP_CHATWOOT_TOKEN_KEY is absent", got.Chatwoot.TokenKey)
	}
}

func TestLoadChatwootImportDBURL(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Chatwoot.ImportDBURL != "" {
		t.Errorf("Load().Chatwoot.ImportDBURL = %q, want empty when WZAP_CHATWOOT_IMPORT_DB_URL is absent", got.Chatwoot.ImportDBURL)
	}

	t.Setenv("WZAP_CHATWOOT_IMPORT_DB_URL", "postgres://chatwoot:secret@db:5432/chatwoot")
	got, err = Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Chatwoot.ImportDBURL != "postgres://chatwoot:secret@db:5432/chatwoot" {
		t.Errorf("Load().Chatwoot.ImportDBURL = %q, want the configured URI", got.Chatwoot.ImportDBURL)
	}
}

func TestLoadS3RequiresCredentialsWhenEndpointSet(t *testing.T) {
	clearWZAPEnv(t)
	setRequiredEnv(t)
	t.Setenv("WZAP_S3_ENDPOINT", "http://minio:9000")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error naming the S3 credentials")
	}
	if !strings.Contains(err.Error(), "WZAP_S3_ACCESS_KEY") || !strings.Contains(err.Error(), "WZAP_S3_SECRET_KEY") {
		t.Errorf("Load() error = %q, want it to mention WZAP_S3_ACCESS_KEY and WZAP_S3_SECRET_KEY", err)
	}

	t.Setenv("WZAP_S3_ACCESS_KEY", "u")
	t.Setenv("WZAP_S3_SECRET_KEY", "p")
	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.S3.Bucket != "wzap-media" || got.S3.Region != "us-east-1" || got.S3.UseTLS {
		t.Errorf("S3 defaults = %+v, want bucket wzap-media, region us-east-1, TLS off", got.S3)
	}
}
