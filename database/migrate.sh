#!/bin/bash

# Database Migration Script for Agent Builder
# Usage: ./migrate.sh [up|down|status] [migration_number]

set -e

# Configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-tas_shared}"
DB_USER="${DB_USER:-tasuser}"
MIGRATIONS_DIR="$(dirname "$0")/migrations"
ROLLBACK_DIR="$MIGRATIONS_DIR/rollback"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if psql is available
check_psql() {
    if ! command -v psql &> /dev/null; then
        print_error "psql command not found. Please install PostgreSQL client."
        exit 1
    fi
}

# Function to test database connection
test_connection() {
    print_status "Testing database connection..."
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
        print_status "Database connection successful."
    else
        print_error "Cannot connect to database. Please check your connection parameters."
        print_error "Host: $DB_HOST, Port: $DB_PORT, Database: $DB_NAME, User: $DB_USER"
        exit 1
    fi
}

# Function to create migration tracking table
create_migration_table() {
    print_status "Creating migration tracking table if it doesn't exist..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        CREATE TABLE IF NOT EXISTS schema_migrations (
            id SERIAL PRIMARY KEY,
            version VARCHAR(50) NOT NULL UNIQUE,
            name VARCHAR(255) NOT NULL,
            applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        );
        CREATE INDEX IF NOT EXISTS idx_schema_migrations_version ON schema_migrations(version);
    " > /dev/null 2>&1
}

# Function to check if migration has been applied
is_migration_applied() {
    local version=$1
    local count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM schema_migrations WHERE version = '$version';
    " 2>/dev/null | xargs)
    
    if [ "$count" -eq "1" ]; then
        return 0  # Migration is applied
    else
        return 1  # Migration is not applied
    fi
}

# Function to apply a migration
apply_migration() {
    local migration_file=$1
    local version=$(basename "$migration_file" .sql)
    local name=$(echo "$version" | sed 's/^[0-9]*_//' | sed 's/_/ /g')
    
    print_status "Applying migration: $version"
    
    if is_migration_applied "$version"; then
        print_warning "Migration $version already applied, skipping."
        return 0
    fi
    
    # Apply the migration
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$migration_file"; then
        # Record the migration
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
            INSERT INTO schema_migrations (version, name) VALUES ('$version', '$name');
        " > /dev/null 2>&1
        print_status "Migration $version applied successfully."
    else
        print_error "Failed to apply migration $version"
        exit 1
    fi
}

# Function to rollback a migration
rollback_migration() {
    local version=$1
    local rollback_file="$ROLLBACK_DIR/${version}.sql"
    
    print_status "Rolling back migration: $version"
    
    if ! is_migration_applied "$version"; then
        print_warning "Migration $version was not applied, skipping rollback."
        return 0
    fi
    
    if [ ! -f "$rollback_file" ]; then
        print_error "Rollback file not found: $rollback_file"
        exit 1
    fi
    
    # Apply the rollback
    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$rollback_file"; then
        # Remove the migration record
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
            DELETE FROM schema_migrations WHERE version = '$version';
        " > /dev/null 2>&1
        print_status "Migration $version rolled back successfully."
    else
        print_error "Failed to rollback migration $version"
        exit 1
    fi
}

# Function to show migration status
show_status() {
    print_status "Migration Status:"
    echo "=================="
    
    # List all migration files
    for migration_file in "$MIGRATIONS_DIR"/*.sql; do
        if [ -f "$migration_file" ]; then
            local version=$(basename "$migration_file" .sql)
            if is_migration_applied "$version"; then
                echo -e "${GREEN}✓${NC} $version (applied)"
            else
                echo -e "${RED}✗${NC} $version (pending)"
            fi
        fi
    done
    
    echo
    print_status "Applied migrations:"
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SELECT version, name, applied_at FROM schema_migrations ORDER BY applied_at;
    " 2>/dev/null || print_warning "No migrations applied yet."
}

# Function to migrate up
migrate_up() {
    local target_version=$1
    
    print_status "Running migrations up..."
    
    for migration_file in "$MIGRATIONS_DIR"/*.sql; do
        if [ -f "$migration_file" ]; then
            local version=$(basename "$migration_file" .sql)
            
            # If target version is specified, only migrate up to that version
            if [ -n "$target_version" ] && [ "$version" \> "$target_version" ]; then
                break
            fi
            
            apply_migration "$migration_file"
        fi
    done
    
    print_status "All migrations completed successfully."
}

# Function to migrate down
migrate_down() {
    local target_version=$1
    
    if [ -z "$target_version" ]; then
        print_error "Target version required for rollback. Usage: ./migrate.sh down 002_migration_name"
        exit 1
    fi
    
    print_status "Rolling back to version: $target_version"
    
    # Get list of applied migrations in reverse order
    local applied_migrations=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT version FROM schema_migrations WHERE version > '$target_version' ORDER BY version DESC;
    " 2>/dev/null | xargs)
    
    for version in $applied_migrations; do
        rollback_migration "$version"
    done
    
    print_status "Rollback completed successfully."
}

# Main script logic
main() {
    local command=${1:-status}
    local version=$2
    
    check_psql
    test_connection
    create_migration_table
    
    case "$command" in
        "up")
            migrate_up "$version"
            ;;
        "down")
            migrate_down "$version"
            ;;
        "status")
            show_status
            ;;
        *)
            echo "Usage: $0 [up|down|status] [migration_version]"
            echo ""
            echo "Commands:"
            echo "  up [version]     - Apply migrations (optionally up to specific version)"
            echo "  down version     - Rollback migrations down to specific version"
            echo "  status           - Show migration status"
            echo ""
            echo "Environment Variables:"
            echo "  DB_HOST          - Database host (default: localhost)"
            echo "  DB_PORT          - Database port (default: 5432)"
            echo "  DB_NAME          - Database name (default: aether)"
            echo "  DB_USER          - Database user (default: postgres)"
            echo "  PGPASSWORD       - Database password"
            exit 1
            ;;
    esac
}

main "$@"