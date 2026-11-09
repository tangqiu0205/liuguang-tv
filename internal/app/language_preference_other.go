//go:build !darwin

package app

import "path/filepath"

func savedLanguagePreference(dataDir string) (string, error) {
	return readLanguagePreferenceFile(filepath.Join(dataDir, "locale"))
}
