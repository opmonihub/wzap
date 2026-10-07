package representation

import (
	"encoding/json"
	"testing"
	"time"

	"wzap/internal/model"
)

func TestSerializationPresenceMatrix(t *testing.T) {
	timer := time.Duration(0)
	for _, tt := range []struct {
		name     string
		instance model.Instance
		settings bool
	}{
		{name: "absent"}, {name: "zero timer", instance: model.Instance{DefaultDisappearing: &timer}, settings: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(NewInstanceResponse(&tt.instance))
			if err != nil {
				t.Fatal(err)
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(b, &obj); err != nil {
				t.Fatal(err)
			}
			_, present := obj["settings"]
			if present != tt.settings {
				t.Errorf("settings presence=%v want%v: %s", present, tt.settings, b)
			}
			var integration map[string]json.RawMessage
			if err := json.Unmarshal(obj["integration"], &integration); err != nil {
				t.Fatal(err)
			}
			if _, ok := integration["chatwoot_config"]; ok {
				t.Error("absent chatwoot must be omitted")
			}
			var hook map[string]json.RawMessage
			if err := json.Unmarshal(integration["webhook"], &hook); err != nil {
				t.Fatal(err)
			}
			if string(hook["enabled"]) != "false" || string(hook["events"]) != "[]" {
				t.Errorf("required zeros=%s", integration["webhook"])
			}
			if _, ok := hook["url"]; ok {
				t.Error("absent URL must be omitted")
			}
		})
	}
}
