package ast

import (
	"context"
	"fmt"
	"sync"

	"github.com/tree-sitter/go-tree-sitter"
)

// Language represents a supported programming language
type Language string

const (
	LanguagePython  Language = "python"
	LanguageGo      Language = "go"
	LanguageJavaScript Language = "javascript"
	LanguageTypeScript Language = "typescript"
)

// Parser manages tree-sitter parsers for different languages
type Parser struct {
	parsers map[Language]*tree_sitter.Parser
	mu      sync.RWMutex
}

// NewParser creates a new multi-language parser
func NewParser() *Parser {
	p := &Parser{
		parsers: make(map[Language]*tree_sitter.Parser),
	}
	
	// Initialize parsers for each supported language
	for _, lang := range []Language{LanguagePython, LanguageGo, LanguageJavaScript, LanguageTypeScript} {
		parser := tree_sitter.NewParser()
		p.parsers[lang] = parser
	}
	
	return p
}

// Parse parses source code into a syntax tree
func (p *Parser) Parse(ctx context.Context, lang Language, source []byte) (*tree_sitter.Tree, error) {
	p.mu.RLock()
	parser, ok := p.parsers[lang]
	p.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	
	// Set the language for this parser
	language, err := GetLanguage(lang)
	if err != nil {
		return nil, err
	}
	
	parser.SetLanguage(language)
	
	// Parse the source code
	tree := parser.Parse(ctx, nil, source)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse source")
	}
	
	return tree, nil
}

// GetLanguage returns the tree-sitter language for a given language
func GetLanguage(lang Language) (*tree_sitter.Language, error) {
	// Note: These require CGO compilation with the tree-sitter grammars
	// For now, we'll return an error indicating the language needs to be compiled in
	return nil, fmt.Errorf("tree-sitter grammar for %s not loaded; requires CGO build with grammar library", lang)
}

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