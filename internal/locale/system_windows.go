package locale

import "syscall"

// System uses the Windows display language, independently of the regional format.
func System() Language {
	language, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	const primaryLanguageMask = 0x3ff
	switch language & primaryLanguageMask {
	case 0x0a: // LANG_SPANISH
		return Spanish
	default:
		return English
	}
}
