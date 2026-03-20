// Package db provides database access for Frappe/ERPNext.
// It reads site_config.json for credentials and provides query helpers
// equivalent to frappe.get_doc(), frappe.get_value(), and frappe.get_list().
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// validIdentifier matches safe SQL identifiers (alphanumeric, underscores, spaces for Frappe doctypes).
var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_ ]+$`)

// sanitizeIdentifier validates that a string is a safe SQL identifier.
// Returns an error if the identifier contains characters that could enable SQL injection.
func sanitizeIdentifier(name, context string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("invalid %s identifier: %q", context, name)
	}
	return nil
}

// SiteConfig represents the database configuration from Frappe's site_config.json.
type SiteConfig struct {
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBName     string `json:"db_name"`
	DBPassword string `json:"db_password"`
	DBUser     string `json:"db_user"`
}

// DB wraps a sql.DB connection with Frappe-aware query helpers.
type DB struct {
	conn *sql.DB
	cfg  SiteConfig
}

// ReadSiteConfig reads and parses a Frappe site_config.json file.
func ReadSiteConfig(sitePath string) (SiteConfig, error) {
	configPath := filepath.Join(sitePath, "site_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return SiteConfig{}, fmt.Errorf("reading site_config.json: %w", err)
	}

	var cfg SiteConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return SiteConfig{}, fmt.Errorf("parsing site_config.json: %w", err)
	}

	// Defaults
	if cfg.DBHost == "" {
		cfg.DBHost = "127.0.0.1"
	}
	if cfg.DBPort == 0 {
		cfg.DBPort = 3306
	}

	return cfg, nil
}

// New creates a new DB connection using the given SiteConfig.
func New(cfg SiteConfig) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &DB{conn: conn, cfg: cfg}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// tableName converts a doctype name to its MariaDB table name.
// e.g., "Foo Bar" -> "`tabFoo Bar`"
func tableName(doctype string) string {
	return fmt.Sprintf("`tab%s`", doctype)
}

// GetDoc retrieves a single document by doctype and name.
// Returns a map of field names to values (equivalent to frappe.get_doc).
func (d *DB) GetDoc(doctype, name string) (map[string]interface{}, error) {
	if err := sanitizeIdentifier(doctype, "doctype"); err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE name = ? LIMIT 1", tableName(doctype))
	rows, err := d.conn.Query(query, name)
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", doctype, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	if !rows.Next() {
		return nil, fmt.Errorf("document %s/%s not found", doctype, name)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("scanning row: %w", err)
	}

	result := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = val
		}
	}

	return result, nil
}

// GetValue retrieves a single field value from a document.
// Equivalent to frappe.get_value(doctype, name, fieldname).
func (d *DB) GetValue(doctype, name, fieldname string) (interface{}, error) {
	if err := sanitizeIdentifier(doctype, "doctype"); err != nil {
		return nil, err
	}
	if err := sanitizeIdentifier(fieldname, "fieldname"); err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT `%s` FROM %s WHERE name = ? LIMIT 1", fieldname, tableName(doctype))
	var value interface{}
	err := d.conn.QueryRow(query, name).Scan(&value)
	if err != nil {
		return nil, fmt.Errorf("getting value %s.%s for %s: %w", doctype, fieldname, name, err)
	}
	if b, ok := value.([]byte); ok {
		return string(b), nil
	}
	return value, nil
}

// ListOptions specifies options for GetList queries.
type ListOptions struct {
	Filters map[string]interface{}
	Fields  []string
	OrderBy string
	Limit   int
}

// RawQuery executes a raw SQL query and returns the resulting rows.
// The caller is responsible for closing the returned *sql.Rows.
// All user-provided values must be passed as args (parameterized queries).
func (d *DB) RawQuery(query string, args ...interface{}) (*sql.Rows, error) {
	return d.conn.Query(query, args...)
}

// RawQueryRow executes a raw SQL query that is expected to return at most one row.
// All user-provided values must be passed as args (parameterized queries).
func (d *DB) RawQueryRow(query string, args ...interface{}) *sql.Row {
	return d.conn.QueryRow(query, args...)
}

// GetList retrieves a list of documents matching the given criteria.
// Equivalent to frappe.get_list(doctype, filters, fields, order_by, limit).
func (d *DB) GetList(doctype string, opts ListOptions) ([]map[string]interface{}, error) {
	if err := sanitizeIdentifier(doctype, "doctype"); err != nil {
		return nil, err
	}

	fields := "*"
	if len(opts.Fields) > 0 {
		quotedFields := make([]string, len(opts.Fields))
		for i, f := range opts.Fields {
			if err := sanitizeIdentifier(f, "field"); err != nil {
				return nil, err
			}
			quotedFields[i] = fmt.Sprintf("`%s`", f)
		}
		fields = strings.Join(quotedFields, ", ")
	}

	query := fmt.Sprintf("SELECT %s FROM %s", fields, tableName(doctype))
	var args []interface{}

	if len(opts.Filters) > 0 {
		var conditions []string
		for field, value := range opts.Filters {
			if err := sanitizeIdentifier(field, "filter field"); err != nil {
				return nil, err
			}
			conditions = append(conditions, fmt.Sprintf("`%s` = ?", field))
			args = append(args, value)
		}
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if opts.OrderBy != "" {
		query += " ORDER BY " + opts.OrderBy
	}

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", doctype, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	return results, nil
}
