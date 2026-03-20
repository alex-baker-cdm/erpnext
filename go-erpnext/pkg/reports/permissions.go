package reports

// GetMatchCond returns a SQL condition string for user-level permission filtering
// on the given doctype. This is used by reports and search queries to enforce
// row-level security.
//
// TODO: Full permission integration requires the Frappe session/user context,
// which is not yet available in the Go layer. For now, returns an empty string
// (no filtering) so that callers can use the correct function signature.
func GetMatchCond(doctype string) string {
	return ""
}

// BuildMatchConditions builds a list of SQL condition strings for permission
// filtering on the given doctype. Each condition represents a separate
// permission rule that should be ANDed together.
//
// TODO: Full permission integration requires the Frappe session/user context,
// which is not yet available in the Go layer. For now, returns an empty slice
// (no conditions) so that callers can use the correct function signature.
func BuildMatchConditions(doctype string) []string {
	return []string{}
}
