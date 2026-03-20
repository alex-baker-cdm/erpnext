package db

import "fmt"

// RawQuery executes a raw SQL query with positional ? parameters and returns
// results as a slice of maps, consistent with GetDoc/GetList.
func (d *DB) RawQuery(query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("raw query: %w", err)
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

// RawQueryRow executes a raw SQL query expected to return at most one row.
// Returns nil (without error) if no rows match.
func (d *DB) RawQueryRow(query string, args ...interface{}) (map[string]interface{}, error) {
	results, err := d.RawQuery(query, args...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}
