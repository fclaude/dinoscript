//go:build !darwin && !windows

package locale

// System returns the supported language matching the user's display language.
func System() Language { return environmentLanguage() }
