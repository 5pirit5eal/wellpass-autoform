package extractor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ReceiptMetadata represents the structured extraction result from Gemini.
type ReceiptMetadata struct {
	Date          string  `json:"date"`           // Format: YYYY-MM-DD
	TicketPrice   float64 `json:"ticket_price"`   // Price in EUR (e.g. 5.44)
	Currency      string  `json:"currency"`       // Currency code, e.g. "EUR"
	Location      string  `json:"location"`       // Swimming pool name, e.g. "Schwimm in Bilk"
	ReceiptNumber string  `json:"receipt_number"` // Receipt or invoice number, e.g. "R-1091126"
	TicketType    string  `json:"ticket_type,omitempty"`
}

// Normalize ensures dates, currency, and string fields are clean and standardized.
func (m *ReceiptMetadata) Normalize() {
	m.Location = strings.TrimSpace(m.Location)
	m.ReceiptNumber = strings.TrimSpace(m.ReceiptNumber)
	m.TicketType = strings.TrimSpace(m.TicketType)

	if m.Currency == "" {
		m.Currency = "EUR"
	} else {
		m.Currency = strings.ToUpper(strings.TrimSpace(m.Currency))
	}

	// Normalize date if it came in German format (DD.MM.YYYY)
	m.Date = normalizeDate(strings.TrimSpace(m.Date))
}

var (
	isoDateRegex    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	germanDateRegex = regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})`)
)

func normalizeDate(d string) string {
	if isoDateRegex.MatchString(d) {
		return d
	}
	if matches := germanDateRegex.FindStringSubmatch(d); len(matches) >= 4 {
		day := fmt.Sprintf("%02s", matches[1])
		month := fmt.Sprintf("%02s", matches[2])
		year := matches[3]
		return fmt.Sprintf("%s-%s-%s", year, month, day)
	}
	// Try parsing standard formats
	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"02.01.2006 15:04:05",
		"02/01/2006",
		"2006/01/02",
		time.RFC3339,
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, d); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return d
}

// Validate checks that required fields (Date, TicketPrice, Location) are present and valid.
func (m *ReceiptMetadata) Validate() error {
	m.Normalize()

	if m.Date == "" {
		return fmt.Errorf("receipt date is missing")
	}
	if !isoDateRegex.MatchString(m.Date) {
		return fmt.Errorf("invalid receipt date format: %q (expected YYYY-MM-DD)", m.Date)
	}
	if m.TicketPrice <= 0 {
		return fmt.Errorf("invalid ticket price: %.2f (must be > 0)", m.TicketPrice)
	}
	if m.Location == "" {
		return fmt.Errorf("pool location is missing")
	}

	return nil
}

// ToMetadataMap converts the structured receipt metadata to a string map for GCS custom metadata.
func (m *ReceiptMetadata) ToMetadataMap() map[string]string {
	m.Normalize()
	meta := map[string]string{
		"date":         m.Date,
		"ticket_price": fmt.Sprintf("%.2f", m.TicketPrice),
		"currency":     m.Currency,
		"location":     m.Location,
	}

	if m.ReceiptNumber != "" {
		meta["receipt_number"] = m.ReceiptNumber
	}
	if m.TicketType != "" {
		meta["ticket_type"] = m.TicketType
	}

	if rawJSON, err := json.Marshal(m); err == nil {
		meta["structured_data"] = string(rawJSON)
	}

	return meta
}
