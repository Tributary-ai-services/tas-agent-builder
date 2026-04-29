// model-migrate detects agent rows whose llm_config.model is no longer in
// the LLM router's current catalog and rewrites them to a current model of
// the same family tier. Every change is written to agent_builder.model_migrations
// for audit and reversal.
//
// Flags:
//
//	--dry-run  (default) report what would change without writing
//	--apply    perform the migration and write audit rows
//
// The binary reads its DB and router config from environment variables, the
// same set used by the main agent-builder service (see config.LoadConfig).
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/tas-agent-builder/config"
)

type providerResponse struct {
	Capabilities struct {
		SupportedModels []struct {
			Name string `json:"name"`
		} `json:"supported_models"`
	} `json:"capabilities"`
}

type staleRow struct {
	AgentID  string
	Provider string
	Model    string
}

func main() {
	apply := flag.Bool("apply", false, "perform the migration (default is dry-run)")
	dryRun := flag.Bool("dry-run", true, "report changes without writing (default true)")
	flag.Parse()
	if *apply {
		*dryRun = false
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := openDB(cfg)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	current, err := fetchCurrentModels(ctx, cfg.Router.BaseURL)
	if err != nil {
		log.Fatalf("fetch current models: %v", err)
	}
	if len(current) == 0 {
		log.Fatalf("router returned no providers/models — refusing to migrate")
	}
	logCurrent(current)

	stale, err := findStaleAgents(ctx, db, current)
	if err != nil {
		log.Fatalf("scan agents: %v", err)
	}

	if len(stale) == 0 {
		log.Println("no stale agent rows found — nothing to migrate")
		return
	}

	log.Printf("found %d stale agent row(s)", len(stale))

	migrated := 0
	skipped := 0
	for _, row := range stale {
		newModel := pickSameTier(current[row.Provider], row.Model)
		if newModel == "" {
			log.Printf("SKIP agent=%s provider=%s model=%s — no same-tier current model in catalog",
				row.AgentID, row.Provider, row.Model)
			skipped++
			continue
		}
		tier := detectTier(row.Model)
		if *dryRun {
			log.Printf("DRY agent=%s provider=%s tier=%s %s -> %s",
				row.AgentID, row.Provider, tier, row.Model, newModel)
			continue
		}
		if err := migrateAgent(ctx, db, row, newModel, tier); err != nil {
			log.Printf("FAIL agent=%s %s -> %s: %v", row.AgentID, row.Model, newModel, err)
			skipped++
			continue
		}
		log.Printf("OK  agent=%s provider=%s tier=%s %s -> %s",
			row.AgentID, row.Provider, tier, row.Model, newModel)
		migrated++
	}

	if *dryRun {
		log.Printf("dry-run complete: %d would migrate, %d would skip", len(stale)-skipped, skipped)
		return
	}
	log.Printf("migration complete: %d migrated, %d skipped", migrated, skipped)
	if skipped > 0 {
		// Non-zero exit signals the cronjob driver that manual attention is needed
		// even though some rows were migrated successfully.
		os.Exit(2)
	}
}

func openDB(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
		cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

// fetchCurrentModels queries the router for each known provider it exposes
// and returns a provider -> []modelName map of the current catalog.
func fetchCurrentModels(ctx context.Context, routerBase string) (map[string][]string, error) {
	providers, err := fetchProviders(ctx, routerBase)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(providers))
	for _, p := range providers {
		models, err := fetchProviderModels(ctx, routerBase, p)
		if err != nil {
			return nil, fmt.Errorf("provider %s: %w", p, err)
		}
		out[p] = models
	}
	return out, nil
}

func fetchProviders(ctx context.Context, routerBase string) ([]string, error) {
	var resp struct {
		Providers []string `json:"providers"`
	}
	if err := getJSON(ctx, routerBase+"/v1/providers", &resp); err != nil {
		return nil, err
	}
	return resp.Providers, nil
}

func fetchProviderModels(ctx context.Context, routerBase, provider string) ([]string, error) {
	var resp providerResponse
	if err := getJSON(ctx, routerBase+"/v1/providers/"+provider, &resp); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(resp.Capabilities.SupportedModels))
	for _, m := range resp.Capabilities.SupportedModels {
		if m.Name != "" {
			models = append(models, m.Name)
		}
	}
	return models, nil
}

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// findStaleAgents returns every agent whose (provider, model) is not present
// in the current catalog. Agents targeting a provider the router does not
// expose are left alone — a missing provider is an operator issue, not a
// model-drift issue, and overwriting them would erase intent.
func findStaleAgents(ctx context.Context, db *sql.DB, current map[string][]string) ([]staleRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id::text,
		       COALESCE(llm_config->>'provider', ''),
		       COALESCE(llm_config->>'model', '')
		FROM agent_builder.agents
		WHERE llm_config ? 'model'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stale []staleRow
	for rows.Next() {
		var r staleRow
		if err := rows.Scan(&r.AgentID, &r.Provider, &r.Model); err != nil {
			return nil, err
		}
		if r.Provider == "" || r.Model == "" {
			continue
		}
		catalog, ok := current[r.Provider]
		if !ok {
			continue
		}
		if contains(catalog, r.Model) {
			continue
		}
		stale = append(stale, r)
	}
	return stale, rows.Err()
}

func migrateAgent(ctx context.Context, db *sql.DB, row staleRow, newModel, tier string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_builder.agents
		SET llm_config = jsonb_set(llm_config, '{model}', to_jsonb($1::text)),
		    updated_at = NOW()
		WHERE id = $2::uuid
	`, newModel, row.AgentID); err != nil {
		return fmt.Errorf("update agent: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_builder.model_migrations
		    (agent_id, provider, old_model, new_model, tier, source)
		VALUES ($1::uuid, $2, $3, $4, $5, 'cronjob')
	`, row.AgentID, row.Provider, row.Model, newModel, tier); err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}

	return tx.Commit()
}

// pickSameTier returns the first model in catalog whose tier matches oldModel's
// tier, or "" if no same-tier model exists.
func pickSameTier(catalog []string, oldModel string) string {
	tier := detectTier(oldModel)
	if tier == "" {
		return ""
	}
	for _, m := range catalog {
		if m == oldModel {
			continue
		}
		if detectTier(m) == tier {
			return m
		}
	}
	return ""
}

// detectTier maps both deprecated and current model IDs to a canonical family
// name so we can migrate within a family (haiku → haiku, etc.) rather than
// across tiers (which would silently change cost/quality characteristics).
func detectTier(modelID string) string {
	m := strings.ToLower(modelID)
	switch {
	case strings.Contains(m, "haiku"):
		return "haiku"
	case strings.Contains(m, "sonnet"):
		return "sonnet"
	case strings.Contains(m, "opus"):
		return "opus"
	case strings.Contains(m, "gpt-4o-mini"):
		return "gpt-4o-mini"
	case strings.Contains(m, "gpt-4o"):
		return "gpt-4o"
	case strings.Contains(m, "gpt-4"):
		return "gpt-4"
	case strings.Contains(m, "gpt-3.5"):
		return "gpt-3.5"
	}
	return ""
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func logCurrent(current map[string][]string) {
	for p, models := range current {
		log.Printf("router catalog: provider=%s models=%v", p, models)
	}
}
