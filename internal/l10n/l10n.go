// Package l10n translates application-owned messages. Simplified Chinese is
// the source language; source text is the stable message ID, as in gettext.
// Never pass channel names, URLs, program titles or other user data to T.
package l10n

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/i18n"
)

//go:embed en.json
var englishJSON []byte

var english = func() map[string]string {
	var messages map[string]string
	if err := json.Unmarshal(englishJSON, &messages); err != nil {
		panic("l10n: invalid embedded English catalog: " + err.Error())
	}
	return messages
}()

// T translates a source message in MyGo's current application language.
// Format strings are translated before interpolation, never afterwards.
func T(source string) string { return In(i18n.Locale(), source) }

// In translates in a specific language without changing application state.
// Unsupported languages and missing translations retain the source message.
func In(tag, source string) string {
	if tag == "en" || strings.HasPrefix(tag, "en-") {
		if translated, ok := english[source]; ok {
			return translated
		}
	}
	return source
}

// Matches recognizes a complete application message in either language.
// Background completion must not depend on the language selected meanwhile.
func Matches(text, source string) bool {
	return text == source || text == In("en", source)
}
