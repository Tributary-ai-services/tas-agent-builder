package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("Creating TAS Agent Builder database tables...")

	// Connect to database
	dsn := "host=localhost port=5432 user=tasuser password=taspassword dbname=tas_shared sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	fmt.Println("✅ Connected to database")

	// Create schema
	fmt.Println("Creating agent_builder schema...")
	_, err = db.Exec(`CREATE SCHEMA IF NOT EXISTS agent_builder`)
	if err != nil {
		log.Printf("Warning: Failed to create schema: %v", err)
	} else {
		fmt.Println("✅ Schema created/verified")
	}

	// Create agents table
	fmt.Println("Creating agents table...")
	createAgentsTable := `
	CREATE TABLE IF NOT EXISTS agent_builder.agents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		description TEXT,
		system_prompt TEXT NOT NULL,
		llm_config JSONB NOT NULL DEFAULT '{}',
		owner_id UUID NOT NULL,
		space_id UUID NOT NULL,
		tenant_id VARCHAR(255) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'draft',
		space_type VARCHAR(50) NOT NULL,
		is_public BOOLEAN NOT NULL DEFAULT FALSE,
		is_template BOOLEAN NOT NULL DEFAULT FALSE,
		notebook_ids JSONB DEFAULT '[]',
		enable_knowledge BOOLEAN NOT NULL DEFAULT TRUE,
		enable_memory BOOLEAN NOT NULL DEFAULT TRUE,
		tags JSONB DEFAULT '[]',
		total_executions INTEGER DEFAULT 0,
		total_cost_usd DECIMAL(10,6) DEFAULT 0.000000,
		avg_response_time_ms INTEGER DEFAULT 0,
		last_executed_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMP WITH TIME ZONE
	)`

	_, err = db.Exec(createAgentsTable)
	if err != nil {
		log.Printf("Warning: Failed to create agents table: %v", err)
	} else {
		fmt.Println("✅ Agents table created/verified")
	}

	// Create agent_executions table
	fmt.Println("Creating agent_executions table...")
	createExecutionsTable := `
	CREATE TABLE IF NOT EXISTS agent_builder.agent_executions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		agent_id UUID NOT NULL,
		user_id UUID NOT NULL,
		session_id VARCHAR(255),
		input_data JSONB NOT NULL,
		output_data JSONB,
		status VARCHAR(50) NOT NULL DEFAULT 'queued',
		router_response JSONB,
		execution_steps JSONB DEFAULT '[]',
		token_usage INTEGER,
		cost_usd DECIMAL(10,6),
		total_duration_ms INTEGER,
		error_message TEXT,
		error_details JSONB,
		started_at TIMESTAMP WITH TIME ZONE,
		completed_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		deleted_at TIMESTAMP WITH TIME ZONE
	)`

	_, err = db.Exec(createExecutionsTable)
	if err != nil {
		log.Printf("Warning: Failed to create agent_executions table: %v", err)
	} else {
		fmt.Println("✅ Agent executions table created/verified")
	}

	// Create agent_usage_stats table
	fmt.Println("Creating agent_usage_stats table...")
	createStatsTable := `
	CREATE TABLE IF NOT EXISTS agent_builder.agent_usage_stats (
		agent_id UUID PRIMARY KEY,
		total_executions INTEGER DEFAULT 0,
		successful_executions INTEGER DEFAULT 0,
		failed_executions INTEGER DEFAULT 0,
		total_cost_usd DECIMAL(10,6) DEFAULT 0.000000,
		total_tokens_used BIGINT DEFAULT 0,
		avg_cost_per_execution DECIMAL(10,6) DEFAULT 0.000000,
		avg_response_time_ms INTEGER DEFAULT 0,
		min_response_time_ms INTEGER,
		max_response_time_ms INTEGER,
		p95_response_time_ms INTEGER,
		executions_today INTEGER DEFAULT 0,
		executions_this_week INTEGER DEFAULT 0,
		executions_this_month INTEGER DEFAULT 0,
		cost_today DECIMAL(8,6) DEFAULT 0.000000,
		cost_this_week DECIMAL(8,6) DEFAULT 0.000000,
		cost_this_month DECIMAL(8,6) DEFAULT 0.000000,
		success_rate DECIMAL(5,4) DEFAULT 0.0000,
		error_rate DECIMAL(5,4) DEFAULT 0.0000,
		avg_knowledge_items_used DECIMAL(8,2) DEFAULT 0.00,
		avg_memory_items_used DECIMAL(8,2) DEFAULT 0.00,
		provider_usage_stats JSONB DEFAULT '{}',
		model_usage_stats JSONB DEFAULT '{}',
		last_execution_at TIMESTAMP WITH TIME ZONE,
		stats_last_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		daily_stats_reset_at DATE DEFAULT CURRENT_DATE,
		weekly_stats_reset_at DATE DEFAULT CURRENT_DATE,
		monthly_stats_reset_at DATE DEFAULT CURRENT_DATE
	)`

	_, err = db.Exec(createStatsTable)
	if err != nil {
		log.Printf("Warning: Failed to create agent_usage_stats table: %v", err)
	} else {
		fmt.Println("✅ Agent usage stats table created/verified")
	}

	// Create indexes
	fmt.Println("Creating indexes...")
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_agents_owner_id ON agent_builder.agents(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_space_id ON agent_builder.agents(space_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agent_builder.agents(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agents_status ON agent_builder.agents(status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_executions_agent_id ON agent_builder.agent_executions(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_executions_user_id ON agent_builder.agent_executions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_executions_status ON agent_builder.agent_executions(status)`,
	}

	for _, index := range indexes {
		_, err = db.Exec(index)
		if err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}
	fmt.Println("✅ Indexes created/verified")

	fmt.Println("\n🎉 Database setup complete!")
	fmt.Println("All tables are ready for the Hello Agent demo.")
}