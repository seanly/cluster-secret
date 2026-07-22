package controllers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewOperatorConfig(t *testing.T) {
	cfg := NewOperatorConfig(" kube-system, kube-public ,,default ")

	if !cfg.isGloballyExcluded("kube-system") {
		t.Fatal("expected kube-system to be excluded")
	}
	if !cfg.isGloballyExcluded("kube-public") {
		t.Fatal("expected kube-public to be excluded")
	}
	if !cfg.isGloballyExcluded("default") {
		t.Fatal("expected default to be excluded")
	}
	if cfg.isGloballyExcluded("app") {
		t.Fatal("expected app not to be excluded")
	}
}

func TestNewOperatorConfigGlobPatterns(t *testing.T) {
	cfg := NewOperatorConfig("u-*,p-*,kube-system")

	for _, ns := range []string{"u-abc123", "u-c-xxxxx", "p-project1", "p-xyz"} {
		if !cfg.isGloballyExcluded(ns) {
			t.Fatalf("expected %s to be excluded", ns)
		}
	}

	for _, ns := range []string{"app", "production", "u", "p", "user-prod"} {
		if cfg.isGloballyExcluded(ns) {
			t.Fatalf("expected %s not to be excluded", ns)
		}
	}
}

func TestMatchesNamespacePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{name: "kube-system", pattern: "kube-system", want: true},
		{name: "kube-public", pattern: "kube-system", want: false},
		{name: "u-abc", pattern: "u-*", want: true},
		{name: "user-prod", pattern: "u-*", want: false},
		{name: "p-dev", pattern: "p-*", want: true},
	}

	for _, tt := range tests {
		if got := matchesNamespacePattern(tt.name, tt.pattern); got != tt.want {
			t.Fatalf("matchesNamespacePattern(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.want)
		}
	}
}

func TestShouldSkipNamespace(t *testing.T) {
	cfg := NewOperatorConfig("kube-system")

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-system"},
	}
	if skip, _ := shouldSkipNamespace(ns, cfg); !skip {
		t.Fatal("expected global exclude to skip namespace")
	}

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app",
			Annotations: map[string]string{
				ignoreAnnotation: "true",
			},
		},
	}
	if skip, _ := shouldSkipNamespace(ns, cfg); !skip {
		t.Fatal("expected annotation to skip namespace")
	}

	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
	}
	if skip, _ := shouldSkipNamespace(ns, cfg); skip {
		t.Fatal("expected namespace to be processed")
	}
}
