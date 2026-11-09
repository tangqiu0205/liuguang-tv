package app

import (
	"os"
	"strings"

	"github.com/egoist/mygo/i18n"
)

// MyGo v0.8.3 exposes the resolved locale but not the saved override. Read
// its existing preference for presentation only; SetLocale remains its writer.
func readLanguagePreference(dataDir string) (string, error) {
	tag, err := savedLanguagePreference(dataDir)
	if err != nil {
		return "", err
	}
	tag = i18n.Normalize(strings.TrimSpace(tag))
	if tag == "" {
		return "", nil
	}
	return i18n.Match([]string{tag}, []string{"zh-Hans", "en"}), nil
}

func readLanguagePreferenceFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}
