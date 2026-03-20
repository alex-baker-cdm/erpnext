package stock

import (
	"fmt"
	"strings"

	"github.com/alex-baker-cdm/erpnext/go-erpnext/pkg/db"
)

// PreviousSLEArgs holds the parameters for GetPreviousSLE, mirroring the Python
// dict passed to get_previous_sle in erpnext/stock/stock_ledger.py:1784.
type PreviousSLEArgs struct {
	ItemCode    string
	Warehouse   string
	PostingDate string
	PostingTime string
	ExtraCond   string
	ExtraArgs   []interface{}
}

// GetPreviousSLE retrieves the most recent Stock Ledger Entry on or before the
// given posting datetime for the specified item and warehouse.
//
// This replicates the SQL in erpnext/stock/stock_ledger.py:1805-1889.
func GetPreviousSLE(d *db.DB, args PreviousSLEArgs) (map[string]interface{}, error) {
	postingDate := args.PostingDate
	if postingDate == "" {
		postingDate = "1900-01-01"
	}
	postingTime := args.PostingTime
	if postingTime == "" {
		postingTime = "00:00:00"
	}
	postingDatetime := postingDate + " " + postingTime

	var conditions []string
	var queryArgs []interface{}

	conditions = append(conditions, "is_cancelled = 0")
	conditions = append(conditions, "posting_datetime <= ?")
	queryArgs = append(queryArgs, postingDatetime)

	if args.ItemCode != "" {
		conditions = append(conditions, "item_code = ?")
		queryArgs = append(queryArgs, args.ItemCode)
	}

	if args.Warehouse != "" {
		conditions = append(conditions, "warehouse = ?")
		queryArgs = append(queryArgs, args.Warehouse)
	}

	if args.ExtraCond != "" {
		conditions = append(conditions, args.ExtraCond)
		queryArgs = append(queryArgs, args.ExtraArgs...)
	}

	query := fmt.Sprintf(
		"SELECT *, posting_datetime as `timestamp` FROM `tabStock Ledger Entry` WHERE %s ORDER BY posting_datetime DESC, creation DESC LIMIT 1",
		strings.Join(conditions, " AND "),
	)

	row, err := d.RawQueryRow(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("get previous SLE: %w", err)
	}
	return row, nil
}
