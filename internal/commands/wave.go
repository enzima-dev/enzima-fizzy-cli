package commands

import (
	"fmt"
	"slices"

	"github.com/basecamp/fizzy-cli/internal/errors"
	"github.com/spf13/cobra"
)

var waveCmd = &cobra.Command{
	Use:   "wave",
	Short: "Manage waves",
	Long:  "Commands for managing board waves: overlapping development cycles that group epics.",
}

// Lifecycle order matters: --next walks this list.
var wavePhases = []string{"discovery", "design", "prototype", "validation", "setup", "evolution"}

func wavesPath(boardID string) string {
	return fmt.Sprintf("/boards/%s/waves", boardID)
}

func wavePath(boardID, waveID string) string {
	return fmt.Sprintf("/boards/%s/waves/%s", boardID, waveID)
}

// Wave list flags
var waveListBoard string

var waveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List waves for a board",
	Long:  "Lists the waves of a board, ordered by start date.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(waveListBoard)
		if err != nil {
			return err
		}

		resp, err := getSDK().Get(cmd.Context(), wavesPath(boardID)+".json")
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(resp.Data)
		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy wave show <id> --board %s", boardID), "View wave"),
			breadcrumb("create", fmt.Sprintf("fizzy wave create --board %s --name \"name\"", boardID), "Create wave"),
			breadcrumb("epics", fmt.Sprintf("fizzy epic list --board %s", boardID), "List epics"),
		}

		printList(items, waveColumns, fmt.Sprintf("%d waves", dataCount(items)), breadcrumbs)
		return nil
	},
}

// Wave show flags
var waveShowBoard string

var waveShowCmd = &cobra.Command{
	Use:   "show WAVE_ID",
	Short: "Show a wave",
	Long:  "Shows a wave with its phase, dates, progress and epic IDs.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(waveShowBoard)
		if err != nil {
			return err
		}

		waveID := args[0]
		resp, err := getSDK().Get(cmd.Context(), wavePath(boardID, waveID)+".json")
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("phase", fmt.Sprintf("fizzy wave phase %s --board %s --next", waveID, boardID), "Advance phase"),
			breadcrumb("update", fmt.Sprintf("fizzy wave update %s --board %s", waveID, boardID), "Update wave"),
			breadcrumb("waves", fmt.Sprintf("fizzy wave list --board %s", boardID), "List waves"),
		}

		printDetail(normalizeAny(resp.Data), "", breadcrumbs)
		return nil
	},
}

// Shared wave create/update attribute flags
type waveAttrs struct {
	name, description, color, owner, start, target, phase string
	epics                                                 []string
	noEpics                                               bool
}

func (a waveAttrs) body() (map[string]any, error) {
	body := map[string]any{}

	if a.name != "" {
		body["name"] = a.name
	}
	if a.description != "" {
		body["description"] = a.description
	}
	if a.owner != "" {
		body["owner_id"] = a.owner
	}
	if a.start != "" {
		body["start_date"] = a.start
	}
	if a.target != "" {
		body["target_date"] = a.target
	}

	color, err := normalizeColumnColor(a.color)
	if err != nil {
		return nil, err
	}
	if color != "" {
		body["color"] = color
	}

	if a.phase != "" {
		if err := validateWavePhase(a.phase, "--phase"); err != nil {
			return nil, err
		}
		body["phase"] = a.phase
	}

	if len(a.epics) > 0 && a.noEpics {
		return nil, errors.NewInvalidArgsError("--epic and --no-epics are mutually exclusive")
	}
	if len(a.epics) > 0 {
		body["epic_ids"] = a.epics
	} else if a.noEpics {
		body["epic_ids"] = []string{}
	}

	return body, nil
}

func validateWavePhase(phase, flag string) error {
	if slices.Contains(wavePhases, phase) {
		return nil
	}
	return errors.NewInvalidArgsError(fmt.Sprintf("%s must be one of: %v (got %q)", flag, wavePhases, phase))
}

// Wave create flags
var waveCreateBoard string
var waveCreateAttrs waveAttrs

var waveCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a wave",
	Long:  "Creates a wave on a board. Repeat --epic to put existing epics in it.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(waveCreateBoard)
		if err != nil {
			return err
		}
		if waveCreateAttrs.name == "" {
			return newRequiredFlagError("name")
		}

		body, err := waveCreateAttrs.body()
		if err != nil {
			return err
		}

		resp, err := getSDK().Post(cmd.Context(), wavesPath(boardID)+".json", map[string]any{"wave": body})
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(resp.Data)
		var breadcrumbs []Breadcrumb
		if waveID := idFrom(items); waveID != "" {
			breadcrumbs = []Breadcrumb{
				breadcrumb("show", fmt.Sprintf("fizzy wave show %s --board %s", waveID, boardID), "View wave"),
				breadcrumb("epic", fmt.Sprintf("fizzy epic create --board %s --wave %s --name \"name\"", boardID, waveID), "Create an epic in this wave"),
			}
		}

		if location := resp.Headers.Get("Location"); location != "" {
			printMutationWithLocation(items, location, breadcrumbs)
		} else {
			printMutation(items, "", breadcrumbs)
		}
		return nil
	},
}

// Wave update flags
var waveUpdateBoard string
var waveUpdateAttrs waveAttrs

var waveUpdateCmd = &cobra.Command{
	Use:   "update WAVE_ID",
	Short: "Update a wave",
	Long:  "Updates a wave. --epic replaces the wave's epics with the given ones; --no-epics empties it.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(waveUpdateBoard)
		if err != nil {
			return err
		}

		body, err := waveUpdateAttrs.body()
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return errors.NewInvalidArgsError("nothing to update: pass at least one attribute flag")
		}

		waveID := args[0]
		resp, err := getSDK().Patch(cmd.Context(), wavePath(boardID, waveID)+".json", map[string]any{"wave": body})
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy wave show %s --board %s", waveID, boardID), "View wave"),
			breadcrumb("waves", fmt.Sprintf("fizzy wave list --board %s", boardID), "List waves"),
		}

		result := normalizeAny(resp.Data)
		if result == nil {
			result = map[string]any{}
		}
		printMutation(result, "", breadcrumbs)
		return nil
	},
}

// Wave delete flags
var waveDeleteBoard string

var waveDeleteCmd = &cobra.Command{
	Use:   "delete WAVE_ID",
	Short: "Delete a wave",
	Long:  "Deletes a wave. Its epics stay on the board, just without a wave.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(waveDeleteBoard)
		if err != nil {
			return err
		}

		if _, err := getSDK().Delete(cmd.Context(), wavePath(boardID, args[0])+".json"); err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("waves", fmt.Sprintf("fizzy wave list --board %s", boardID), "List waves"),
		}
		printMutation(map[string]any{"deleted": true}, "", breadcrumbs)
		return nil
	},
}

// Wave phase flags
var wavePhaseBoard string
var wavePhaseSet string
var wavePhaseNext bool

var wavePhaseCmd = &cobra.Command{
	Use:   "phase WAVE_ID",
	Short: "Change a wave's phase",
	Long:  "Sets a wave's phase (--set) or moves it to the next one in the lifecycle (--next).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if (wavePhaseSet == "") == !wavePhaseNext {
			return errors.NewInvalidArgsError("pass exactly one of --set PHASE or --next")
		}

		boardID, err := requireBoard(wavePhaseBoard)
		if err != nil {
			return err
		}

		waveID := args[0]
		phase := wavePhaseSet
		if wavePhaseNext {
			phase, err = nextWavePhase(cmd, boardID, waveID)
			if err != nil {
				return err
			}
		} else if err = validateWavePhase(phase, "--set"); err != nil {
			return err
		}

		resp, err := getSDK().Patch(cmd.Context(), wavePath(boardID, waveID)+"/phase.json", map[string]any{"phase": phase})
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy wave show %s --board %s", waveID, boardID), "View wave"),
			breadcrumb("next", fmt.Sprintf("fizzy wave phase %s --board %s --next", waveID, boardID), "Advance again"),
		}

		result := normalizeAny(resp.Data)
		if result == nil {
			result = map[string]any{"phase": phase}
		}
		printMutation(result, "", breadcrumbs)
		return nil
	},
}

func nextWavePhase(cmd *cobra.Command, boardID, waveID string) (string, error) {
	resp, err := getSDK().Get(cmd.Context(), wavePath(boardID, waveID)+".json")
	if err != nil {
		return "", convertSDKError(err)
	}

	current := ""
	if wave, ok := normalizeAny(resp.Data).(map[string]any); ok {
		current, _ = wave["phase"].(string)
	}

	for i, phase := range wavePhases {
		if phase == current {
			if i+1 < len(wavePhases) {
				return wavePhases[i+1], nil
			}
			return "", errors.NewInvalidArgsError(fmt.Sprintf("wave is already in the last phase (%s)", current))
		}
	}
	return "", errors.NewInvalidArgsError(fmt.Sprintf("unknown current phase %q", current))
}

func addWaveAttrFlags(cmd *cobra.Command, attrs *waveAttrs, nameUsage string) {
	cmd.Flags().StringVar(&attrs.name, "name", "", nameUsage)
	cmd.Flags().StringVar(&attrs.description, "description", "", "Wave goal / description")
	cmd.Flags().StringVar(&attrs.color, "color", "", "Wave color ("+columnColorNamesHelp+")")
	cmd.Flags().StringVar(&attrs.owner, "owner", "", "Owner user ID")
	cmd.Flags().StringVar(&attrs.start, "start", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&attrs.target, "target", "", "Target date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&attrs.phase, "phase", "", "Phase (discovery, design, prototype, validation, setup, evolution)")
	cmd.Flags().StringSliceVar(&attrs.epics, "epic", nil, "Epic ID to put in the wave (repeatable or comma-separated)")
	cmd.Flags().BoolVar(&attrs.noEpics, "no-epics", false, "Remove all epics from the wave")
}

func init() {
	rootCmd.AddCommand(waveCmd)

	waveListCmd.Flags().StringVar(&waveListBoard, "board", "", "Board ID (required)")
	waveCmd.AddCommand(waveListCmd)

	waveShowCmd.Flags().StringVar(&waveShowBoard, "board", "", "Board ID (required)")
	waveCmd.AddCommand(waveShowCmd)

	waveCreateCmd.Flags().StringVar(&waveCreateBoard, "board", "", "Board ID (required)")
	addWaveAttrFlags(waveCreateCmd, &waveCreateAttrs, "Wave name (required)")
	waveCmd.AddCommand(waveCreateCmd)

	waveUpdateCmd.Flags().StringVar(&waveUpdateBoard, "board", "", "Board ID (required)")
	addWaveAttrFlags(waveUpdateCmd, &waveUpdateAttrs, "Wave name")
	waveCmd.AddCommand(waveUpdateCmd)

	waveDeleteCmd.Flags().StringVar(&waveDeleteBoard, "board", "", "Board ID (required)")
	waveCmd.AddCommand(waveDeleteCmd)

	wavePhaseCmd.Flags().StringVar(&wavePhaseBoard, "board", "", "Board ID (required)")
	wavePhaseCmd.Flags().StringVar(&wavePhaseSet, "set", "", "Phase to set")
	wavePhaseCmd.Flags().BoolVar(&wavePhaseNext, "next", false, "Move to the next phase")
	waveCmd.AddCommand(wavePhaseCmd)
}
