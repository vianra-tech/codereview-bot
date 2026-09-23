package ast

import (
	"path/filepath"
)

// DetectLanguage attempts to detect the language from a file path
func DetectLanguage(filePath string) Language {
	ext := filepath.Ext(filePath)
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