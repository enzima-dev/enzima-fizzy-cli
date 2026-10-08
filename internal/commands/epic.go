package commands

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/basecamp/fizzy-cli/internal/errors"
	"github.com/spf13/cobra"
)

// Epics and waves are Enzima-fork resources; the SDK has no generated
// services for them, so these commands use the account client's raw verbs.

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage epics",
	Long:  "Commands for managing board epics (groups of cards toward a bigger outcome).",
}

var epicStatuses = []string{"not_started", "in_progress", "completed"}

func epicsPath(boardID string) string {
	return fmt.Sprintf("/boards/%s/epics", boardID)
}

func epicPath(boardID, epicID string) string {
	return fmt.Sprintf("/boards/%s/epics/%s", boardID, epicID)
}

// Epic list flags
var epicListBoard string

var epicListCmd = &cobra.Command{
	Use:   "list",
	Short: "List epics for a board",
	Long:  "Lists all epics of a board, in board order.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(epicListBoard)
		if err != nil {
			return err
		}

		resp, err := getSDK().Get(cmd.Context(), epicsPath(boardID)+".json")
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(resp.Data)
		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy epic show <id> --board %s", boardID), "View epic"),
			breadcrumb("create", fmt.Sprintf("fizzy epic create --board %s --name \"name\"", boardID), "Create epic"),
			breadcrumb("waves", fmt.Sprintf("fizzy wave list --board %s", boardID), "List waves"),
		}

		printList(items, epicColumns, fmt.Sprintf("%d epics", dataCount(items)), breadcrumbs)
		return nil
	},
}

// Epic show flags
var epicShowBoard string

var epicShowCmd = &cobra.Command{
	Use:   "show EPIC_ID",
	Short: "Show an epic",
	Long:  "Shows an epic with its progress, dates, owner and wave.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(epicShowBoard)
		if err != nil {
			return err
		}

		epicID := args[0]
		resp, err := getSDK().Get(cmd.Context(), epicPath(boardID, epicID)+".json")
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("cards", fmt.Sprintf("fizzy epic cards %s --board %s", epicID, boardID), "List epic cards"),
			breadcrumb("update", fmt.Sprintf("fizzy epic update %s --board %s", epicID, boardID), "Update epic"),
			breadcrumb("epics", fmt.Sprintf("fizzy epic list --board %s", boardID), "List epics"),
		}

		printDetail(normalizeAny(resp.Data), "", breadcrumbs)
		return nil
	},
}

// Shared epic create/update attribute flags
type epicAttrs struct {
	name, description, color, owner, start, target, status, wave string
	noWave                                                       bool
}

func (a epicAttrs) body() (map[string]any, error) {
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

	if a.status != "" {
		if !slices.Contains(epicStatuses, a.status) {
			return nil, errors.NewInvalidArgsError(fmt.Sprintf("--status must be one of: %v (got %q)", epicStatuses, a.status))
		}
		body["status"] = a.status
	}

	if a.wave != "" && a.noWave {
		return nil, errors.NewInvalidArgsError("--wave and --no-wave are mutually exclusive")
	}
	if a.wave != "" {
		body["wave_id"] = a.wave
	} else if a.noWave {
		body["wave_id"] = nil
	}

	return body, nil
}

func idFrom(item any) string {
	if m, ok := item.(map[string]any); ok {
		if id, ok := m["id"]; ok && id != nil {
			return fmt.Sprintf("%v", id)
		}
	}
	return ""
}

// Epic create flags
var epicCreateBoard string
var epicCreateAttrs epicAttrs

var epicCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an epic",
	Long:  "Creates a new epic on a board, optionally inside a wave.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(epicCreateBoard)
		if err != nil {
			return err
		}
		if epicCreateAttrs.name == "" {
			return newRequiredFlagError("name")
		}

		body, err := epicCreateAttrs.body()
		if err != nil {
			return err
		}

		resp, err := getSDK().Post(cmd.Context(), epicsPath(boardID)+".json", map[string]any{"epic": body})
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(resp.Data)
		var breadcrumbs []Breadcrumb
		if epicID := idFrom(items); epicID != "" {
			breadcrumbs = []Breadcrumb{
				breadcrumb("show", fmt.Sprintf("fizzy epic show %s --board %s", epicID, boardID), "View epic"),
				breadcrumb("add-card", fmt.Sprintf("fizzy epic add-card %s --board %s --card <number>", epicID, boardID), "Add a card to the epic"),
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

// Epic update flags
var epicUpdateBoard string
var epicUpdateAttrs epicAttrs

var epicUpdateCmd = &cobra.Command{
	Use:   "update EPIC_ID",
	Short: "Update an epic",
	Long:  "Updates an epic. Use --wave to move it into a wave or --no-wave to take it out.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(epicUpdateBoard)
		if err != nil {
			return err
		}

		body, err := epicUpdateAttrs.body()
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return errors.NewInvalidArgsError("nothing to update: pass at least one attribute flag")
		}

		epicID := args[0]
		resp, err := getSDK().Patch(cmd.Context(), epicPath(boardID, epicID)+".json", map[string]any{"epic": body})
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy epic show %s --board %s", epicID, boardID), "View epic"),
			breadcrumb("epics", fmt.Sprintf("fizzy epic list --board %s", boardID), "List epics"),
		}

		result := normalizeAny(resp.Data)
		if result == nil {
			result = map[string]any{}
		}
		printMutation(result, "", breadcrumbs)
		return nil
	},
}

// Epic delete flags
var epicDeleteBoard string

var epicDeleteCmd = &cobra.Command{
	Use:   "delete EPIC_ID",
	Short: "Delete an epic",
	Long:  "Deletes an epic. Its cards stay on the board, just unlinked.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		boardID, err := requireBoard(epicDeleteBoard)
		if err != nil {
			return err
		}

		if _, err := getSDK().Delete(cmd.Context(), epicPath(boardID, args[0])+".json"); err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("epics", fmt.Sprintf("fizzy epic list --board %s", boardID), "List epics"),
		}
		printMutation(map[string]any{"deleted": true}, "", breadcrumbs)
		return nil
	},
}

// Epic cards flags
var epicCardsBoard string
var epicCardsPage int
var epicCardsAll bool

var epicCardsCmd = &cobra.Command{
	Use:   "cards EPIC_ID",
	Short: "List the cards of an epic",
	Long:  "Lists the cards linked to an epic.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}
		if err := checkLimitAll(epicCardsAll); err != nil {
			return err
		}

		boardID, err := requireBoard(epicCardsBoard)
		if err != nil {
			return err
		}

		epicID := args[0]
		path := epicPath(boardID, epicID) + "/cards.json"
		if epicCardsPage > 0 {
			path += "?page=" + strconv.Itoa(epicCardsPage)
		}

		ac := getSDK()
		var items any
		var linkNext string

		if epicCardsAll {
			pages, err := ac.GetAll(cmd.Context(), path)
			if err != nil {
				return convertSDKError(err)
			}
			items = jsonAnySlice(pages)
		} else {
			resp, err := ac.Get(cmd.Context(), path)
			if err != nil {
				return convertSDKError(err)
			}
			items = normalizeAny(resp.Data)
			linkNext = parseSDKLinkNext(resp)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy card show <number>", "View card"),
			breadcrumb("add-card", fmt.Sprintf("fizzy epic add-card %s --board %s --card <number>", epicID, boardID), "Add a card"),
		}

		summary := fmt.Sprintf("%d cards", dataCount(items))
		printListPaginated(items, cardColumns, linkNext != "", linkNext, epicCardsAll, summary, breadcrumbs)
		return nil
	},
}

// Epic add-card / remove-card flags
var epicAddCardBoard string
var epicAddCardNumber string
var epicRemoveCardNumber string

var epicAddCardCmd = &cobra.Command{
	Use:   "add-card EPIC_ID",
	Short: "Add a card to an epic",
	Long:  "Links a card (by number) to an epic, replacing any epic it had.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}
		if epicAddCardNumber == "" {
			return newRequiredFlagError("card")
		}

		boardID, err := requireBoard(epicAddCardBoard)
		if err != nil {
			return err
		}

		epicID := args[0]
		if err := setCardEpic(cmd, epicAddCardNumber, epicID); err != nil {
			return err
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("cards", fmt.Sprintf("fizzy epic cards %s --board %s", epicID, boardID), "List epic cards"),
			breadcrumb("card", fmt.Sprintf("fizzy card show %s", epicAddCardNumber), "View card"),
		}
		printMutation(map[string]any{"card": epicAddCardNumber, "epic_id": epicID}, "", breadcrumbs)
		return nil
	},
}

var epicRemoveCardCmd = &cobra.Command{
	Use:   "remove-card",
	Short: "Remove a card from its epic",
	Long:  "Unlinks a card (by number) from whatever epic it belongs to.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}
		if epicRemoveCardNumber == "" {
			return newRequiredFlagError("card")
		}

		if err := setCardEpic(cmd, epicRemoveCardNumber, ""); err != nil {
			return err
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("card", fmt.Sprintf("fizzy card show %s", epicRemoveCardNumber), "View card"),
		}
		printMutation(map[string]any{"card": epicRemoveCardNumber, "epic_id": nil}, "", breadcrumbs)
		return nil
	},
}

// The card update endpoint only reads epic_id from the nested "card" key,
// and an empty string clears it.
func setCardEpic(cmd *cobra.Command, cardNumber, epicID string) error {
	body := map[string]any{"card": map[string]any{"epic_id": epicID}}
	if _, err := getSDK().Patch(cmd.Context(), "/cards/"+cardNumber+".json", body); err != nil {
		return convertSDKError(err)
	}
	return nil
}

func addEpicAttrFlags(cmd *cobra.Command, attrs *epicAttrs, nameUsage string) {
	cmd.Flags().StringVar(&attrs.name, "name", "", nameUsage)
	cmd.Flags().StringVar(&attrs.description, "description", "", "Epic description")
	cmd.Flags().StringVar(&attrs.color, "color", "", "Epic color ("+columnColorNamesHelp+")")
	cmd.Flags().StringVar(&attrs.owner, "owner", "", "Owner user ID")
	cmd.Flags().StringVar(&attrs.start, "start", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&attrs.target, "target", "", "Target date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&attrs.status, "status", "", "Status (not_started, in_progress, completed)")
	cmd.Flags().StringVar(&attrs.wave, "wave", "", "Wave ID to put the epic in")
	cmd.Flags().BoolVar(&attrs.noWave, "no-wave", false, "Take the epic out of its wave")
}

func init() {
	rootCmd.AddCommand(epicCmd)

	epicListCmd.Flags().StringVar(&epicListBoard, "board", "", "Board ID (required)")
	epicCmd.AddCommand(epicListCmd)

	epicShowCmd.Flags().StringVar(&epicShowBoard, "board", "", "Board ID (required)")
	epicCmd.AddCommand(epicShowCmd)

	epicCreateCmd.Flags().StringVar(&epicCreateBoard, "board", "", "Board ID (required)")
	addEpicAttrFlags(epicCreateCmd, &epicCreateAttrs, "Epic name (required)")
	epicCmd.AddCommand(epicCreateCmd)

	epicUpdateCmd.Flags().StringVar(&epicUpdateBoard, "board", "", "Board ID (required)")
	addEpicAttrFlags(epicUpdateCmd, &epicUpdateAttrs, "Epic name")
	epicCmd.AddCommand(epicUpdateCmd)

	epicDeleteCmd.Flags().StringVar(&epicDeleteBoard, "board", "", "Board ID (required)")
	epicCmd.AddCommand(epicDeleteCmd)

	epicCardsCmd.Flags().StringVar(&epicCardsBoard, "board", "", "Board ID (required)")
	epicCardsCmd.Flags().IntVar(&epicCardsPage, "page", 0, "Page number")
	epicCardsCmd.Flags().BoolVar(&epicCardsAll, "all", false, "Fetch all pages")
	epicCmd.AddCommand(epicCardsCmd)

	epicAddCardCmd.Flags().StringVar(&epicAddCardBoard, "board", "", "Board ID (required)")
	epicAddCardCmd.Flags().StringVar(&epicAddCardNumber, "card", "", "Card number (required)")
	epicCmd.AddCommand(epicAddCardCmd)

	epicRemoveCardCmd.Flags().StringVar(&epicRemoveCardNumber, "card", "", "Card number (required)")
	epicCmd.AddCommand(epicRemoveCardCmd)
}
