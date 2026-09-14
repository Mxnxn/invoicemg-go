package textcase

import "testing"

// Every expected value in this table was PRINTED by the Node helper, not reasoned out:
//
//	node -e 'const {docNumber,titleCase,sentenceCase} = require("./Helpers/TextCase"); ...'
//
// against invoice-mg-api at the commit this package was written. That matters more than usual
// here - these are Mongoose setters applied on write, so a Go route that disagrees stores text
// differently from a Node route into the SAME collection while both services are live. Nothing
// errors; the list just looks wrong and a lookup by number misses.
//
// If a case here ever needs changing, change the Node helper first and re-capture the output.
func TestMatchesTheNodeHelper(t *testing.T) {
	cases := []struct {
		in, doc, title, sentence string
	}{
		{"  mg/26-27/qt-1 ", "MG/26-27/QT-1", "Mg/26-27/qt-1", "Mg/26-27/qt-1"},
		{"24abcde1234f1z5", "24ABCDE1234F1Z5", "24abcde1234f1z5", "24abcde1234f1z5"},
		{"", "", "", ""},
		{"JOHN", "JOHN", "John", "JOHN"},
		{"john", "JOHN", "John", "John"},
		{"jOhN", "JOHN", "John", "JOhN"},

		// Deliberate casing survives: a brand spelled this way was spelled this way on purpose.
		{"InvoiceMG", "INVOICEMG", "InvoiceMG", "InvoiceMG"},
		{"BlackBack", "BLACKBACK", "BlackBack", "BlackBack"},

		// Hyphens and apostrophes are word boundaries, including the typographic apostrophe a
		// phone keyboard produces.
		{"jean-pierre", "JEAN-PIERRE", "Jean-Pierre", "Jean-pierre"},
		{"o'brien", "O'BRIEN", "O'Brien", "O'brien"},
		{"o’brien", "O’BRIEN", "O’Brien", "O’brien"},

		// titleCase collapses runs of whitespace; docNumber and sentenceCase do not.
		{"  acme   signs  pvt ltd ", "ACME   SIGNS  PVT LTD", "Acme Signs Pvt Ltd", "Acme   signs  pvt ltd"},

		// A lower-case-initial brand IS normalised - the documented trade-off for keeping the
		// rule predictable.
		{"iPhone", "IPHONE", "Iphone", "IPhone"},

		// All-caps is input to tidy, not intent to preserve.
		{"MG", "MG", "Mg", "MG"},
		{"a", "A", "A", "A"},

		// Prose keeps its acronyms under sentenceCase and loses them under titleCase - which is
		// exactly why descriptions use one and names the other.
		{
			"call customer before 5pm about THE rate change",
			"CALL CUSTOMER BEFORE 5PM ABOUT THE RATE CHANGE",
			"Call Customer Before 5pm About The Rate Change",
			"Call customer before 5pm about THE rate change",
		},
		{"  hello world  ", "HELLO WORLD", "Hello World", "Hello world"},

		// The one that nearly went wrong. Node's titleCase regex is [a-z], which does not match
		// é, so the accented word is left alone - while sentenceCase, using toUpperCase(), does
		// capitalise it. An implementation that "correctly" handled Unicode in both would
		// disagree with Node on the first customer with an accent in their name.
		{"élan vital", "ÉLAN VITAL", "élan Vital", "Élan vital"},

		{"3m tapes", "3M TAPES", "3m Tapes", "3m tapes"},
	}

	for _, c := range cases {
		if got := DocNumber(c.in); got != c.doc {
			t.Errorf("DocNumber(%q) = %q, node says %q", c.in, got, c.doc)
		}
		if got := TitleCase(c.in); got != c.title {
			t.Errorf("TitleCase(%q) = %q, node says %q", c.in, got, c.title)
		}
		if got := SentenceCase(c.in); got != c.sentence {
			t.Errorf("SentenceCase(%q) = %q, node says %q", c.in, got, c.sentence)
		}
	}
}
