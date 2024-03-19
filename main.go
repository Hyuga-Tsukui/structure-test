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
{{- range . }}
func {{.Name}}(t *testing.T) {
	t.Parallel()
	{{- range .Subtests }}
	t.Run("{{ .Name }}", func(t *testing.T){{ .Body }})
	{{- end }}
}
{{- end }}
`

type FuncTemplateData struct {
	Name     string
	Subtests []Subtest
}

type Subtest struct {
	Name string
	Body string
}

func main() {
	filename := os.Args[1]
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		panic(err)
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

	data := make([]FuncTemplateData, 0, len(funcGroups))
	for group, subtests := range funcGroups {
		data = append(data, FuncTemplateData{
			Name:     group,
			Subtests: subtests,
		})
	}

	tmpl, err := template.New("func").Parse(funcTemplate)
	if err != nil {
		panic(err)
	}

	o, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer o.Close()

	if err := tmpl.Execute(o, data); err != nil {
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
