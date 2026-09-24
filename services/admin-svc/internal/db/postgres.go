package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB handles database operations for the admin service
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection
func NewDB(connectionString string) (*DB, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	return &DB{conn: db}, nil
}

// Ping checks the database connection
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateOrganization creates a new organization record
func (db *DB) CreateOrganization(ctx context.Context, name, slug, provider, providerID string) (string, error) {
	var id string
	query := `INSERT INTO organizations (name, slug, provider, provider_id) 
			  VALUES ($1, $2, $3, $4) RETURNING id`

	err := db.conn.QueryRowContext(ctx, query, name, slug, provider, providerID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create org: %w", err)
	}

	return id, nil
}

// GetOrganization retrieves organization by ID
func (db *DB) GetOrganization(ctx context.Context, id string) (map[string]interface{}, error) {
	query := `SELECT id, name, slug, provider, provider_id, settings FROM organizations WHERE id = $1`

	var orgID, name, slug, provider, providerID string
	var settings []byte

	err := db.conn.QueryRowContext(ctx, query, id).Scan(&orgID, &name, &slug, &provider, &providerID, &settings)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":          orgID,
		"name":        name,
		"slug":        slug,
		"provider":    provider,
		"provider_id": providerID,
		"settings":    string(settings),
	}, nil
}

// AddRepository links a repository to an organization
func (db *DB) AddRepository(ctx context.Context, orgID, provider, providerRepoID, fullName, defaultBranch, language string, isPrivate bool) error {
	query := `INSERT INTO repositories (org_id, provider, provider_repo_id, full_name, default_branch, language, is_private) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7) 
			  ON CONFLICT (provider, provider_repo_id) DO UPDATE SET full_name = EXCLUDED.full_name`

	_, err := db.conn.ExecContext(ctx, query, orgID, provider, providerRepoID, fullName, defaultBranch, language, isPrivate)
	return err
}

// GetOrganizationRepos retrieves all repositories for an organization
func (db *DB) GetOrganizationRepos(ctx context.Context, orgID string) ([]map[string]interface{}, error) {
	query := `SELECT id, full_name, language, is_private FROM repositories WHERE org_id = $1`

	rows, err := db.conn.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []map[string]interface{}
	for rows.Next() {
		var id, fullName, language string
		var isPrivate bool
		if err := rows.Scan(&id, &fullName, &language, &isPrivate); err != nil {
			return nil, err
		}
		repos = append(repos, map[string]interface{}{
			"id":         id,
			"full_name":  fullName,
			"language":   language,
			"is_private": isPrivate,
		})
	}

	return repos, nil
}

// UpdateSubscription updates the billing plan for an organization
func (db *DB) UpdateSubscription(ctx context.Context, orgID, plan, stripeCustomerID, stripeSubscriptionID string) error {
	query := `INSERT INTO subscriptions (org_id, plan, stripe_customer_id, stripe_subscription_id, status) 
			  VALUES ($1, $2, $3, $4, 'active')
			  ON CONFLICT (stripe_subscription_id) DO UPDATE SET plan = EXCLUDED.plan, status = 'active'`

	_, err := db.conn.ExecContext(ctx, query, orgID, plan, stripeCustomerID, stripeSubscriptionID)
	return err
}

// GetOrgSubscription retrieves subscription details
func (db *DB) GetOrgSubscription(ctx context.Context, orgID string) (map[string]interface{}, error) {
	query := `SELECT plan, status, stripe_customer_id FROM subscriptions WHERE org_id = $1 LIMIT 1`

	var plan, status, customerID string
	err := db.conn.QueryRowContext(ctx, query, orgID).Scan(&plan, &status, &customerID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"plan":        plan,
		"status":      status,
		"customer_id": customerID,
	}, nil
}
