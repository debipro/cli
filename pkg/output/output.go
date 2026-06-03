// Package output renders API responses to the terminal, either as raw JSON or
// as colorized, indented JSON for interactive use.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/fatih/color"
)

var (
	keyColor     = color.New(color.FgCyan).SprintFunc()
	stringColor  = color.New(color.FgGreen).SprintFunc()
	numberColor  = color.New(color.FgYellow).SprintFunc()
	literalColor = color.New(color.FgMagenta).SprintFunc()
)

// PrintJSON writes the given JSON body to w. When raw is true (or the body is
// not valid JSON), it is written verbatim. Otherwise it is indented, and
// colorized when colorEnabled is true.
func PrintJSON(w io.Writer, body []byte, raw, colorEnabled bool) error {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}

	if raw || !json.Valid(body) {
		_, err := fmt.Fprintln(w, string(body))
		return err
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		_, err := fmt.Fprintln(w, string(body))
		return err
	}

	out := buf.String()
	if colorEnabled {
		out = colorize(out)
	}
	_, err := fmt.Fprintln(w, out)
	return err
}

// colorize applies ANSI colors to an already-indented JSON document, preserving
// the original key ordering and whitespace.
func colorize(s string) string {
	var b strings.Builder
	runes := []rune(s)
	n := len(runes)

	for i := 0; i < n; i++ {
		ch := runes[i]
		switch {
		case ch == '"':
			str, next := readString(runes, i)
			if isKey(runes, next) {
				b.WriteString(keyColor(str))
			} else {
				b.WriteString(stringColor(str))
			}
			i = next - 1
		case ch == '-' || unicode.IsDigit(ch):
			tok, next := readToken(runes, i)
			b.WriteString(numberColor(tok))
			i = next - 1
		case unicode.IsLetter(ch):
			tok, next := readToken(runes, i)
			switch tok {
			case "true", "false", "null":
				b.WriteString(literalColor(tok))
			default:
				b.WriteString(tok)
			}
			i = next - 1
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// readString returns the full quoted string starting at index i (which points
// at the opening quote) and the index just past the closing quote.
func readString(runes []rune, i int) (string, int) {
	j := i + 1
	for j < len(runes) {
		if runes[j] == '\\' {
			j += 2
			continue
		}
		if runes[j] == '"' {
			j++
			break
		}
		j++
	}
	return string(runes[i:j]), j
}

// readToken reads a run of non-structural characters (a number or literal).
func readToken(runes []rune, i int) (string, int) {
	j := i
	for j < len(runes) {
		ch := runes[j]
		if ch == ',' || ch == '}' || ch == ']' || ch == ':' || unicode.IsSpace(ch) {
			break
		}
		j++
	}
	return string(runes[i:j]), j
}

// isKey reports whether the next non-space character at or after index i is a
// colon, meaning the preceding string was an object key.
func isKey(runes []rune, i int) bool {
	for i < len(runes) {
		if unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		return runes[i] == ':'
	}
	return false
}
