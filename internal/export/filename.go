package export

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	fileDateLayout  = "2006-01-02"
	fileUnsafeRunes = `/\:*?"<>|`
)

func FileName(title string, date time.Time, extension string) string {
	safe := strings.Map(fileSafeRune, title)
	name := strings.Join(strings.Fields(safe), " ")

	return fmt.Sprintf("RetroTide - %s - %s.%s", name, date.UTC().Format(fileDateLayout), extension)
}

func fileSafeRune(r rune) rune {
	if unicode.IsControl(r) {
		return ' '
	}

	if strings.ContainsRune(fileUnsafeRunes, r) {
		return '-'
	}

	return r
}
