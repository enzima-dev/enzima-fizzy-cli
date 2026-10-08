package commands

import (
	"testing"

	"github.com/basecamp/fizzy-cli/internal/client"
	"github.com/basecamp/fizzy-cli/internal/errors"
)

func setupCardClassificationTest(t *testing.T) *MockClient {
	t.Helper()
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	SetTestConfig("token", "account", "https://api.example.com")
	t.Cleanup(func() {
		resetTest()
		cardCreateBoard, cardCreateTitle, cardCreateType, cardCreateEpic = "", "", "", ""
		cardUpdateTitle, cardUpdateType, cardUpdateEpic = "", "", ""
	})
	return mock
}

func TestCardCreateClassification(t *testing.T) {
	t.Run("sets the type and epic after creating the card", func(t *testing.T) {
		mock := setupCardClassificationTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]any{"number": 42, "title": "New"}}
		mock.PatchResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{"number": 42, "card_type": "bug", "epic_id": "e1"}}

		cardCreateBoard, cardCreateTitle, cardCreateType, cardCreateEpic = "123", "New", "bug", "e1"
		err := cardCreateCmd.RunE(cardCreateCmd, []string{})

		assertExitCode(t, err, 0)
		if mock.PatchCalls[0].Path != "/cards/42.json" {
			t.Errorf("expected card path, got %q", mock.PatchCalls[0].Path)
		}
		card := mock.PatchCalls[0].Body.(map[string]any)["card"].(map[string]any)
		if card["card_type"] != "bug" || card["epic_id"] != "e1" {
			t.Errorf("unexpected classification: %v", card)
		}
	})

	t.Run("skips the extra request without type or epic", func(t *testing.T) {
		mock := setupCardClassificationTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]any{"number": 42}}

		cardCreateBoard, cardCreateTitle = "123", "New"
		err := cardCreateCmd.RunE(cardCreateCmd, []string{})

		assertExitCode(t, err, 0)
		if len(mock.PatchCalls) != 0 {
			t.Errorf("expected no patch, got %d", len(mock.PatchCalls))
		}
	})

	t.Run("rejects an unknown type before creating", func(t *testing.T) {
		mock := setupCardClassificationTest(t)

		cardCreateBoard, cardCreateTitle, cardCreateType = "123", "New", "story"
		err := cardCreateCmd.RunE(cardCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PostCalls))
		}
	})
}

func TestCardUpdateClassification(t *testing.T) {
	t.Run("only patches the classification when nothing else changes", func(t *testing.T) {
		mock := setupCardClassificationTest(t)
		mock.PatchResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{"number": 42, "card_type": "chore"}}

		cardUpdateType = "chore"
		err := cardUpdateCmd.RunE(cardUpdateCmd, []string{"42"})

		assertExitCode(t, err, 0)
		if len(mock.PatchCalls) != 1 || mock.PatchCalls[0].Path != "/cards/42.json" {
			t.Fatalf("expected a single classification patch, got %v", mock.PatchCalls)
		}
		card := mock.PatchCalls[0].Body.(map[string]any)["card"].(map[string]any)
		if card["card_type"] != "chore" {
			t.Errorf("expected card_type chore, got %v", card["card_type"])
		}
	})

	t.Run("updates content and classification together", func(t *testing.T) {
		mock := setupCardClassificationTest(t)
		mock.PatchResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{"number": 42}}

		cardUpdateTitle, cardUpdateEpic = "Renamed", "e2"
		err := cardUpdateCmd.RunE(cardUpdateCmd, []string{"42"})

		assertExitCode(t, err, 0)
		if len(mock.PatchCalls) != 2 {
			t.Fatalf("expected content and classification patches, got %d", len(mock.PatchCalls))
		}
		card := mock.PatchCalls[1].Body.(map[string]any)["card"].(map[string]any)
		if card["epic_id"] != "e2" {
			t.Errorf("expected epic_id e2, got %v", card["epic_id"])
		}
	})

	t.Run("rejects an unknown type", func(t *testing.T) {
		mock := setupCardClassificationTest(t)

		cardUpdateType = "story"
		err := cardUpdateCmd.RunE(cardUpdateCmd, []string{"42"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PatchCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PatchCalls))
		}
	})
}
