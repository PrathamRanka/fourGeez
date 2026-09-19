package catalog

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/fourgeez/agentpay/internal/domain"
)

const (
	MaximumPublicDirectorySearchTerms = 16
	MaximumPublicDirectoryQueryTerms  = 4
	MinimumPublicDirectoryTermLength  = 2
	MaximumPublicDirectoryTermLength  = 32
)

// PublicDirectoryProjection is the bounded non-authoritative index record for one published route.
type PublicDirectoryProjection struct {
	SellerID     domain.ID        `json:"sellerId"`
	RouteID      domain.ID        `json:"routeId"`
	DisplaySort  string           `json:"displaySort"`
	SearchTerms  []string         `json:"searchTerms"`
	RouteVersion uint64           `json:"routeVersion"`
	UpdatedAt    domain.Timestamp `json:"updatedAt"`
}

// SortKey returns the stable lexical ordering key shared by persistence adapters.
func (projection PublicDirectoryProjection) SortKey() string {
	return "PRODUCT#" + projection.DisplaySort + "#" + projection.SellerID.String() + "#" + projection.RouteID.String()
}

// PublicDirectoryQuery selects one stable lexical candidate page.
type PublicDirectoryQuery struct {
	SearchTerm string
	After      string
	Limit      int
}

// PublicDirectoryCandidate includes the persistence sort key used for opaque pagination.
type PublicDirectoryCandidate struct {
	Projection PublicDirectoryProjection
	SortKey    string
}

// PublicDirectoryCandidatePage is one bounded query result from the maintained projection.
type PublicDirectoryCandidatePage struct {
	Items     []PublicDirectoryCandidate
	Exhausted bool
}

// PublicDirectoryRepository queries the maintained public discovery projection.
type PublicDirectoryRepository interface {
	GetPublicDirectoryRoute(context.Context, domain.ID, domain.ID) (PaidRoute, error)
	ListPublicDirectoryCandidates(context.Context, PublicDirectoryQuery) (PublicDirectoryCandidatePage, error)
}

// NewPublicDirectoryProjection returns a projection only for a currently published route snapshot.
func NewPublicDirectoryProjection(route PaidRoute) (PublicDirectoryProjection, bool) {
	route.normalizeLegacyFields()
	if route.LifecycleStatus != RouteLifecyclePublished || !route.Enabled {
		return PublicDirectoryProjection{}, false
	}
	return PublicDirectoryProjection{
		SellerID:     route.SellerID,
		RouteID:      route.RouteID,
		DisplaySort:  normalizeDirectorySort(route.DisplayName),
		SearchTerms:  publicDirectoryTerms(route.DisplayName + " " + route.Description),
		RouteVersion: route.Version,
		UpdatedAt:    route.UpdatedAt,
	}, true
}

// NormalizePublicDirectoryQuery validates one exact-term public search query.
func NormalizePublicDirectoryQuery(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	runs := directoryTermRuns(raw)
	if len(runs) == 0 || len(runs) > MaximumPublicDirectoryQueryTerms {
		return nil, domain.NewValidationError("q", "terms", "search must contain one to four terms")
	}
	terms := make([]string, 0, len(runs))
	seen := make(map[string]struct{}, len(runs))
	for _, term := range runs {
		length := utf8.RuneCountInString(term)
		if length < MinimumPublicDirectoryTermLength || length > MaximumPublicDirectoryTermLength {
			return nil, domain.NewValidationError("q", "term_length", "search terms must contain 2 to 32 letters or digits")
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms, nil
}

func publicDirectoryTerms(raw string) []string {
	terms := make([]string, 0, MaximumPublicDirectorySearchTerms)
	seen := make(map[string]struct{}, MaximumPublicDirectorySearchTerms)
	for _, term := range directoryTermRuns(raw) {
		length := utf8.RuneCountInString(term)
		if length < MinimumPublicDirectoryTermLength || length > MaximumPublicDirectoryTermLength {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) == MaximumPublicDirectorySearchTerms {
			break
		}
	}
	return terms
}

func directoryTermRuns(raw string) []string {
	terms := make([]string, 0)
	current := make([]rune, 0)
	appendTerm := func() {
		if len(current) == 0 {
			return
		}
		terms = append(terms, string(current))
		current = current[:0]
	}
	for _, character := range strings.ToLower(raw) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			current = append(current, character)
			continue
		}
		appendTerm()
	}
	appendTerm()
	return terms
}

func normalizeDirectorySort(displayName string) string {
	terms := publicDirectoryTerms(displayName)
	if len(terms) == 0 {
		return "product"
	}
	return strings.Join(terms, "-")
}
