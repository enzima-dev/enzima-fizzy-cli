package commands

import (
	"testing"

	"github.com/basecamp/fizzy-cli/internal/client"
	"github.com/basecamp/fizzy-cli/internal/errors"
)

func setupWaveTest(t *testing.T) *MockClient {
	t.Helper()
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	SetTestConfig("token", "account", "https://api.example.com")
	t.Cleanup(func() {
		resetTest()
		waveListBoard, waveShowBoard, waveCreateBoard, waveUpdateBoard, waveDeleteBoard = "", "", "", "", ""
		wavePhaseBoard, wavePhaseSet, wavePhaseNext = "", "", false
		waveCreateAttrs, waveUpdateAttrs = waveAttrs{}, waveAttrs{}
	})
	return mock
}

func TestWaveList(t *testing.T) {
	t.Run("lists the board waves", func(t *testing.T) {
		mock := setupWaveTest(t)
		mock.OnGet("/boards/123/waves.json", &client.APIResponse{
			StatusCode: 200,
			Data:       []any{map[string]any{"id": "w1", "name": "Matrícula online", "phase": "validation"}},
		})

		waveListBoard = "123"
		err := waveListCmd.RunE(waveListCmd, []string{})

		assertExitCode(t, err, 0)
		if mock.GetCalls[0].Path != "/boards/123/waves.json" {
			t.Errorf("expected waves path, got %q", mock.GetCalls[0].Path)
		}
	})

	t.Run("requires a board", func(t *testing.T) {
		setupWaveTest(t)

		err := waveListCmd.RunE(waveListCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}

func TestWaveShow(t *testing.T) {
	mock := setupWaveTest(t)
	mock.OnGet("/boards/123/waves/w1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "w1"}})

	waveShowBoard = "123"
	err := waveShowCmd.RunE(waveShowCmd, []string{"w1"})

	assertExitCode(t, err, 0)
	if mock.GetCalls[0].Path != "/boards/123/waves/w1.json" {
		t.Errorf("expected wave path, got %q", mock.GetCalls[0].Path)
	}
}

func TestWaveCreate(t *testing.T) {
	t.Run("posts the nested wave attributes with its epics", func(t *testing.T) {
		mock := setupWaveTest(t)
		mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]any{"id": "w9"}}

		waveCreateBoard = "123"
		waveCreateAttrs = waveAttrs{name: "Rematrícula", phase: "design", color: "pink", start: "2026-09-14", target: "2026-12-18", epics: []string{"e1", "e2"}}
		err := waveCreateCmd.RunE(waveCreateCmd, []string{})

		assertExitCode(t, err, 0)
		if mock.PostCalls[0].Path != "/boards/123/waves.json" {
			t.Errorf("expected waves path, got %q", mock.PostCalls[0].Path)
		}
		wave := mock.PostCalls[0].Body.(map[string]any)["wave"].(map[string]any)
		if wave["name"] != "Rematrícula" || wave["phase"] != "design" || wave["color"] != "var(--color-card-8)" {
			t.Errorf("unexpected wave attributes: %v", wave)
		}
		if wave["start_date"] != "2026-09-14" || wave["target_date"] != "2026-12-18" {
			t.Errorf("unexpected dates: %v", wave)
		}
		epics := wave["epic_ids"].([]any)
		if len(epics) != 2 || epics[0] != "e1" || epics[1] != "e2" {
			t.Errorf("expected epic_ids [e1 e2], got %v", epics)
		}
	})

	t.Run("requires a name", func(t *testing.T) {
		setupWaveTest(t)

		waveCreateBoard = "123"
		err := waveCreateCmd.RunE(waveCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})

	t.Run("rejects an unknown phase", func(t *testing.T) {
		mock := setupWaveTest(t)

		waveCreateBoard = "123"
		waveCreateAttrs = waveAttrs{name: "X", phase: "launch"}
		err := waveCreateCmd.RunE(waveCreateCmd, []string{})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PostCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PostCalls))
		}
	})
}

func TestWaveUpdate(t *testing.T) {
	t.Run("empties the wave with --no-epics", func(t *testing.T) {
		mock := setupWaveTest(t)

		waveUpdateBoard = "123"
		waveUpdateAttrs = waveAttrs{noEpics: true}
		err := waveUpdateCmd.RunE(waveUpdateCmd, []string{"w1"})

		assertExitCode(t, err, 0)
		if mock.PatchCalls[0].Path != "/boards/123/waves/w1.json" {
			t.Errorf("expected wave path, got %q", mock.PatchCalls[0].Path)
		}
		wave := mock.PatchCalls[0].Body.(map[string]any)["wave"].(map[string]any)
		if epics, ok := wave["epic_ids"].([]any); !ok || len(epics) != 0 {
			t.Errorf("expected empty epic_ids, got %v", wave["epic_ids"])
		}
	})

	t.Run("rejects --epic together with --no-epics", func(t *testing.T) {
		setupWaveTest(t)

		waveUpdateBoard = "123"
		waveUpdateAttrs = waveAttrs{epics: []string{"e1"}, noEpics: true}
		err := waveUpdateCmd.RunE(waveUpdateCmd, []string{"w1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})

	t.Run("requires something to update", func(t *testing.T) {
		setupWaveTest(t)

		waveUpdateBoard = "123"
		err := waveUpdateCmd.RunE(waveUpdateCmd, []string{"w1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}

func TestWaveDelete(t *testing.T) {
	mock := setupWaveTest(t)

	waveDeleteBoard = "123"
	err := waveDeleteCmd.RunE(waveDeleteCmd, []string{"w1"})

	assertExitCode(t, err, 0)
	if mock.DeleteCalls[0].Path != "/boards/123/waves/w1.json" {
		t.Errorf("expected wave path, got %q", mock.DeleteCalls[0].Path)
	}
}

func TestWavePhase(t *testing.T) {
	t.Run("sets an explicit phase", func(t *testing.T) {
		mock := setupWaveTest(t)

		wavePhaseBoard = "123"
		wavePhaseSet = "setup"
		err := wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})

		assertExitCode(t, err, 0)
		if mock.PatchCalls[0].Path != "/boards/123/waves/w1/phase.json" {
			t.Errorf("expected phase path, got %q", mock.PatchCalls[0].Path)
		}
		if body := mock.PatchCalls[0].Body.(map[string]any); body["phase"] != "setup" {
			t.Errorf("expected phase setup, got %v", body["phase"])
		}
	})

	t.Run("moves to the next phase", func(t *testing.T) {
		mock := setupWaveTest(t)
		mock.OnGet("/boards/123/waves/w1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "w1", "phase": "validation"}})

		wavePhaseBoard = "123"
		wavePhaseNext = true
		err := wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})

		assertExitCode(t, err, 0)
		if body := mock.PatchCalls[0].Body.(map[string]any); body["phase"] != "setup" {
			t.Errorf("expected phase setup after validation, got %v", body["phase"])
		}
	})

	t.Run("refuses --next past the last phase", func(t *testing.T) {
		mock := setupWaveTest(t)
		mock.OnGet("/boards/123/waves/w1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "w1", "phase": "evolution"}})

		wavePhaseBoard = "123"
		wavePhaseNext = true
		err := wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
		if len(mock.PatchCalls) != 0 {
			t.Errorf("expected no request, got %d", len(mock.PatchCalls))
		}
	})

	t.Run("requires exactly one of --set or --next", func(t *testing.T) {
		setupWaveTest(t)

		wavePhaseBoard = "123"
		err := wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})
		assertExitCode(t, err, errors.ExitInvalidArgs)

		wavePhaseSet, wavePhaseNext = "setup", true
		err = wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})
		assertExitCode(t, err, errors.ExitInvalidArgs)
	})

	t.Run("rejects an unknown phase", func(t *testing.T) {
		setupWaveTest(t)

		wavePhaseBoard = "123"
		wavePhaseSet = "launch"
		err := wavePhaseCmd.RunE(wavePhaseCmd, []string{"w1"})

		assertExitCode(t, err, errors.ExitInvalidArgs)
	})
}
