package cache

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	namespacesByName = map[string]Namespace{
		"Setting":              Setting,
		"UserRole":             UserRole,
		"RolePermissions":      RolePermissions,
		"OrgMembership":        OrgMembership,
		"TeamMembership":       TeamMembership,
		"TeamHeader":           TeamHeader,
		"Subscription":         Subscription,
		"EntitlementOverrides": EntitlementOverrides,
		"OGMeta":               OGMeta,
	}
)

func declaredNamespaceNames(t *testing.T) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "keys.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err)

	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}

				literal, ok := value.Values[i].(*ast.CompositeLit)
				if !ok {
					continue
				}

				typeName, ok := literal.Type.(*ast.Ident)
				if ok && typeName.Name == "Namespace" {
					names = append(names, name.Name)
				}
			}
		}
	}

	return names
}

func TestKeysTestCoversEveryDeclaredNamespace(t *testing.T) {
	// given
	checked := slices.Collect(maps.Keys(namespacesByName))

	// when
	declared := declaredNamespaceNames(t)

	// then
	assert.ElementsMatch(t, declared, checked)
}

func TestNamespacePrefixesAreUnique(t *testing.T) {
	// given
	names := slices.Sorted(maps.Keys(namespacesByName))

	// when
	var overlaps []string
	for i, first := range names {
		for _, second := range names[i+1:] {
			a := namespacesByName[first].Prefix
			b := namespacesByName[second].Prefix

			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				overlaps = append(overlaps, first+" ("+a+") and "+second+" ("+b+")")
			}
		}
	}

	// then
	assert.Empty(t, overlaps, "two namespaces could write the same key")
}

func TestSettingNeverExpires(t *testing.T) {
	// given
	namespace := Setting

	// when
	ttl := namespace.TTL

	// then a setting changes only through a settings write, which writes its new value straight into the cache
	assert.Zero(t, ttl)
}

func TestEveryOtherNamespaceHasAFiniteTTL(t *testing.T) {
	for name, ns := range namespacesByName {
		if ns == Setting {
			continue
		}

		t.Run(name, func(t *testing.T) {
			// given
			namespace := ns

			// when
			ttl := namespace.TTL

			// then a missed invalidation heals itself instead of lasting until restart
			assert.Positive(t, ttl)
		})
	}
}
