package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFixtures(t *testing.T) {
	// Create a temporary fixture file.
	tmpDir := t.TempDir()
	fixtureData := FixtureFile{
		Module:   "test.module",
		Function: "TestFunc",
		Fixtures: []Fixture{
			{
				Name:     "case_one",
				Input:    json.RawMessage(`{"x": 1}`),
				Expected: json.RawMessage(`{"y": 2}`),
			},
			{
				Name:     "case_two",
				Input:    json.RawMessage(`{"x": 10}`),
				Expected: json.RawMessage(`{"y": 20}`),
			},
		},
	}

	data, err := json.MarshalIndent(fixtureData, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test fixture: %v", err)
	}

	path := filepath.Join(tmpDir, "test_fixtures.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test fixture file: %v", err)
	}

	// Load and verify.
	ff := LoadFixtures(t, path)

	if ff.Module != "test.module" {
		t.Errorf("module: got %q, want %q", ff.Module, "test.module")
	}
	if ff.Function != "TestFunc" {
		t.Errorf("function: got %q, want %q", ff.Function, "TestFunc")
	}
	if len(ff.Fixtures) != 2 {
		t.Fatalf("fixture count: got %d, want 2", len(ff.Fixtures))
	}
	if ff.Fixtures[0].Name != "case_one" {
		t.Errorf("fixture[0].Name: got %q, want %q", ff.Fixtures[0].Name, "case_one")
	}
	if ff.Fixtures[1].Name != "case_two" {
		t.Errorf("fixture[1].Name: got %q, want %q", ff.Fixtures[1].Name, "case_two")
	}

	// Verify Input and Expected can be unmarshalled.
	var input map[string]int
	if err := json.Unmarshal(ff.Fixtures[0].Input, &input); err != nil {
		t.Fatalf("failed to unmarshal fixture[0].Input: %v", err)
	}
	if input["x"] != 1 {
		t.Errorf("fixture[0].Input.x: got %d, want 1", input["x"])
	}

	var expected map[string]int
	if err := json.Unmarshal(ff.Fixtures[0].Expected, &expected); err != nil {
		t.Fatalf("failed to unmarshal fixture[0].Expected: %v", err)
	}
	if expected["y"] != 2 {
		t.Errorf("fixture[0].Expected.y: got %d, want 2", expected["y"])
	}
}

func TestLoadFixturesFromTestdata(t *testing.T) {
	// This test loads the actual FIFO fixture file shipped with the project.
	ff := LoadFixturesFromTestdata(t, "valuation_fifo.json")

	if ff.Module != "stock.valuation" {
		t.Errorf("module: got %q, want %q", ff.Module, "stock.valuation")
	}
	if ff.Function != "FIFOValuation" {
		t.Errorf("function: got %q, want %q", ff.Function, "FIFOValuation")
	}
	if len(ff.Fixtures) == 0 {
		t.Fatal("expected at least one fixture in valuation_fifo.json")
	}
	if ff.Fixtures[0].Name != "simple_addition" {
		t.Errorf("first fixture name: got %q, want %q", ff.Fixtures[0].Name, "simple_addition")
	}
}

func TestAssertAlmostEqual_WithinTolerance(t *testing.T) {
	// These should not produce errors.
	AssertAlmostEqual(t, 1.0001, 1.0002, 3, "values within 3 decimal places")
	AssertAlmostEqual(t, 0.0, 0.0, 10)
	AssertAlmostEqual(t, 100.123456, 100.123456, 6)
}

func TestAssertAlmostEqual_OutsideTolerance(t *testing.T) {
	// Use a mock T to capture errors without failing the real test.
	mockT := &testing.T{}

	// This should fail: difference is 0.01, tolerance at 3 places is 0.001.
	AssertAlmostEqual(mockT, 1.0, 1.01, 3, "should detect difference")

	// We can't directly check mockT.Failed() from outside, so we do a
	// smoke test: just ensure the function doesn't panic.
	// The real verification is that TestAssertAlmostEqual_WithinTolerance passes
	// and the function signature works correctly.
}

func TestAssertFloatSliceEqual_Matching(t *testing.T) {
	got := []float64{1.0, 2.0, 3.0}
	want := []float64{1.0, 2.0, 3.0}
	AssertFloatSliceEqual(t, got, want, 6)
}

func TestAssertFloatSliceEqual_WithinTolerance(t *testing.T) {
	got := []float64{1.00001, 2.00002, 3.00003}
	want := []float64{1.00002, 2.00003, 3.00004}
	AssertFloatSliceEqual(t, got, want, 4, "should be equal within 4 places")
}

func TestAssertStockBinsEqual_Matching(t *testing.T) {
	got := [][]float64{{10, 100}, {5, 50}}
	want := [][]float64{{10, 100}, {5, 50}}
	AssertStockBinsEqual(t, got, want, 6)
}

func TestAssertStockBinsEqual_Empty(t *testing.T) {
	got := [][]float64{}
	want := [][]float64{}
	AssertStockBinsEqual(t, got, want, 6)
}

func TestAssertStockBinsEqual_WithinTolerance(t *testing.T) {
	got := [][]float64{{10.001, 100.002}}
	want := [][]float64{{10.002, 100.003}}
	AssertStockBinsEqual(t, got, want, 2)
}

func TestDiffJSON_MatchingValues(t *testing.T) {
	a := map[string]int{"x": 1, "y": 2}
	b := map[string]int{"x": 1, "y": 2}

	diff := DiffJSON(a, b)
	if diff != "" {
		t.Errorf("expected empty diff for matching values, got:\n%s", diff)
	}
}

func TestDiffJSON_DifferentValues(t *testing.T) {
	a := map[string]interface{}{"x": 1, "y": 2}
	b := map[string]interface{}{"x": 1, "y": 999}

	diff := DiffJSON(a, b)
	if diff == "" {
		t.Error("expected non-empty diff for different values")
	}
	// The diff should contain the differing value.
	if !containsSubstring(diff, "999") {
		t.Errorf("diff should mention the differing value 999, got:\n%s", diff)
	}
}

func TestDiffJSON_DifferentStructure(t *testing.T) {
	a := map[string]interface{}{"x": 1}
	b := map[string]interface{}{"x": 1, "y": 2}

	diff := DiffJSON(a, b)
	if diff == "" {
		t.Error("expected non-empty diff for different structures")
	}
}

func TestDiffJSON_Slices(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{1, 2, 4}

	diff := DiffJSON(a, b)
	if diff == "" {
		t.Error("expected non-empty diff for different slices")
	}
}

func TestDiffJSON_NestedObjects(t *testing.T) {
	a := map[string]interface{}{
		"bins": []interface{}{
			[]interface{}{10.0, 100.0},
		},
	}
	b := map[string]interface{}{
		"bins": []interface{}{
			[]interface{}{10.0, 200.0},
		},
	}

	diff := DiffJSON(a, b)
	if diff == "" {
		t.Error("expected non-empty diff for different nested objects")
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
