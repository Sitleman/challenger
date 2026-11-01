package challenge

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strings"
	"unicode/utf16"
)

//go:embed templates/*.tmpl
var templateFiles embed.FS

var messageTemplates = template.Must(template.New("").Funcs(template.FuncMap{
	"counted": counted,
	"name":    shortName,
}).ParseFS(templateFiles, "templates/*.tmpl"))

var messageTags = regexp.MustCompile(`<[^>]*>`)

func renderMessage(name string, data any) (string, error) {
	var out bytes.Buffer
	if err := messageTemplates.ExecuteTemplate(&out, name, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func shortName(name string, limit int) string {
	runes := []rune(strings.Join(strings.Fields(name), " "))
	if len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return string(runes)
}

func messageLength(text string) int {
	plain := html.UnescapeString(messageTags.ReplaceAllString(text, ""))
	return len(utf16.Encode([]rune(plain)))
}

func counted(n int, one, few, many string) string {
	word := many
	if n%100 < 11 || n%100 > 14 {
		switch n % 10 {
		case 1:
			word = one
		case 2, 3, 4:
			word = few
		}
	}
	return fmt.Sprintf("%d %s", n, word)
}
