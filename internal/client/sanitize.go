package client

import (
	"strings"
	"unicode"

	"phosphornet/internal/protocol"
)

func sanitizeChromeText(value string) string {
	return sanitizeRemoteText(value, protocol.MaxChromeTextRunes, false)
}

func sanitizeSingleLineText(value string) string {
	return sanitizeRemoteText(value, protocol.MaxSingleLineTextRunes, false)
}

func sanitizeMultilineText(value string) string {
	return sanitizeRemoteText(value, protocol.MaxMultilineTextRunes, true)
}

func sanitizeMarkdownText(value string) string {
	return sanitizeRemoteText(value, protocol.MaxMultilineTextRunes, true)
}

func sanitizeRemoteText(value string, maxRunes int, preserveNewlines bool) string {
	if value == "" || maxRunes <= 0 {
		return ""
	}

	runes := make([]rune, 0, minInt(len([]rune(value)), maxRunes))
	for _, r := range value {
		if len(runes) >= maxRunes {
			break
		}
		switch {
		case r == '\n' && preserveNewlines:
			runes = append(runes, r)
		case r == '\n' || r == '\r' || r == '\t':
			runes = append(runes, ' ')
		case unicode.IsControl(r):
			continue
		case unicode.Is(unicode.Cf, r):
			continue
		case unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			if preserveNewlines {
				runes = append(runes, '\n')
			}
		case unicode.IsPrint(r):
			runes = append(runes, r)
		case unicode.IsSpace(r):
			runes = append(runes, ' ')
		default:
			continue
		}
	}

	return strings.TrimSpace(string(runes))
}
