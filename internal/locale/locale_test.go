package locale

import "testing"

func TestLanguageTags(t *testing.T) {
	for tag, want := range map[string]Language{
		"es": Spanish, "es-CL": Spanish, "es-MX": Spanish, "es_CL.UTF-8": Spanish, "es_ES@modern": Spanish,
		"en": English, "en_GB.UTF-8": English, "pl-PL": English, "C.UTF-8": English, "": English,
	} {
		if got := fromTag(tag); got != want {
			t.Errorf("fromTag(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestEnvironmentLanguage(t *testing.T) {
	t.Setenv("LANGUAGE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "es_CL.UTF-8")
	t.Setenv("LANG", "en_US.UTF-8")
	if got := environmentLanguage(); got != Spanish {
		t.Errorf("Spanish message locale gave %q", got)
	}
	t.Setenv("LANGUAGE", "fr_FR:es_CL")
	if got := environmentLanguage(); got != English {
		t.Errorf("unsupported primary language gave %q", got)
	}
}
