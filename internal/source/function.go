package source

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/0stick/CodeWhy/internal/model"
)

func FindFunction(root, file string, line int) (*model.Function, error) {
	if !strings.EqualFold(filepath.Ext(file), ".go") {
		return nil, fmt.Errorf("function analysis currently supports Go files only")
	}
	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(file))
	}
	fileset := token.NewFileSet()
	parsed, err := parser.ParseFile(fileset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot parse Go file %q: %w", file, err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fileset.Position(function.Pos()).Line
		end := fileset.Position(function.End()).Line
		if line < start || line > end {
			continue
		}
		name := function.Name.Name
		if function.Recv != nil && len(function.Recv.List) > 0 {
			name = receiverName(function.Recv.List[0].Type) + "." + name
		}
		return &model.Function{Name: name, StartLine: start, EndLine: end}, nil
	}
	return nil, fmt.Errorf("line %d is not inside a named Go function in %s", line, file)
}

func receiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	case *ast.IndexListExpr:
		return receiverName(value.X)
	default:
		return "receiver"
	}
}
