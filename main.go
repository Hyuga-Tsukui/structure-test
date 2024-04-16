package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"text/template"
)

var funcTemplate = `
package {{ .Package }}

import (
{{ .Imports }}
)

{{- range .Funcs }}
func {{.Name}}(t *testing.T) {
	t.Parallel()
	{{- range .Subtests }}
	t.Run("{{ .Name }}", func(t *testing.T){{ .Body }})
	{{- end }}
}
{{- end }}
`

type TemplateData struct {
    Package string
    Imports string
    Funcs []FuncData
}

type FuncData struct {
	Name     string
	Subtests []Subtest
}

type Subtest struct {
	Name string
	Body string
}

func main() {
	filename := os.Args[1]
    newFilename := filename[:len(filename)-3] + "_generated_test.go"
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		panic(err)
	}

    imports := bytes.Buffer{}
	for _, imp := range f.Imports {
		format.Node(&imports, fset, imp)
		imports.WriteString("\n")
	}

	funcGroups := make(map[string][]Subtest)
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			group, err := extractFnGroup(x.Name.Name)
			if err != nil {
				fmt.Println(err)
				return false
			}
			subTestName, err := extractSubtestName(x.Name.Name)
			if err != nil {
				fmt.Println(err)
				return false
			}
			var buf bytes.Buffer
			if err := format.Node(&buf, fset, x.Body); err != nil {
				fmt.Println(err)
				return false
			}
			funcGroups[group] = append(funcGroups[group], Subtest{
				Name: subTestName,
				Body: buf.String(),
			})
		}
		return true
	})

    templData := TemplateData{
        Package: f.Name.Name,
        Imports: string(imports.Bytes()),
    }

	fnData := make([]FuncData, 0, len(funcGroups))
	for group, subtests := range funcGroups {
		fnData = append(fnData, FuncData{
			Name:     group,
			Subtests: subtests,
		})
	}
    templData.Funcs = fnData

	tmpl, err := template.New("func").Parse(funcTemplate)
	if err != nil {
		panic(err)
	}

    outputFile, err := os.Create(newFilename)
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

    if err := tmpl.Execute(outputFile, templData); err != nil {
		panic(err)
	}
}

func extractFnGroup(funcName string) (string, error) {
	// TestFoo_Bar -> TestFoo
	re := regexp.MustCompile(`^(Test[^_]+)(?:_.*|$)`)
	matches := re.FindStringSubmatch(funcName)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not extract group from %s", funcName)
}

func extractSubtestName(funcName string) (string, error) {
	// TestFoo_Bar -> Bar
	suptestPattern := regexp.MustCompile(`^Test[^_]+_(.*)$`)
	if matches := suptestPattern.FindStringSubmatch(funcName); len(matches) > 1 {
		return matches[1], nil
	}

	// TestBar -> Bar
	mainTestPattern := regexp.MustCompile(`^Test(.+)$`)
	if matches := mainTestPattern.FindStringSubmatch(funcName); len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("could not extract subtest name from %s", funcName)
}
