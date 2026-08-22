package matcher

import (
	"testing"
)

func TestPoolMatcher(t *testing.T) {
	m := NewPoolMatcher(nil)

	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		{
			input:       "Schwimm in Bilk",
			expected:    "Schwimm' in Bilk Düsseldorf",
			expectError: false,
		},
		{
			input:       "Schwimm' in Bilk Düsseldorf",
			expected:    "Schwimm' in Bilk Düsseldorf",
			expectError: false,
		},
		{
			input:       "Münster Therme",
			expected:    "Münster Therme Düsseldorf",
			expectError: false,
		},
		{
			input:       "Freizeitbad Düsselstrand",
			expected:    "Freizeitbad Düsselstrand Düsseldorf",
			expectError: false,
		},
		{
			input:       "Badehaus Benrath",
			expected:    "Badehaus Benrath Düsseldorf",
			expectError: false,
		},
		{
			input:       "Allwetterbad Flingern Hallenbad",
			expected:    "Allwetterbad Flingern (Hallenbad) Düsseldorf",
			expectError: false,
		},
		{
			input:       "Rheinbad Düsseldorf",
			expected:    "Hallen- und Freibad Rheinbad Düsseldorf",
			expectError: false,
		},
		{
			input:       "ABC Nesselwang",
			expected:    "ABC Nesselwang",
			expectError: false,
		},
		{
			input:       "Nonexistent Pool 123456789",
			expectError: true,
		},
		{
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := m.Match(tt.input)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error for input %q, got match: %q", tt.input, res.MatchedLabel)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tt.input, err)
			}
			if res.MatchedLabel != tt.expected {
				t.Errorf("input %q: got %q, want %q (confidence: %.2f)", tt.input, res.MatchedLabel, tt.expected, res.Confidence)
			}
		})
	}
}
