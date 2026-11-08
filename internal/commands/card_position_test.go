package commands

import (
	"testing"

	"github.com/basecamp/fizzy-cli/internal/client"
	"github.com/basecamp/fizzy-cli/internal/errors"
)

func setupCardPositionTest(t *testing.T) (*MockClient, *CommandResult) {
	t.Helper()
	mock := NewMockClient()
	result := SetTestModeWithSDK(mock)
	SetTestConfig("token", "account", "https://api.example.com")
	t.Cleanup(func() {
		resetTest()
		cardPositionFlags.reset()
		cardColumnPlacement.reset()
		cardColumnColumn = ""
		cardListBoard, cardListColumn, cardListTag, cardListPage = "", "", "", 0
	})
	return mock, result
}

func TestCardPosition(t *testing.T) {
	cases := []struct {
		name      string
		placement cardPlacement
		body      map[string]any
	}{
		{"top", cardPlacement{top: true}, map[string]any{"placement": "top"}},
		{"bottom", cardPlacement{bottom: true}, map[string]any{"placement": "bottom"}},
		{"before", cardPlacement{before: "1579"}, map[string]any{"before_card_id": "1579"}},
		{"after", cardPlacement{after: "1579"}, map[string]any{"after_card_id": "1579"}},
	}

	for _, tc := range cases {
		t.Run("places the card "+tc.name, func(t *testing.T) {
			mock, result := setupCardPositionTest(t)
			mock.PostResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{"number": 1580, "position": 1.5}}

			cardPositionFlags = tc.placement
			err := cardPositionCmd.RunE(cardPositionCmd, []string{"1580"})

			assertExitCode(t, err, 0)
			if len(mock.PostCalls) != 1 || mock.PostCalls[0].Path != "/cards/1580/position.json" {
				t.Fatalf("expected post to '/cards/1580/position.json', got %+v", mock.PostCalls)
			}
			body := mock.PostCalls[0].Body.(map[string]any)
			if len(body) != len(tc.body) {
				t.Errorf("expected body %v, got %v", tc.body, body)
			}
			for key, value := range tc.body {
				if body[key] != value {
					t.Errorf("expected %s=%v, got %v", key, value, body[key])
				}
			}
			card := result.Response.Data.(map[string]any)
			if card["position"] != 1.5 {
				t.Errorf("expected the positioned card in the response, got %v", card)
			}
		})
	}

	t.Run("requires a placement", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)

		err := cardPositionCmd.RunE(cardPositionCmd, []string{"1580"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PostCalls))
		}
	})

	t.Run("rejects more than one placement", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)

		cardPositionFlags = cardPlacement{top: true, after: "1579"}
		err := cardPositionCmd.RunE(cardPositionCmd, []string{"1580"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PostCalls))
		}
	})

	t.Run("surfaces the API error", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.PostError = errors.NewValidationError("O card #1579 não está na mesma coluna do card #1580")

		cardPositionFlags = cardPlacement{after: "1579"}
		err := cardPositionCmd.RunE(cardPositionCmd, []string{"1580"})

		assertExitCode(t, err, errors.ExitValidation)
	})
}

func TestCardColumnPlacement(t *testing.T) {
	t.Run("moves the card and places it in one call", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{"number": 1580}}

		cardColumnColumn = "col-1"
		cardColumnPlacement = cardPlacement{after: "1579"}
		err := cardColumnCmd.RunE(cardColumnCmd, []string{"1580"})

		assertExitCode(t, err, 0)
		if len(mock.PostCalls) != 2 {
			t.Fatalf("expected triage + position, got %+v", mock.PostCalls)
		}
		if mock.PostCalls[0].Path != "/cards/1580/triage.json" || mock.PostCalls[1].Path != "/cards/1580/position.json" {
			t.Errorf("unexpected calls: %+v", mock.PostCalls)
		}
		if mock.PostCalls[1].Body.(map[string]any)["after_card_id"] != "1579" {
			t.Errorf("unexpected position body: %v", mock.PostCalls[1].Body)
		}
	})

	t.Run("skips positioning without placement flags", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{}}

		cardColumnColumn = "col-1"
		err := cardColumnCmd.RunE(cardColumnCmd, []string{"1580"})

		assertExitCode(t, err, 0)
		if len(mock.PostCalls) != 1 {
			t.Errorf("expected only the triage call, got %+v", mock.PostCalls)
		}
	})

	t.Run("rejects placement on a pseudo column before any request", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)

		cardColumnColumn = "not-now"
		cardColumnPlacement = cardPlacement{top: true}
		err := cardColumnCmd.RunE(cardColumnCmd, []string{"1580"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %+v", mock.PostCalls)
		}
	})

	t.Run("rejects more than one placement before any request", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)

		cardColumnColumn = "col-1"
		cardColumnPlacement = cardPlacement{top: true, bottom: true}
		err := cardColumnCmd.RunE(cardColumnCmd, []string{"1580"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %+v", mock.PostCalls)
		}
	})
}

func TestCardListBoardOrder(t *testing.T) {
	t.Run("lists a column in board order when the board is known", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.GetWithPaginationResponse = &client.APIResponse{StatusCode: 200, Data: []any{}}

		cardListBoard, cardListColumn, cardListPage = "board-1", "col-1", 2
		err := cardListCmd.RunE(cardListCmd, []string{})

		assertExitCode(t, err, 0)
		if path := mock.GetWithPaginationCalls[0].Path; path != "/boards/board-1/columns/col-1/cards.json?page=2" {
			t.Errorf("expected the board column endpoint, got '%s'", path)
		}
	})

	t.Run("uses the default board", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.GetWithPaginationResponse = &client.APIResponse{StatusCode: 200, Data: []any{}}
		cfg.Board = "board-1"

		cardListColumn = "col-1"
		err := cardListCmd.RunE(cardListCmd, []string{})

		assertExitCode(t, err, 0)
		if path := mock.GetWithPaginationCalls[0].Path; path != "/boards/board-1/columns/col-1/cards.json" {
			t.Errorf("expected the board column endpoint, got '%s'", path)
		}
	})

	t.Run("falls back to the filtered search with extra filters", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.GetWithPaginationResponse = &client.APIResponse{StatusCode: 200, Data: []any{}}

		cardListBoard, cardListColumn, cardListTag = "board-1", "col-1", "tag-1"
		err := cardListCmd.RunE(cardListCmd, []string{})

		assertExitCode(t, err, 0)
		if path := mock.GetWithPaginationCalls[0].Path; path != "/cards.json?board_ids[]=board-1&column_ids[]=col-1&tag_ids[]=tag-1" {
			t.Errorf("expected the filtered search, got '%s'", path)
		}
	})

	t.Run("keeps pseudo columns on the filtered search", func(t *testing.T) {
		mock, _ := setupCardPositionTest(t)
		mock.GetWithPaginationResponse = &client.APIResponse{StatusCode: 200, Data: []any{}}

		cardListBoard, cardListColumn = "board-1", "done"
		err := cardListCmd.RunE(cardListCmd, []string{})

		assertExitCode(t, err, 0)
		if path := mock.GetWithPaginationCalls[0].Path; path != "/cards.json?board_ids[]=board-1&indexed_by=closed" {
			t.Errorf("expected the filtered search, got '%s'", path)
		}
	})
}
