package message

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wzap/internal/session"
)

// richEnqueueInput builds a valid poll input for the service tests.
func richPollInput() EnqueueInput {
	return EnqueueInput{
		Type:                TypePoll,
		To:                  "5547988359190",
		PollQuestion:        "Qual o melhor horário?",
		PollOptions:         []string{"manhã", "tarde"},
		PollSelectableCount: 1,
	}
}

func TestEnqueuePollAccepted(t *testing.T) {
	fixture := newServiceFixture(session.StatusConnected)

	id, err := fixture.service.Enqueue(context.Background(), fixture.instance, richPollInput())
	if err != nil {
		t.Fatalf("Enqueue poll: %v", err)
	}
	if id.String() == "" {
		t.Fatal("Enqueue poll returned an empty id")
	}
	if len(fixture.messages.created) != 1 {
		t.Fatalf("created messages = %d, want 1", len(fixture.messages.created))
	}
	stored := fixture.messages.created[0]
	if stored.Type != TypePoll {
		t.Errorf("stored type = %q, want %q", stored.Type, TypePoll)
	}
	payload := decodePayload(t, stored)
	if payload["question"] != "Qual o melhor horário?" {
		t.Errorf("payload question = %v, want the poll question", payload["question"])
	}
	options, _ := payload["options"].([]any)
	if len(options) != 2 {
		t.Errorf("payload options = %v, want the two poll options", payload["options"])
	}
}

func TestEnqueuePollInvalid(t *testing.T) {
	question300 := strings.Repeat("p", 300)
	question301 := strings.Repeat("p", 301)
	option100 := strings.Repeat("o", 100)
	option101 := strings.Repeat("o", 101)
	manyOptions := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"}
	twelveOptions := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}

	tests := []struct {
		name   string
		mutate func(*EnqueueInput)
	}{
		{name: "empty question", mutate: func(in *EnqueueInput) { in.PollQuestion = "" }},
		{name: "question over 300", mutate: func(in *EnqueueInput) { in.PollQuestion = question301 }},
		{name: "question at 300 ok", mutate: func(in *EnqueueInput) { in.PollQuestion = question300 }},
		{name: "single option", mutate: func(in *EnqueueInput) { in.PollOptions = []string{"só"} }},
		{name: "no options", mutate: func(in *EnqueueInput) { in.PollOptions = nil }},
		{name: "thirteen options", mutate: func(in *EnqueueInput) { in.PollOptions = manyOptions }},
		{name: "empty option", mutate: func(in *EnqueueInput) { in.PollOptions = []string{"ok", ""} }},
		{name: "option over 100", mutate: func(in *EnqueueInput) { in.PollOptions = []string{"ok", option101} }},
		{name: "selectable two", mutate: func(in *EnqueueInput) { in.PollSelectableCount = 2 }},
		{name: "selectable negative", mutate: func(in *EnqueueInput) { in.PollSelectableCount = -1 }},
	}
	_ = twelveOptions
	_ = option100

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newServiceFixture(session.StatusConnected)
			input := richPollInput()
			tt.mutate(&input)
			_, err := fixture.service.Enqueue(context.Background(), fixture.instance, input)
			if tt.name == "question at 300 ok" {
				if err != nil {
					t.Fatalf("Enqueue poll at the limit: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Enqueue poll error = nil, want invalid input")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Enqueue poll error = %v, want ErrInvalidInput", err)
			}
			if len(fixture.messages.created) != 0 {
				t.Errorf("created messages = %d, want none persisted", len(fixture.messages.created))
			}
		})
	}
}

func TestEnqueueReactionAccepted(t *testing.T) {
	for _, emoji := range []string{"👍", ""} {
		fixture := newServiceFixture(session.StatusConnected)
		_, err := fixture.service.Enqueue(context.Background(), fixture.instance, EnqueueInput{
			Type:           TypeReaction,
			To:             "5547988359190",
			ReactionTarget: "wamid.orig",
			ReactionEmoji:  emoji,
		})
		if err != nil {
			t.Fatalf("Enqueue reaction %q: %v", emoji, err)
		}
		if len(fixture.messages.created) != 1 {
			t.Fatalf("created messages = %d, want 1", len(fixture.messages.created))
		}
		payload := decodePayload(t, fixture.messages.created[0])
		if payload["target"] != "wamid.orig" {
			t.Errorf("payload target = %v, want the reacted message", payload["target"])
		}
		if payload["emoji"] != emoji {
			t.Errorf("payload emoji = %v, want %q (empty removes)", payload["emoji"], emoji)
		}
	}
}

func TestEnqueueReactionInvalid(t *testing.T) {
	tests := []struct {
		name   string
		target string
		emoji  string
	}{
		{name: "missing target", target: "", emoji: "👍"},
		{name: "emoji over 32", target: "wamid.orig", emoji: strings.Repeat("e", 33)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newServiceFixture(session.StatusConnected)
			_, err := fixture.service.Enqueue(context.Background(), fixture.instance, EnqueueInput{
				Type:           TypeReaction,
				To:             "5547988359190",
				ReactionTarget: tt.target,
				ReactionEmoji:  tt.emoji,
			})
			if err == nil {
				t.Fatal("Enqueue reaction error = nil, want invalid input")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Enqueue reaction error = %v, want ErrInvalidInput", err)
			}
			if len(fixture.messages.created) != 0 {
				t.Errorf("created messages = %d, want none persisted", len(fixture.messages.created))
			}
		})
	}
}

func TestEnqueueListAccepted(t *testing.T) {
	fixture := newServiceFixture(session.StatusConnected)

	_, err := fixture.service.Enqueue(context.Background(), fixture.instance, EnqueueInput{
		Type:       TypeList,
		To:         "5547988359190",
		ListTitle:  "Cardápio",
		ListButton: "Ver opções",
		ListSections: []ListSection{
			{Title: "Lanches", Rows: []ListRow{
				{ID: "x-burger", Title: "X-Burger", Description: "pão, carne e queijo"},
				{ID: "x-salada", Title: "X-Salada"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("Enqueue list: %v", err)
	}
	if len(fixture.messages.created) != 1 {
		t.Fatalf("created messages = %d, want 1", len(fixture.messages.created))
	}
}

func TestEnqueueListInvalid(t *testing.T) {
	manySections := make([]ListSection, 11)
	for i := range manySections {
		manySections[i] = ListSection{Title: "s", Rows: []ListRow{{ID: "r", Title: "r"}}}
	}
	manyRows := make([]ListRow, 11)
	for i := range manyRows {
		manyRows[i] = ListRow{ID: "r", Title: "r"}
	}
	tests := []struct {
		name   string
		mutate func(*EnqueueInput)
	}{
		{name: "missing button", mutate: func(in *EnqueueInput) { in.ListButton = "" }},
		{name: "no sections", mutate: func(in *EnqueueInput) { in.ListSections = nil }},
		{name: "eleven sections", mutate: func(in *EnqueueInput) { in.ListSections = manySections }},
		{name: "section without rows", mutate: func(in *EnqueueInput) {
			in.ListSections = []ListSection{{Title: "vazia"}}
		}},
		{name: "eleven rows", mutate: func(in *EnqueueInput) {
			in.ListSections = []ListSection{{Title: "s", Rows: manyRows}}
		}},
		{name: "row without id", mutate: func(in *EnqueueInput) {
			in.ListSections = []ListSection{{Title: "s", Rows: []ListRow{{Title: "sem id"}}}}
		}},
		{name: "text over 300", mutate: func(in *EnqueueInput) { in.ListButton = strings.Repeat("b", 301) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newServiceFixture(session.StatusConnected)
			input := EnqueueInput{
				Type:       TypeList,
				To:         "5547988359190",
				ListButton: "Ver opções",
				ListSections: []ListSection{
					{Title: "Lanches", Rows: []ListRow{{ID: "x", Title: "X"}}},
				},
			}
			tt.mutate(&input)
			_, err := fixture.service.Enqueue(context.Background(), fixture.instance, input)
			if err == nil {
				t.Fatal("Enqueue list error = nil, want invalid input")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Enqueue list error = %v, want ErrInvalidInput", err)
			}
			if len(fixture.messages.created) != 0 {
				t.Errorf("created messages = %d, want none persisted", len(fixture.messages.created))
			}
		})
	}
}

func TestEnqueueButtonsAccepted(t *testing.T) {
	fixture := newServiceFixture(session.StatusConnected)

	_, err := fixture.service.Enqueue(context.Background(), fixture.instance, EnqueueInput{
		Type:        TypeButtons,
		To:          "5547988359190",
		ButtonsText: "Escolha a forma de pagamento (PIX: loja@example.com)",
		Buttons: []Button{
			{ID: "pix-copia", Title: "Copiar chave PIX"},
			{ID: "dinheiro", Title: "Dinheiro"},
		},
	})
	if err != nil {
		t.Fatalf("Enqueue buttons: %v", err)
	}
	if len(fixture.messages.created) != 1 {
		t.Fatalf("created messages = %d, want 1", len(fixture.messages.created))
	}
}

func TestEnqueueButtonsInvalid(t *testing.T) {
	four := []Button{{ID: "a", Title: "a"}, {ID: "b", Title: "b"}, {ID: "c", Title: "c"}, {ID: "d", Title: "d"}}
	tests := []struct {
		name   string
		mutate func(*EnqueueInput)
	}{
		{name: "missing text", mutate: func(in *EnqueueInput) { in.ButtonsText = "" }},
		{name: "no buttons", mutate: func(in *EnqueueInput) { in.Buttons = nil }},
		{name: "four buttons", mutate: func(in *EnqueueInput) { in.Buttons = four }},
		{name: "button without id", mutate: func(in *EnqueueInput) {
			in.Buttons = []Button{{Title: "sem id"}}
		}},
		{name: "title over 64", mutate: func(in *EnqueueInput) {
			in.Buttons = []Button{{ID: "a", Title: strings.Repeat("t", 65)}}
		}},
		{name: "id over 64", mutate: func(in *EnqueueInput) {
			in.Buttons = []Button{{ID: strings.Repeat("i", 65), Title: "a"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newServiceFixture(session.StatusConnected)
			input := EnqueueInput{
				Type:        TypeButtons,
				To:          "5547988359190",
				ButtonsText: "Escolha",
				Buttons:     []Button{{ID: "a", Title: "A"}},
			}
			tt.mutate(&input)
			_, err := fixture.service.Enqueue(context.Background(), fixture.instance, input)
			if err == nil {
				t.Fatal("Enqueue buttons error = nil, want invalid input")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Enqueue buttons error = %v, want ErrInvalidInput", err)
			}
			if len(fixture.messages.created) != 0 {
				t.Errorf("created messages = %d, want none persisted", len(fixture.messages.created))
			}
		})
	}
}

func TestEnqueueRichDisconnected(t *testing.T) {
	for _, typ := range []string{TypePoll, TypeReaction, TypeList, TypeButtons} {
		fixture := newServiceFixture(session.StatusDisconnected)
		_, err := fixture.service.Enqueue(context.Background(), fixture.instance, EnqueueInput{Type: typ, To: "5547"})
		if err == nil {
			t.Fatalf("Enqueue %s error = nil, want instance not connected", typ)
		}
		if !errors.Is(err, ErrInstanceNotConnected) {
			t.Errorf("Enqueue %s error = %v, want ErrInstanceNotConnected", typ, err)
		}
	}
}
