package polymarket

import "strings"

// Question words carry no signal and actively mislead the search.
//
// "win" is deliberately NOT here. Market titles are full of it — the target
// of the very query that prompted this is "ballon-dor-winner-2026" — so it is
// signal rather than noise, and a live search for "win ballon dor" returns
// that market first.
//
// Gamma's search matches on phrases, so "who will win the ballon dor?" scores
// highly against every "Who will win ..." market on the site and returns a
// Rhode Island mayoral race and Dancing With The Stars, while the actual
// Ballon d'Or market does not appear at all. Searching "ballon dor" returns
// it first.
//
// People ask questions. The raw question is a bad query, so the distinctive
// terms are what get sent.
var stopWords = map[string]bool{
	"a": true, "about": true, "an": true, "and": true, "are": true, "at": true,
	"be": true, "become": true, "by": true,
	"can": true, "could": true,
	"did": true, "do": true, "does": true,
	"for": true, "from": true,
	"going": true, "gonna": true,
	"happen": true, "happening": true, "has": true, "have": true, "how": true,
	"in": true, "is": true, "it": true, "its": true,
	"me": true, "moving": true, "my": true,
	"of": true, "on": true, "or": true,
	"shall": true, "should": true,
	"tell": true, "that": true, "the": true, "there": true, "this": true,
	"to":  true,
	"was": true, "were": true, "what": true, "when": true, "where": true,
	"which": true, "who": true, "whom": true, "why": true, "will": true,
	"with": true, "would": true,
	"you": true, "your": true,
}

// normaliseQuery reduces a natural-language request to its distinctive terms.
//
// Conservative on purpose: if stripping stop words would leave nothing, the
// original text is returned rather than an empty search. A query made
// entirely of common words is unlikely to match anything useful either way,
// and returning nothing at all is a worse answer than returning a poor one.
func normaliseQuery(query string) string {
	fields := strings.Fields(strings.ToLower(query))

	kept := make([]string, 0, len(fields))
	for _, field := range fields {
		// Trim punctuation that rides along with words: "dor?" must match
		// "dor". Hyphens are preserved, because a slug pasted into a question
		// is one of the most useful things someone can type.
		word := strings.Trim(field, "?!.,;:'\"()[]{}")
		if word == "" || stopWords[word] {
			continue
		}
		kept = append(kept, word)
	}

	if len(kept) == 0 {
		return strings.TrimSpace(query)
	}
	return strings.Join(kept, " ")
}
