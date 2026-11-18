// Package locale identifies the languages Dinoscript bundles.
package locale

import (
	"os"
	"strings"
)

type Language string

const (
	English Language = "en"
	Spanish Language = "es-CL"
)

var supported = []struct {
	language Language
	name     string
}{
	{English, "English"},
	{Spanish, "Español"},
}

func Languages() []Language {
	languages := make([]Language, 0, len(supported))
	for _, item := range supported {
		languages = append(languages, item.language)
	}
	return languages
}

func (language Language) Name() string {
	for _, item := range supported {
		if item.language == language {
			return item.name
		}
	}
	return ""
}

func fromTag(tag string) Language {
	primary := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(primary, "-_.@"); i >= 0 {
		primary = primary[:i]
	}
	for _, item := range supported {
		language, _, _ := strings.Cut(string(item.language), "-")
		if primary == language {
			return item.language
		}
	}
	return English
}

func environmentLanguage() Language {
	for _, key := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		for tag := range strings.SplitSeq(os.Getenv(key), ":") {
			if strings.TrimSpace(tag) != "" {
				return fromTag(tag)
			}
		}
	}
	return English
}
