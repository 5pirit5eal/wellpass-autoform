package matcher

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// MatchResult holds the matched pool choice, confidence score, and original input.
type MatchResult struct {
	MatchedLabel string  `json:"matched_label"`
	Confidence   float64 `json:"confidence"`
	Query        string  `json:"query"`
}

// PoolMatcher matches raw extracted swimming pool names to exact Typeform dropdown choices.
type PoolMatcher struct {
	choices []string
	aliases map[string]string
}

// NewPoolMatcher creates a new PoolMatcher using default embedded choices and aliases.
func NewPoolMatcher(customChoices []string) *PoolMatcher {
	choices := PoolChoices
	if len(customChoices) > 0 {
		choices = customChoices
	}
	return &PoolMatcher{
		choices: choices,
		aliases: KnownAliases,
	}
}

// Match finds the best matching Typeform pool option for a given raw location name.
func (m *PoolMatcher) Match(rawLocation string) (*MatchResult, error) {
	trimmed := strings.TrimSpace(rawLocation)
	if trimmed == "" {
		return nil, fmt.Errorf("empty pool location query")
	}

	normQuery := normalize(trimmed)

	// 1. Direct check against aliases
	if aliasMatch, ok := m.aliases[normQuery]; ok {
		return &MatchResult{
			MatchedLabel: aliasMatch,
			Confidence:   1.0,
			Query:        trimmed,
		}, nil
	}

	// Also check if normalized alias is substring of query or vice versa
	for alias, mapped := range m.aliases {
		if strings.Contains(normQuery, alias) || strings.Contains(alias, normQuery) {
			return &MatchResult{
				MatchedLabel: mapped,
				Confidence:   0.98,
				Query:        trimmed,
			}, nil
		}
	}

	// 2. Exact normalized check against all pool choices
	for _, choice := range m.choices {
		if normalize(choice) == normQuery {
			return &MatchResult{
				MatchedLabel: choice,
				Confidence:   1.0,
				Query:        trimmed,
			}, nil
		}
	}

	// 3. Substring / Token matching
	queryTokens := tokenize(normQuery)
	var bestChoice string
	var bestScore float64

	for _, choice := range m.choices {
		normChoice := normalize(choice)

		// Full containment
		if strings.Contains(normChoice, normQuery) || strings.Contains(normQuery, normChoice) {
			score := 0.90 + (float64(len(normQuery))/float64(len(normChoice)+1))*0.05
			if score > bestScore {
				bestScore = score
				bestChoice = choice
			}
			continue
		}

		// Token overlap score
		choiceTokens := tokenize(normChoice)
		score := calculateTokenSimilarity(queryTokens, choiceTokens)

		// Bigram similarity
		bigramScore := bigramSimilarity(normQuery, normChoice)
		combinedScore := (score * 0.6) + (bigramScore * 0.4)

		if combinedScore > bestScore {
			bestScore = combinedScore
			bestChoice = choice
		}
	}

	if bestScore >= 0.50 && bestChoice != "" {
		return &MatchResult{
			MatchedLabel: bestChoice,
			Confidence:   bestScore,
			Query:        trimmed,
		}, nil
	}

	return nil, fmt.Errorf("could not match pool location %q (best match was %q with score %.2f)", trimmed, bestChoice, bestScore)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9äöüÄÖÜß\s]`)

func normalize(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

func tokenize(s string) []string {
	fields := strings.Fields(s)
	var tokens []string
	// filter out very short stop-words
	for _, f := range fields {
		if len(f) > 1 {
			tokens = append(tokens, f)
		}
	}
	return tokens
}

func calculateTokenSimilarity(queryTokens, choiceTokens []string) float64 {
	if len(queryTokens) == 0 || len(choiceTokens) == 0 {
		return 0
	}

	matched := 0
	for _, qt := range queryTokens {
		for _, ct := range choiceTokens {
			if qt == ct || strings.Contains(ct, qt) || strings.Contains(qt, ct) {
				matched++
				break
			}
		}
	}

	return float64(matched) / float64(len(queryTokens))
}

func bigramSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	b1 := getBigrams(s1)
	b2 := getBigrams(s2)
	if len(b1) == 0 || len(b2) == 0 {
		return 0.0
	}

	intersection := 0
	counts := make(map[string]int)
	for _, bg := range b1 {
		counts[bg]++
	}
	for _, bg := range b2 {
		if counts[bg] > 0 {
			intersection++
			counts[bg]--
		}
	}

	return 2.0 * float64(intersection) / float64(len(b1)+len(b2))
}

func getBigrams(s string) []string {
	runes := []rune(s)
	if len(runes) < 2 {
		return nil
	}
	var bigrams []string
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsSpace(runes[i]) && unicode.IsSpace(runes[i+1]) {
			continue
		}
		bigrams = append(bigrams, string(runes[i:i+2]))
	}
	return bigrams
}
