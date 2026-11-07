// Package formatting implements the pinned GitVersion label-expression contracts.
package formatting

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type TokenType string

const (
	Property    TokenType = "Property"
	Environment TokenType = "Environment"
	Literal     TokenType = "Literal"
)

type Token struct {
	Name   string
	Type   TokenType
	Format *string
}

// ParseTokens splits a label expression into ordered ?? fallbacks. Resolution of
// properties and environment variables is a separate operation. Empty input has
// no tokens, as in the pinned implementation.
func ParseTokens(input string) ([]Token, error) {
	p := tokenizer{input: []rune(input)}
	out := []Token{}
	separated := false
	p.whitespace()
	for p.index < len(p.input) {
		identifier, err := p.identifier()
		if err != nil {
			return nil, err
		}
		p.whitespace()
		if identifier == "" {
			return nil, fmt.Errorf("Invalid format sequence, expected identifier")
		}
		token, err := parseToken(identifier)
		if err != nil {
			return nil, err
		}
		out = append(out, token)
		separated, err = p.separator()
		if err != nil {
			return nil, err
		}
		p.whitespace()
	}
	if separated {
		return nil, fmt.Errorf("Invalid format sequence, expected identifier after '??' separator")
	}
	return out, nil
}

type tokenizer struct {
	input []rune
	index int
}

func (p *tokenizer) whitespace() {
	for p.index < len(p.input) && unicode.IsSpace(p.input[p.index]) {
		p.index++
	}
}
func (p *tokenizer) quote() bool {
	if p.index < len(p.input) && p.input[p.index] == '"' {
		p.index++
		return true
	}
	return false
}
func (p *tokenizer) identifier() (string, error) {
	var value strings.Builder
	quoted := p.quote()
	if quoted {
		value.WriteRune('"')
	}
	for p.index < len(p.input) {
		c := p.input[p.index]
		if !quoted && c == '"' {
			return "", fmt.Errorf("Literal value was not correctly quoted")
		}
		if p.quote() {
			value.WriteRune('"')
			return value.String(), nil
		}
		if !quoted && (unicode.IsSpace(c) || c == '?') {
			return value.String(), nil
		}
		if c == '\\' && p.index+1 < len(p.input) && p.input[p.index+1] == '"' {
			value.WriteRune('"')
			p.index += 2
		} else {
			value.WriteRune(c)
			p.index++
		}
	}
	if quoted {
		return "", fmt.Errorf("Literal value is missing closing quote")
	}
	return value.String(), nil
}
func (p *tokenizer) separator() (bool, error) {
	seen := 0
	for p.index < len(p.input) && seen < 2 {
		if p.input[p.index] != '?' {
			return false, fmt.Errorf("Expected '??' separator")
		}
		seen++
		p.index++
	}
	// Preserve the pin's one-trailing-question-mark behavior. A lone '?' at EOF
	// returns false; '?' followed by whitespace or another token is an error.
	return seen == 2, nil
}
func parseToken(identifier string) (Token, error) {
	if len(identifier) >= 4 && strings.EqualFold(identifier[:4], "env:") {
		return keyToken(identifier[4:], Environment)
	}
	if strings.HasPrefix(identifier, "\"") && strings.HasSuffix(identifier, "\"") {
		return Token{Name: identifier[1 : len(identifier)-1], Type: Literal}, nil
	}
	if _, err := strconv.ParseInt(identifier, 10, 32); err == nil {
		return Token{Name: identifier, Type: Literal}, nil
	}
	return keyToken(identifier, Property)
}
func keyToken(identifier string, kind TokenType) (Token, error) {
	parts := strings.Split(identifier, ":")
	if len(parts) > 2 {
		return Token{}, fmt.Errorf("Invalid format string: %s", identifier)
	}
	out := Token{Name: parts[0], Type: kind}
	if len(parts) == 2 {
		out.Format = &parts[1]
	}
	return out, nil
}
