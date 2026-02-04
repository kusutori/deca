package config

import (
	"fmt"
	"strings"
)

// PackageMatches evaluates OS/Arch conditions and returns whether the package
// should be processed on the given runtime. It also returns the effective
// OS/Arch for asset matching (always the runtime values when matched).
func PackageMatches(pkg *Package, runtimeOS, runtimeArch string) (bool, string, string, error) {
	effectiveOS := normalizeOS(runtimeOS)
	effectiveArch := normalizeArch(runtimeArch)

	if pkg.OS != "" {
		if isExpr(pkg.OS) {
			ok, err := evalValueExpr(pkg.OS, effectiveOS, normalizeOS)
			if err != nil {
				return false, "", "", fmt.Errorf("invalid os expression %q: %w", pkg.OS, err)
			}
			if !ok {
				return false, "", "", nil
			}
		} else if normalizeOS(pkg.OS) != effectiveOS {
			return false, "", "", nil
		}
	}

	if pkg.Arch != "" {
		if isExpr(pkg.Arch) {
			ok, err := evalValueExpr(pkg.Arch, effectiveArch, normalizeArch)
			if err != nil {
				return false, "", "", fmt.Errorf("invalid arch expression %q: %w", pkg.Arch, err)
			}
			if !ok {
				return false, "", "", nil
			}
		} else if normalizeArch(pkg.Arch) != effectiveArch {
			return false, "", "", nil
		}
	}

	return true, effectiveOS, effectiveArch, nil
}

func isExpr(s string) bool {
	return strings.ContainsAny(s, "&|!()")
}

func normalizeOS(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "macos", "osx":
		return "darwin"
	default:
		return v
	}
}

func normalizeArch(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return v
	}
}

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenIdent
	tokenAnd
	tokenOr
	tokenNot
	tokenLParen
	tokenRParen
)

type token struct {
	typ tokenType
	val string
}

func evalValueExpr(expr, actual string, normalize func(string) string) (bool, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return false, err
	}
	p := &parser{tokens: tokens, normalize: normalize, actual: actual}
	result, err := p.parseExpr()
	if err != nil {
		return false, err
	}
	if p.peek().typ != tokenEOF {
		return false, fmt.Errorf("unexpected token %q", p.peek().val)
	}
	return result, nil
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	s := strings.TrimSpace(input)
	for len(s) > 0 {
		switch {
		case strings.HasPrefix(s, "&&"):
			tokens = append(tokens, token{typ: tokenAnd, val: "&&"})
			s = strings.TrimSpace(s[2:])
		case strings.HasPrefix(s, "||"):
			tokens = append(tokens, token{typ: tokenOr, val: "||"})
			s = strings.TrimSpace(s[2:])
		case strings.HasPrefix(s, "!"):
			tokens = append(tokens, token{typ: tokenNot, val: "!"})
			s = strings.TrimSpace(s[1:])
		case strings.HasPrefix(s, "("):
			tokens = append(tokens, token{typ: tokenLParen, val: "("})
			s = strings.TrimSpace(s[1:])
		case strings.HasPrefix(s, ")"):
			tokens = append(tokens, token{typ: tokenRParen, val: ")"})
			s = strings.TrimSpace(s[1:])
		default:
			ident := readIdent(s)
			if ident == "" {
				return nil, fmt.Errorf("invalid token near %q", s)
			}
			tokens = append(tokens, token{typ: tokenIdent, val: ident})
			s = strings.TrimSpace(s[len(ident):])
		}
	}
	tokens = append(tokens, token{typ: tokenEOF})
	return tokens, nil
}

func readIdent(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			break
		}
	}
	return b.String()
}

type parser struct {
	tokens    []token
	pos       int
	actual    string
	normalize func(string) string
}

func (p *parser) parseExpr() (bool, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for p.peek().typ == tokenOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *parser) parseAnd() (bool, error) {
	left, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for p.peek().typ == tokenAnd {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *parser) parseUnary() (bool, error) {
	if p.peek().typ == tokenNot {
		p.next()
		value, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		return !value, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (bool, error) {
	switch p.peek().typ {
	case tokenIdent:
		val := p.next().val
		return p.normalize(val) == p.actual, nil
	case tokenLParen:
		p.next()
		value, err := p.parseExpr()
		if err != nil {
			return false, err
		}
		if p.peek().typ != tokenRParen {
			return false, fmt.Errorf("missing closing )")
		}
		p.next()
		return value, nil
	default:
		return false, fmt.Errorf("unexpected token %q", p.peek().val)
	}
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}
