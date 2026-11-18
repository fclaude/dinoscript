package locale

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// System uses the first macOS preferred language, independently of the region.
func System() Language {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/defaults", "read", "-g", "AppleLanguages").Output()
	if err == nil {
		tag, _, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "(")), ",")
		if tag = strings.Trim(tag, " \n\r\t\")"); tag != "" {
			return fromTag(tag)
		}
	}
	return environmentLanguage()
}
