// Package textcase is Helpers/TextCase.js, in Go.
//
// These are Mongoose SETTERS in the Node app - applied on write, on every path, across 13
// models. That placement is deliberate there and matters more here: while both services are
// live they write into the same collections, so a Go route that skips these rules stores
// "mg/26-27/qt-1" into a column where every Node-written row reads "MG/26-27/QT-1". Nothing
// errors. The list just looks wrong, sorts wrong, and a lookup by number misses.
//
// So these belong in the store layer, applied on the way to the database - not in handlers,
// where the next route to be written is the one that forgets.
package textcase

import (
	"regexp"
	"strings"
	"unicode"
)

// DocNumber is for identifiers: GST numbers, invoice and quotation numbers, job-ids.
// Uppercase and trimmed. Inner punctuation (MG/26-27/00001, 24ABCDE1234F1Z5) is left exactly
// as typed - only the case changes.
func DocNumber(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

var spaces = regexp.MustCompile(`\s+`)

// isDeliberateCasing reports whether a word was spelled a particular way on purpose.
//
// "InvoiceMG" and "BlackBack" start with a capital and carry another one later; lowercasing
// the tail would rewrite a brand into something nobody typed. "JOHN" is all caps and "jOhN"
// starts lower - both are input to be tidied, not intent to preserve.
func isDeliberateCasing(word string) bool {
	if word == "" {
		return false
	}
	runes := []rune(word)
	if !unicode.IsUpper(runes[0]) {
		return false
	}
	hasUpperInTail := false
	for _, r := range runes[1:] {
		if unicode.IsUpper(r) {
			hasUpperInTail = true
			break
		}
	}
	return hasUpperInTail && word != strings.ToUpper(word)
}

// TitleCase capitalises the first letter of each word - names of people, firms, suppliers,
// employees, products.
//
// Capitalises after hyphens and apostrophes too, so "jean-pierre" and "o'brien" come out right
// rather than as "Jean-pierre" and "O'brien". Both apostrophes are handled: the typewriter '
// and the typographic U+2019, because a phone keyboard produces the second one and the Node
// regex matches both.
func TitleCase(value string) string {
	trimmed := spaces.ReplaceAllString(strings.TrimSpace(value), " ")
	if trimmed == "" {
		return trimmed
	}

	words := strings.Split(trimmed, " ")
	for i, word := range words {
		if isDeliberateCasing(word) {
			continue
		}
		words[i] = capitaliseSegments(strings.ToLower(word))
	}
	return strings.Join(words, " ")
}

// capitaliseSegments upper-cases the first letter of the word and of anything following a
// hyphen or apostrophe, matching /(^|[\-'’])([a-z])/g in the Node helper.
//
// ASCII a-z ONLY, deliberately. The JavaScript character class is [a-z], which does not match
// an accented letter - so Node title-cases "élan vital" to "élan Vital", leaving the é alone.
// Using unicode.IsLower here instead would produce "Élan Vital", and a customer named Élan
// would be stored one way by Node and another by Go in the same column. Matching a quirk is
// the job; improving on it is how two live services drift apart.
//
// SentenceCase is NOT restricted this way: it uses charAt(0).toUpperCase() in Node, which is
// fully Unicode-aware, so "élan vital" does become "Élan vital" there. Two genuinely different
// rules - that difference lives in the Node helper, it is not an oversight here.
func capitaliseSegments(word string) string {
	out := make([]rune, 0, len(word))
	atBoundary := true
	for _, r := range word {
		if atBoundary && r >= 'a' && r <= 'z' {
			out = append(out, unicode.ToUpper(r))
		} else {
			out = append(out, r)
		}
		atBoundary = r == '-' || r == '\'' || r == '’'
	}
	return string(out)
}

// SentenceCase upper-cases the first letter of the whole string and nothing else - a note may
// contain a product code or an acronym that must survive as typed. Title-casing prose gives
// "Call Customer Before 5pm About The Rate Change", which reads as broken rather than tidy.
func SentenceCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	runes := []rune(trimmed)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
