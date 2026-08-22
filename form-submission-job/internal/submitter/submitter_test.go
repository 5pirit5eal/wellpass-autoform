package submitter

import (
	"testing"
)

func TestExtractSearchTerm(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Schwimm' in Bilk Düsseldorf", "Schwimm in"},
		{"Münster Therme Düsseldorf", "Münster Therme"},
		{"ABC Nesselwang", "ABC Nesselwang"},
	}

	for _, tt := range tests {
		got := extractSearchTerm(tt.input)
		if got != tt.want {
			t.Errorf("extractSearchTerm(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
