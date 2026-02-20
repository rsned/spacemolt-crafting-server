// Package db provides SQLite database access for the crafting server.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
)

//go:embed schema.sql
var schemaFS embed.FS

// Schema returns the SQL schema for the database.
func Schema() (string, error) {
	data, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return "", fmt.Errorf("reading embedded schema: %w", err)
	}
	return string(data), nil
}

// InitSchema creates all tables if they don't exist.
func InitSchema(ctx context.Context, db *sql.DB) error {
	schema, err := Schema()
	if err != nil {
		return err
	}
	
	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("executing schema: %w", err)
	}
	
	return nil
}
