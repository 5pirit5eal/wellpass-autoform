package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wellpass-autoform/receipts-function/internal/config"
	genai "google.golang.org/genai"
)

// ReceiptExtractor defines the contract for extracting structured metadata from receipt files.
type ReceiptExtractor interface {
	ExtractReceipt(ctx context.Context, fileBytes []byte, mimeType string) (*ReceiptMetadata, error)
}

// GeminiExtractor implements ReceiptExtractor using the Google GenAI SDK and Gemini models.
type GeminiExtractor struct {
	client *genai.Client
	model  string
}

// NewGeminiExtractor creates a new GeminiExtractor with the specified configuration.
func NewGeminiExtractor(ctx context.Context, cfg *config.Config) (*GeminiExtractor, error) {
	clientConfig := &genai.ClientConfig{}

	if cfg.UseVertexAI {
		clientConfig.Backend = genai.BackendVertexAI
		clientConfig.Project = cfg.ProjectID
		clientConfig.Location = cfg.GeminiLocation
	} else if cfg.GeminiAPIKey != "" {
		clientConfig.Backend = genai.BackendGeminiAPI
		clientConfig.APIKey = cfg.GeminiAPIKey
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	model := cfg.GeminiModel
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}

	return &GeminiExtractor{
		client: client,
		model:  model,
	}, nil
}

// NewGeminiExtractorWithClient creates an extractor using a pre-configured GenAI client.
func NewGeminiExtractorWithClient(client *genai.Client, model string) *GeminiExtractor {
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	return &GeminiExtractor{
		client: client,
		model:  model,
	}
}

const systemInstruction = `You are a precise receipt data extractor for swimming pool and sports facility receipts in Germany (e.g. Bädergesellschaft Düsseldorf, Schwimm in Bilk, Freibad Allwetterbad Flingern, Münster-Therme, Rheinbad, etc.).

Analyze the provided receipt (PDF or text/image) and extract the required fields:
- date: The date of the ticket purchase or visit formatted strictly as YYYY-MM-DD (e.g., 2026-08-07).
- ticket_price: The total price paid for the ticket/admission as a decimal number in EUR (e.g., 5.44). If discounts or customer credits were used, extract the actual ticket value / amount charged.
- currency: Currency code (e.g., "EUR").
- location: The specific name and location of the swimming pool or facility (e.g., "Schwimm in Bilk", "Freibad Allwetterbad Flingern", "Münster-Therme").
- receipt_number: The receipt number, invoice number (Rechnung Nr.), order number (Bestellung), reservation number, or POS transaction ID if present.
- ticket_type: The description of the ticket (e.g., "Erwachsene 1", "1 x Erwachsene Bad BgA DPS").`

// ExtractReceipt calls Gemini with structured output schema to extract receipt metadata.
func (g *GeminiExtractor) ExtractReceipt(ctx context.Context, fileBytes []byte, mimeType string) (*ReceiptMetadata, error) {
	if len(fileBytes) == 0 {
		return nil, fmt.Errorf("empty file data provided")
	}

	// Normalize mime type
	normalizedMime := strings.ToLower(strings.TrimSpace(mimeType))
	if normalizedMime == "" || normalizedMime == "application/octet-stream" {
		if isPDF(fileBytes) {
			normalizedMime = "application/pdf"
		} else {
			normalizedMime = "text/plain"
		}
	}

	// Build the structured output schema
	schema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"date": {
				Type:        genai.TypeString,
				Description: "Date of the receipt or visit in YYYY-MM-DD format (e.g. 2026-08-07)",
			},
			"ticket_price": {
				Type:        genai.TypeNumber,
				Description: "Total price paid for the ticket/admission in EUR as a number (e.g. 5.44)",
			},
			"currency": {
				Type:        genai.TypeString,
				Description: "Currency code, usually EUR",
			},
			"location": {
				Type:        genai.TypeString,
				Description: "Name and location of the swimming pool or facility (e.g. 'Schwimm in Bilk', 'Freibad Allwetterbad Flingern')",
			},
			"receipt_number": {
				Type:        genai.TypeString,
				Description: "Receipt, invoice, booking, or order number",
			},
			"ticket_type": {
				Type:        genai.TypeString,
				Description: "Type or description of the ticket",
			},
		},
		Required: []string{"date", "ticket_price", "location"},
	}

	genConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{
				genai.NewPartFromText(systemInstruction),
			},
			Role: genai.RoleUser,
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
	}

	// Pass file content part and user prompt
	parts := []*genai.Part{
		{
			InlineData: &genai.Blob{
				MIMEType: normalizedMime,
				Data:     fileBytes,
			},
		},
		genai.NewPartFromText("Extract the structured receipt details from this receipt document."),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, genConfig)
	if err != nil {
		return nil, fmt.Errorf("gemini content generation failed: %w", err)
	}

	responseText := strings.TrimSpace(resp.Text())
	if responseText == "" {
		return nil, fmt.Errorf("gemini returned empty response")
	}

	var metadata ReceiptMetadata
	if err := json.Unmarshal([]byte(responseText), &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse structured JSON from Gemini response: %w (raw response: %s)", err, responseText)
	}

	if err := metadata.Validate(); err != nil {
		return nil, fmt.Errorf("extracted metadata validation failed: %w", err)
	}

	return &metadata, nil
}

func isPDF(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == "%PDF"
}
