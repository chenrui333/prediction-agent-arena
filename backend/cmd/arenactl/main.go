package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type client struct {
	baseURL string
	token   string
	http    *http.Client
}

type round struct {
	ID                  int64  `json:"id"`
	Slug                string `json:"slug"`
	Name                string `json:"name"`
	Mode                string `json:"mode"`
	Status              string `json:"status"`
	InitialBalanceCents int64  `json:"initial_balance_cents"`
}

type team struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

type market struct {
	ID          int64  `json:"id"`
	Venue       string `json:"venue"`
	ExternalID  string `json:"external_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	YesPriceBPS int64  `json:"yes_price_bps"`
	NoPriceBPS  int64  `json:"no_price_bps"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return usage()
	}
	c := client{
		baseURL: strings.TrimRight(env("ARENA_BASE_URL", "http://localhost:8080"), "/"),
		token:   env("ARENA_ADMIN_TOKEN", env("ADMIN_TOKEN", "dev-admin-token")),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
	switch args[0] {
	case "create-team":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		slug := fs.String("slug", "", "team slug")
		name := fs.String("name", "", "team name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *slug == "" {
			return errors.New("--slug is required")
		}
		if *name == "" {
			*name = *slug
		}
		var result map[string]interface{}
		if err := c.do("POST", "/api/v1/admin/teams", map[string]string{"slug": *slug, "name": *name}, &result); err != nil {
			return err
		}
		if err := printJSON(out, result); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "New token shown once. Store it privately; existing token hashes are never printable.")
		return nil
	case "seed-demo":
		return c.seedDemo(out)
	case "create-round":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		slug := fs.String("slug", env("ROUND", ""), "round slug")
		name := fs.String("name", "", "round name")
		mode := fs.String("mode", "practice", "round mode")
		initial := fs.Int64("initial-balance-cents", 1000000, "initial balance in cents")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *slug == "" {
			return errors.New("--slug or ROUND is required")
		}
		if *name == "" {
			*name = *slug
		}
		return c.print(out, "POST", "/api/v1/admin/rounds", map[string]interface{}{"slug": *slug, "name": *name, "mode": *mode, "status": "draft", "initial_balance_cents": *initial})
	case "activate-round", "pause-round", "complete-round", "reset-round", "freeze-leaderboard", "export-round":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		roundArg := fs.String("round", env("ROUND", ""), "round id or slug")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		r, err := c.resolveRound(*roundArg)
		if err != nil {
			return err
		}
		action := strings.TrimSuffix(args[0], "-round")
		if args[0] == "freeze-leaderboard" {
			return c.print(out, "POST", fmt.Sprintf("/api/v1/admin/rounds/%d/freeze-leaderboard", r.ID), nil)
		}
		if args[0] == "export-round" {
			return c.print(out, "GET", fmt.Sprintf("/api/v1/admin/export/%d", r.ID), nil)
		}
		return c.print(out, "POST", fmt.Sprintf("/api/v1/admin/rounds/%d/%s", r.ID, action), nil)
	case "pause-team", "resume-team", "reset-team":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		teamArg := fs.String("team", env("TEAM", ""), "team id or slug")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		t, err := c.resolveTeam(*teamArg)
		if err != nil {
			return err
		}
		action := strings.TrimSuffix(args[0], "-team")
		return c.print(out, "POST", fmt.Sprintf("/api/v1/admin/teams/%d/%s", t.ID, action), nil)
	case "print-active-round":
		summary := struct {
			ActiveRound *round `json:"active_round"`
		}{}
		if err := c.do("GET", "/api/v1/admin/summary", nil, &summary); err != nil {
			return err
		}
		if summary.ActiveRound == nil {
			return errors.New("no active round")
		}
		return printJSON(out, summary.ActiveRound)
	case "print-team-tokens":
		fmt.Fprintln(out, "Existing team tokens cannot be printed. Only create-team and seed-demo show newly generated tokens once.")
		return nil
	default:
		return usage()
	}
}

func (c client) resolveRound(value string) (round, error) {
	if value == "" {
		summary := struct {
			ActiveRound *round `json:"active_round"`
			LatestRound *round `json:"latest_round"`
		}{}
		if err := c.do("GET", "/api/v1/admin/summary", nil, &summary); err != nil {
			return round{}, err
		}
		if summary.ActiveRound != nil {
			return *summary.ActiveRound, nil
		}
		if summary.LatestRound != nil {
			return *summary.LatestRound, nil
		}
		return round{}, errors.New("no round found")
	}
	var rounds []round
	if err := c.do("GET", "/api/v1/admin/rounds", nil, &rounds); err != nil {
		return round{}, err
	}
	for _, r := range rounds {
		if fmt.Sprint(r.ID) == value || r.Slug == value {
			return r, nil
		}
	}
	return round{}, fmt.Errorf("round not found: %s", value)
}

func (c client) resolveTeam(value string) (team, error) {
	if value == "" {
		return team{}, errors.New("--team or TEAM is required")
	}
	var teams []team
	if err := c.do("GET", "/api/v1/admin/teams", nil, &teams); err != nil {
		return team{}, err
	}
	for _, t := range teams {
		if fmt.Sprint(t.ID) == value || t.Slug == value {
			return t, nil
		}
	}
	return team{}, fmt.Errorf("team not found: %s", value)
}

func (c client) seedDemo(out io.Writer) error {
	r, err := c.ensureRound("practice-1", "Practice Round 1")
	if err != nil {
		return err
	}
	markets := []market{
		{Venue: "fake", ExternalID: "bootcamp-demo-1", Slug: "ai-tool-usage-above-60", Title: "Will bootcamp agents average more than 60 percent tool-use accuracy?", Category: "bootcamp", Status: "open", YesPriceBPS: 5700, NoPriceBPS: 4300},
		{Venue: "fake", ExternalID: "bootcamp-demo-2", Slug: "leaderboard-return-positive", Title: "Will at least five teams finish practice round with positive return?", Category: "bootcamp", Status: "open", YesPriceBPS: 5100, NoPriceBPS: 4900},
		{Venue: "fake", ExternalID: "bootcamp-demo-3", Slug: "risk-rejections-under-20", Title: "Will total rejected orders stay under 20 by round end?", Category: "bootcamp", Status: "open", YesPriceBPS: 6300, NoPriceBPS: 3700},
		{Venue: "fake", ExternalID: "bootcamp-demo-4", Slug: "final-demo-on-time", Title: "Will every team submit a final demo before the deadline?", Category: "bootcamp", Status: "open", YesPriceBPS: 6900, NoPriceBPS: 3100},
	}
	for _, m := range markets {
		var created market
		payload := map[string]interface{}{
			"venue":         m.Venue,
			"external_id":   m.ExternalID,
			"slug":          m.Slug,
			"title":         m.Title,
			"category":      m.Category,
			"status":        m.Status,
			"yes_price_bps": m.YesPriceBPS,
			"no_price_bps":  m.NoPriceBPS,
			"metadata_json": "{}",
		}
		if err := c.do("POST", "/api/v1/admin/markets", payload, &created); err != nil {
			return err
		}
		if err := c.do("POST", fmt.Sprintf("/api/v1/admin/rounds/%d/markets/%d", r.ID, created.ID), nil, nil); err != nil {
			return err
		}
	}
	existing, err := c.listTeams()
	if err != nil {
		return err
	}
	existingBySlug := map[string]bool{}
	for _, t := range existing {
		existingBySlug[t.Slug] = true
	}
	type createdTeam struct {
		ID       int64  `json:"id"`
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		APIToken string `json:"api_token"`
	}
	created := []createdTeam{}
	for i := 1; i <= 10; i++ {
		slug := fmt.Sprintf("team-%02d", i)
		if existingBySlug[slug] {
			continue
		}
		var result createdTeam
		if err := c.do("POST", "/api/v1/admin/teams", map[string]string{"slug": slug, "name": fmt.Sprintf("Team %02d", i)}, &result); err != nil {
			return err
		}
		created = append(created, result)
	}
	var activated round
	if err := c.do("POST", fmt.Sprintf("/api/v1/admin/rounds/%d/activate", r.ID), nil, &activated); err != nil {
		return err
	}
	fmt.Fprintf(out, "Seeded %s at %s\n", activated.Slug, c.baseURL)
	if len(created) == 0 {
		fmt.Fprintln(out, "Demo teams already exist; existing token values are not available.")
		return nil
	}
	fmt.Fprintln(out, "Demo team tokens, shown once:")
	for _, t := range created {
		fmt.Fprintf(out, "%s %s\n", t.Slug, t.APIToken)
	}
	return nil
}

func (c client) ensureRound(slug, name string) (round, error) {
	rounds, err := c.listRounds()
	if err != nil {
		return round{}, err
	}
	for _, r := range rounds {
		if r.Slug == slug {
			return r, nil
		}
	}
	var created round
	err = c.do("POST", "/api/v1/admin/rounds", map[string]interface{}{"slug": slug, "name": name, "mode": "practice", "status": "draft", "initial_balance_cents": 1000000}, &created)
	return created, err
}

func (c client) listRounds() ([]round, error) {
	var rounds []round
	err := c.do("GET", "/api/v1/admin/rounds", nil, &rounds)
	return rounds, err
}

func (c client) listTeams() ([]team, error) {
	var teams []team
	err := c.do("GET", "/api/v1/admin/teams", nil, &teams)
	return teams, err
}

func (c client) print(out io.Writer, method, path string, payload interface{}) error {
	var result interface{}
	if err := c.do(method, path, payload, &result); err != nil {
		return err
	}
	return printJSON(out, result)
}

func (c client) do(method, path string, payload interface{}, dest interface{}) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s failed: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func printJSON(out io.Writer, value interface{}) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(raw))
	return err
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func usage() error {
	return errors.New("usage: arenactl <seed-demo|create-team|create-round|activate-round|pause-round|complete-round|reset-team|pause-team|resume-team|reset-round|freeze-leaderboard|export-round|print-active-round|print-team-tokens>")
}
