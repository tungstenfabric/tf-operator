package templates

import (
	"strings"
	"text/template"
)

var tfFuncs = template.FuncMap{
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"add": func(i, j int) int {
		return i + j
	},
	"upperOrDefault": func(word, default_word string) string {
		if word == "" {
			return strings.ToUpper(default_word)
		}
		return strings.ToUpper(word)
	},
	"lowerOrDefault": func(word, default_word string) string {
		if word == "" {
			return strings.ToLower(default_word)
		}
		return strings.ToLower(word)
	},
}
