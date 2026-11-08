package commands

import (
	"fmt"

	"github.com/basecamp/fizzy-cli/internal/errors"
	"github.com/spf13/cobra"
)

// cardPlacement holds the mutually exclusive --before / --after / --top / --bottom flags.
type cardPlacement struct {
	before string
	after  string
	top    bool
	bottom bool
}

func (p *cardPlacement) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&p.before, "before", "", "Place the card right before this card number (same column)")
	cmd.Flags().StringVar(&p.after, "after", "", "Place the card right after this card number (same column)")
	cmd.Flags().BoolVar(&p.top, "top", false, "Place the card at the top of its column")
	cmd.Flags().BoolVar(&p.bottom, "bottom", false, "Place the card at the bottom of its column")
}

func (p *cardPlacement) reset() {
	*p = cardPlacement{}
}

func (p *cardPlacement) given() bool {
	return p.before != "" || p.after != "" || p.top || p.bottom
}

// body builds the request for POST /cards/{number}/position.json.
func (p *cardPlacement) body() (map[string]any, error) {
	set := 0
	for _, on := range []bool{p.before != "", p.after != "", p.top, p.bottom} {
		if on {
			set++
		}
	}

	switch {
	case set == 0:
		return nil, errors.NewInvalidArgsError("one of --before, --after, --top or --bottom is required")
	case set > 1:
		return nil, errors.NewInvalidArgsError("use only one of --before, --after, --top or --bottom")
	case p.before != "":
		return map[string]any{"before_card_id": p.before}, nil
	case p.after != "":
		return map[string]any{"after_card_id": p.after}, nil
	case p.top:
		return map[string]any{"placement": "top"}, nil
	default:
		return map[string]any{"placement": "bottom"}, nil
	}
}

func (p *cardPlacement) describe() string {
	switch {
	case p.before != "":
		return "before #" + p.before
	case p.after != "":
		return "after #" + p.after
	case p.top:
		return "at the top of its column"
	default:
		return "at the bottom of its column"
	}
}

func positionCard(cmd *cobra.Command, cardNumber string, body map[string]any) (any, error) {
	resp, err := getSDK().Post(cmd.Context(), "/cards/"+cardNumber+"/position.json", body)
	if err != nil {
		return nil, convertSDKError(err)
	}
	if resp == nil {
		return map[string]any{}, nil
	}
	return normalizeAny(resp.Data), nil
}

func cardPositionBreadcrumbs(cardNumber string) []Breadcrumb {
	return []Breadcrumb{
		breadcrumb("show", fmt.Sprintf("fizzy card show %s", cardNumber), "View card"),
		breadcrumb("list", "fizzy card list --board <board_id> --column <column_id>", "List the column in board order"),
		breadcrumb("position", fmt.Sprintf("fizzy card position <number> --after %s", cardNumber), "Place another card after this one"),
	}
}

var cardPositionFlags cardPlacement

var cardPositionCmd = &cobra.Command{
	Use:   "position CARD_NUMBER",
	Short: "Reorder card within its column",
	Long: `Places a card before or after another card of the same column, or at the top or bottom of it.

Exactly one of --before, --after, --top or --bottom is required. The card must already be in a column;
use "fizzy card column CARD_NUMBER --column <id>" with the same flags to move and place it in one go.`,
	Example: `  fizzy card position 1579 --top
  fizzy card position 1580 --after 1579
  fizzy card position 1590 --before 1581
  fizzy card position 1600 --bottom`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		body, err := cardPositionFlags.body()
		if err != nil {
			return err
		}

		cardNumber := args[0]
		card, err := positionCard(cmd, cardNumber, body)
		if err != nil {
			return err
		}

		summary := fmt.Sprintf("Card #%s placed %s", cardNumber, cardPositionFlags.describe())
		printMutation(card, summary, cardPositionBreadcrumbs(cardNumber))
		return nil
	},
}

func init() {
	cardPositionFlags.addFlags(cardPositionCmd)
	cardCmd.AddCommand(cardPositionCmd)
}
