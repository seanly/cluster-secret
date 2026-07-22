package controllers

import (
	"path/filepath"
	"strings"
)

// OperatorConfig holds global operator settings.
type OperatorConfig struct {
	excludePatterns []string
}

// NewOperatorConfig creates an OperatorConfig from a comma-separated namespace list.
// Entries support shell-style globs, e.g. "u-*" or "p-*".
func NewOperatorConfig(excludeNamespaces string) *OperatorConfig {
	cfg := &OperatorConfig{}

	for _, ns := range strings.Split(excludeNamespaces, ",") {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}
		cfg.excludePatterns = append(cfg.excludePatterns, ns)
	}

	return cfg
}

func (c *OperatorConfig) isGloballyExcluded(name string) bool {
	if c == nil || len(c.excludePatterns) == 0 {
		return false
	}

	for _, pattern := range c.excludePatterns {
		if matchesNamespacePattern(name, pattern) {
			return true
		}
	}

	return false
}

func matchesNamespacePattern(name, pattern string) bool {
	if !strings.ContainsAny(pattern, "*?[") {
		return name == pattern
	}

	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}
