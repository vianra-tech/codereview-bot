package ast

import (
	"path/filepath"
	"strings"
)

// Language represents a supported programming language
type Language string

const (
	LanguagePython     Language = "python"
	LanguageGo         Language = "go"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
)

// SupportedLanguages returns the list of supported languages
func SupportedLanguages() []Language {
	return []Language{LanguagePython, LanguageGo, LanguageJavaScript, LanguageTypeScript}
}

// IsSupported checks if a language is supported
func IsSupported(lang Language) bool {
	for _, l := range SupportedLanguages() {
		if l == lang {
			return true
		}
	}
	return false
}

// DetectLanguage attempts to detect the language from a file path
func DetectLanguage(filePath string) Language {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".py":
		return LanguagePython
	case ".go":
		return LanguageGo
	case ".js", ".jsx":
		return LanguageJavaScript
	case ".ts", ".tsx":
		return LanguageTypeScript
	default:
		return ""
	}
}

// String returns the language identifier as a string
func (l Language) String() string {
	return string(l)
}
