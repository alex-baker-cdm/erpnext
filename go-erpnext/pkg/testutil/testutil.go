package testutil

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Fixture represents a single test case from a JSON fixture file.
type Fixture struct {
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Expected json.RawMessage `json:"expected"`
}

// FixtureFile represents a collection of test fixtures.
type FixtureFile struct {
	Module   string    `json:"module"`
	Function string    `json:"function"`
	Fixtures []Fixture `json:"fixtures"`
}

// LoadFixtures reads and parses a JSON fixture file from the given absolute path.
func LoadFixtures(t *testing.T, path string) FixtureFile {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture file %s: %v", path, err)
	}

	var ff FixtureFile
	if err := json.Unmarshal(data, &ff); err != nil {
		t.Fatalf("failed to parse fixture file %s: %v", path, err)
	}

	return ff
}

// LoadFixturesFromTestdata loads fixtures from the testdata directory relative
// to the module root. It walks up from the current working directory to find
// the testdata folder.
func LoadFixturesFromTestdata(t *testing.T, filename string) FixtureFile {
	t.Helper()

	// Try to find testdata relative to the go-erpnext module root.
	// The test binary's working directory is the package directory,
	// so we walk up until we find testdata/.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		candidate := filepath.Join(dir, "testdata", filename)
		if _, err := os.Stat(candidate); err == nil {
			return LoadFixtures(t, candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("fixture file %s not found in any testdata directory", filename)
	return FixtureFile{} // unreachable
}

// AssertAlmostEqual checks that two float64 values are equal within a tolerance
// determined by the number of decimal places.
func AssertAlmostEqual(t *testing.T, got, want float64, places int, msgAndArgs ...interface{}) {
	t.Helper()

	tol := math.Pow(10, -float64(places))
	if math.Abs(got-want) > tol {
		msg := fmt.Sprintf("values not almost equal within %d places: got %v, want %v (diff %v)",
			places, got, want, math.Abs(got-want))
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
		}
		t.Error(msg)
	}
}

// AssertFloatSliceEqual checks that two float64 slices are equal element-wise
// within the given tolerance (number of decimal places).
func AssertFloatSliceEqual(t *testing.T, got, want []float64, places int, msgAndArgs ...interface{}) {
	t.Helper()

	if len(got) != len(want) {
		msg := fmt.Sprintf("slice length mismatch: got %d, want %d\n  got:  %v\n  want: %v",
			len(got), len(want), got, want)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
		}
		t.Error(msg)
		return
	}

	for i := range got {
		tol := math.Pow(10, -float64(places))
		if math.Abs(got[i]-want[i]) > tol {
			msg := fmt.Sprintf("slice element [%d] mismatch: got %v, want %v (diff %v)",
				i, got[i], want[i], math.Abs(got[i]-want[i]))
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
			}
			t.Error(msg)
		}
	}
}

// AssertStockBinsEqual checks that two stock bin slices ([][]float64) are equal
// element-wise within the given tolerance.
func AssertStockBinsEqual(t *testing.T, got, want [][]float64, places int, msgAndArgs ...interface{}) {
	t.Helper()

	if len(got) != len(want) {
		msg := fmt.Sprintf("bin count mismatch: got %d bins, want %d bins\n  got:  %v\n  want: %v",
			len(got), len(want), got, want)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
		}
		t.Error(msg)
		return
	}

	for i := range got {
		if len(got[i]) != len(want[i]) {
			msg := fmt.Sprintf("bin [%d] length mismatch: got %d, want %d\n  got:  %v\n  want: %v",
				i, len(got[i]), len(want[i]), got[i], want[i])
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
			}
			t.Error(msg)
			continue
		}
		for j := range got[i] {
			tol := math.Pow(10, -float64(places))
			if math.Abs(got[i][j]-want[i][j]) > tol {
				msg := fmt.Sprintf("bin [%d][%d] mismatch: got %v, want %v (diff %v)",
					i, j, got[i][j], want[i][j], math.Abs(got[i][j]-want[i][j]))
				if len(msgAndArgs) > 0 {
					msg = fmt.Sprintf("%s — %s", msg, formatMsgAndArgs(msgAndArgs))
				}
				t.Error(msg)
			}
		}
	}
}

// DiffJSON produces a human-readable diff between two values by comparing their
// JSON representations. Returns an empty string if the values are equivalent.
func DiffJSON(got, want interface{}) string {
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal got: %v", err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		return fmt.Sprintf("failed to marshal want: %v", err)
	}

	gotStr := string(gotJSON)
	wantStr := string(wantJSON)

	if gotStr == wantStr {
		return ""
	}

	// Produce a line-by-line diff.
	gotLines := strings.Split(gotStr, "\n")
	wantLines := strings.Split(wantStr, "\n")

	var sb strings.Builder
	sb.WriteString("JSON diff:\n")

	// Use a simple approach: compare normalized JSON objects if possible,
	// then show line-by-line differences.
	var gotObj, wantObj interface{}
	json.Unmarshal(gotJSON, &gotObj)
	json.Unmarshal(wantJSON, &wantObj)

	if reflect.DeepEqual(gotObj, wantObj) {
		return ""
	}

	maxLines := len(gotLines)
	if len(wantLines) > maxLines {
		maxLines = len(wantLines)
	}

	for i := 0; i < maxLines; i++ {
		gotLine := ""
		wantLine := ""
		if i < len(gotLines) {
			gotLine = gotLines[i]
		}
		if i < len(wantLines) {
			wantLine = wantLines[i]
		}
		if gotLine != wantLine {
			if gotLine != "" {
				sb.WriteString(fmt.Sprintf("  - got:  %s\n", gotLine))
			}
			if wantLine != "" {
				sb.WriteString(fmt.Sprintf("  + want: %s\n", wantLine))
			}
		} else {
			sb.WriteString(fmt.Sprintf("    %s\n", gotLine))
		}
	}

	return sb.String()
}

// formatMsgAndArgs formats optional message arguments for assertion output.
func formatMsgAndArgs(msgAndArgs []interface{}) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if len(msgAndArgs) == 1 {
		if s, ok := msgAndArgs[0].(string); ok {
			return s
		}
		return fmt.Sprintf("%v", msgAndArgs[0])
	}
	if s, ok := msgAndArgs[0].(string); ok {
		return fmt.Sprintf(s, msgAndArgs[1:]...)
	}
	return fmt.Sprintf("%v", msgAndArgs)
}
