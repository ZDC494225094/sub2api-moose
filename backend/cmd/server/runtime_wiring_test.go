package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Starting a full application requires live infrastructure. Inspect the actual
// generated injector instead, and pair this with provider/behavior tests so a
// side-effect-only provider cannot silently disappear on the next regeneration.
func TestGeneratedApplicationRetainsRuntimeWiring(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "wire_gen.go", nil, 0)
	require.NoError(t, err)
	var injector *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "initializeApplication" {
			injector = fn
			break
		}
	}
	require.NotNil(t, injector)
	provided := map[string]string{}
	calls := map[string]*ast.CallExpr{}
	ast.Inspect(injector.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) == 0 {
			return true
		}
		call, ok := assignment.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch function := call.Fun.(type) {
		case *ast.Ident:
			name = function.Name
		case *ast.SelectorExpr:
			name = function.Sel.Name
		}
		if variable, ok := assignment.Lhs[0].(*ast.Ident); ok {
			provided[name] = variable.Name
			calls[name] = call
		}
		return true
	})

	require.NotEmpty(t, provided["ProvideBillingSchedulingAdmission"], "Wire must construct billing admission")
	require.NotEmpty(t, provided["ProvidePluginManager"], "Wire must use the account-directory-aware provider")
	require.NotContains(t, calls, "NewPluginManager", "the bare constructor loses account-directory wiring")
	application := calls["provideApplication"]
	require.NotNil(t, application, "the application provider must consume the admission dependency")
	require.Len(t, application.Args, 5)
	admission, ok := application.Args[4].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, provided["ProvideBillingSchedulingAdmission"], admission.Name)
	manager := calls["ProvidePluginManager"]
	require.Len(t, manager.Args, 6)
	directory, ok := manager.Args[5].(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, provided["NewOpenAIGatewayService"], directory.Name, "the gateway owns the authorized OAuth account directory")
}
