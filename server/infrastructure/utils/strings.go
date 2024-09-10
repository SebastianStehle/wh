package utils

import (
	"unicode"
	"unicode/utf8"
)

func LessLower(sa string, sb string) bool {
	for {
		rb, nb := utf8.DecodeRuneInString(sb)
		if nb == 0 {
			// The number of runes in sa is greater than or
			// equal to the number of runes in sb. It follows
			// that sa is not less than sb.
			return false
		}

		ra, na := utf8.DecodeRuneInString(sa)
		if na == 0 {
			// The number of runes in sa is less than the
			// number of runes in sb. It follows that sa
			// is less than sb.
			return true
		}

		rb = unicode.ToLower(rb)
		ra = unicode.ToLower(ra)

		if ra != rb {
			return ra < rb
		}

		// Trim rune from the beginning of each string.
		sa = sa[na:]
		sb = sb[nb:]
	}
}
