package commands

import (
	"testing"

	"github.com/basecamp/fizzy-cli/internal/client"
	"github.com/basecamp/fizzy-cli/internal/errors"
)

func setupEpicTest(t *testing.T) *MockClient {
	t.Helper()
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	SetTestConfig("token", "account", "https://api.example.com")
	t.Cleanup(func() {
		resetTest()
		epicListBoard, epicShowBoard, epicCreateBoard, epicUpdateBoard, epicDeleteBoard = "", "", "", "", ""
		epicCardsBoard, epicCardsPage, epicCardsAll = "", 0, false
		epicAddCardBoard, epicAddCardNumber, epicRemoveCardNumber = "", "", ""
		epicCreateAttrs, epicUpdateAttrs = epicAttrs{}, epicAttrs{}
	})
	return mock
}

func TestEpicList(t *testing.T) {
	t.Run("lists the board epics", func(t *testing.T) {
		mock := setupEpicTest(t)
		mock.OnGet("/boards/123/epics.json", &client.APIResponse{
			StatusCode: 200,
			Data: []any{
				map[string]any{"id": "e1", "name": "Login", "derived_status": "completed", "progress": map[string]any{"percent": 100}},
				map[string]any{"id": "e2", "name": "Matrícula", "derived_status": "in_progress", "wave_id": "w1"},
			},
		})

		epicListBoard = "123"
		err := epicListCmd.RunE(epicListCmd, []string{})

		assertExitCode(t, err, 0)
		if mock.GetCalls[0].Path != "/boards/123/epics.json" {
			t.Errorf("expected epics path, got %q", mock.GetCalls[0].Path)
		}
	})

	t.Run("requires a board", func(t *testing.T) {
		setupEpicTest(t)

		err := epicListCmd.RunE(epicListCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}

func TestEpicShow(t *testing.T) {
	mock := setupEpicTest(t)
	mock.OnGet("/boards/123/epics/e1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "e1", "name": "Login"}})

	epicShowBoard = "123"
	err := epicShowCmd.RunE(epicShowCmd, []string{"e1"})

	assertExitCode(t, err, 0)
	if mock.GetCalls[0].Path != "/boards/123/epics/e1.json" {
		t.Errorf("expected epic path, got %q", mock.GetCalls[0].Path)
	}
}

func TestEpicCreate(t *testing.T) {
	t.Run("posts the nested epic attributes", func(t *testing.T) {
		mock := setupEpicTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]any{"id": "e9", "name": "Painel"}}

		epicCreateBoard = "123"
		epicCreateAttrs = epicAttrs{name: "Painel", color: "violet", start: "2026-09-14", target: "2026-11-06", status: "in_progress", wave: "w1", owner: "u1", description: "Dashboards"}
		err := epicCreateCmd.RunE(epicCreateCmd, []string{})

		assertExitCode(t, err, 0)
		if mock.PostCalls[0].Path != "/boards/123/epics.json" {
			t.Errorf("expected epics path, got %q", mock.PostCalls[0].Path)
		}
		epic := mock.PostCalls[0].Body.(map[string]any)["epic"].(map[string]any)
		expected := map[string]any{
			"name": "Painel", "color": "var(--color-card-6)", "start_date": "2026-09-14", "target_date": "2026-11-06",
			"status": "in_progress", "wave_id": "w1", "owner_id": "u1", "description": "Dashboards",
		}
		for key, value := range expected {
			if epic[key] != value {
				t.Errorf("expected %s=%v, got %v", key, value, epic[key])
			}
		}
	})

	t.Run("requires a name", func(t *testing.T) {
		setupEpicTest(t)

		epicCreateBoard = "123"
		err := epicCreateCmd.RunE(epicCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})

	t.Run("rejects an unknown status", func(t *testing.T) {
		mock := setupEpicTest(t)

		epicCreateBoard = "123"
		epicCreateAttrs = epicAttrs{name: "X", status: "done"}
		err := epicCreateCmd.RunE(epicCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PostCalls))
		}
	})

	t.Run("rejects an unknown color", func(t *testing.T) {
		setupEpicTest(t)

		epicCreateBoard = "123"
		epicCreateAttrs = epicAttrs{name: "X", color: "orange"}
		err := epicCreateCmd.RunE(epicCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}

func TestEpicUpdate(t *testing.T) {
	t.Run("takes the epic out of its wave", func(t *testing.T) {
		mock := setupEpicTest(t)

		epicUpdateBoard = "123"
		epicUpdateAttrs = epicAttrs{noWave: true}
		err := epicUpdateCmd.RunE(epicUpdateCmd, []string{"e1"})

		assertExitCode(t, err, 0)
		if mock.PatchCalls[0].Path != "/boards/123/epics/e1.json" {
			t.Errorf("expected epic path, got %q", mock.PatchCalls[0].Path)
		}
		epic := mock.PatchCalls[0].Body.(map[string]any)["epic"].(map[string]any)
		if value, ok := epic["wave_id"]; !ok || value != nil {
			t.Errorf("expected wave_id null, got %v (present=%v)", value, ok)
		}
	})

	t.Run("rejects --wave together with --no-wave", func(t *testing.T) {
		setupEpicTest(t)

		epicUpdateBoard = "123"
		epicUpdateAttrs = epicAttrs{wave: "w1", noWave: true}
		err := epicUpdateCmd.RunE(epicUpdateCmd, []string{"e1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})

	t.Run("requires something to update", func(t *testing.T) {
		mock := setupEpicTest(t)

		epicUpdateBoard = "123"
		err := epicUpdateCmd.RunE(epicUpdateCmd, []string{"e1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PatchCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PatchCalls))
		}
	})
}

func TestEpicDelete(t *testing.T) {
	mock := setupEpicTest(t)

	epicDeleteBoard = "123"
	err := epicDeleteCmd.RunE(epicDeleteCmd, []string{"e1"})

	assertExitCode(t, err, 0)
	if mock.DeleteCalls[0].Path != "/boards/123/epics/e1.json" {
		t.Errorf("expected epic path, got %q", mock.DeleteCalls[0].Path)
	}
}

func TestEpicCards(t *testing.T) {
	mock := setupEpicTest(t)
	mock.OnGet("/boards/123/epics/e1/cards.json?page=2", &client.APIResponse{StatusCode: 200, Data: []any{map[string]any{"number": 7, "title": "Card"}}})

	epicCardsBoard = "123"
	epicCardsPage = 2
	err := epicCardsCmd.RunE(epicCardsCmd, []string{"e1"})

	assertExitCode(t, err, 0)
	if mock.GetCalls[0].Path != "/boards/123/epics/e1/cards.json?page=2" {
		t.Errorf("expected epic cards path, got %q", mock.GetCalls[0].Path)
	}
}

func TestEpicAddCard(t *testing.T) {
	t.Run("sets the card's epic", func(t *testing.T) {
		mock := setupEpicTest(t)

		epicAddCardBoard = "123"
		epicAddCardNumber = "42"
		err := epicAddCardCmd.RunE(epicAddCardCmd, []string{"e1"})

		assertExitCode(t, err, 0)
		if mock.PatchCalls[0].Path != "/cards/42.json" {
			t.Errorf("expected card path, got %q", mock.PatchCalls[0].Path)
		}
		card := mock.PatchCalls[0].Body.(map[string]any)["card"].(map[string]any)
		if card["epic_id"] != "e1" {
			t.Errorf("expected epic_id e1, got %v", card["epic_id"])
		}
	})

	t.Run("requires a card", func(t *testing.T) {
		setupEpicTest(t)

		epicAddCardBoard = "123"
		err := epicAddCardCmd.RunE(epicAddCardCmd, []string{"e1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}

func TestEpicRemoveCard(t *testing.T) {
	mock := setupEpicTest(t)

	epicRemoveCardNumber = "42"
	err := epicRemoveCardCmd.RunE(epicRemoveCardCmd, []string{})

	assertExitCode(t, err, 0)
	card := mock.PatchCalls[0].Body.(map[string]any)["card"].(map[string]any)
	if card["epic_id"] != "" {
		t.Errorf("expected epic_id cleared with an empty string, got %v", card["epic_id"])
	}
}
