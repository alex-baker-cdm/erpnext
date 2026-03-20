package stock

import (
	"testing"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"float64", float64(42.5), 42.5},
		{"float32", float32(3.14), 3.140000104904175},
		{"int64", int64(100), 100.0},
		{"int", int(7), 7.0},
		{"string", "123.45", 123.45},
		{"bytes", []byte("99.9"), 99.9},
		{"nil", nil, 0.0},
		{"bool", true, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toFloat64(tt.in)
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"bytes", []byte("world"), "world"},
		{"nil", nil, ""},
		{"int", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToString(tt.in)
			if got != tt.want {
				t.Errorf("ToString(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"yes", true},
		{"anything", true},
		{"0", false},
		{"false", false},
		{"False", false},
		{"no", false},
		{"none", false},
		{"None", false},
		{"", false},
		{"  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := ToBool(tt.in)
			if got != tt.want {
				t.Errorf("ToBool(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
