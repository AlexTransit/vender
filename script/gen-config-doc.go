package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"

	config_global "github.com/AlexTransit/vender/internal/config"
)

func main() {
	files := []string{"/home/vmc/vender-1/internal/config/config_type.go", "/home/vmc/vender-1/internal/engine/inventory/inventory.go", "/home/vmc/vender-1/internal/engine/inventory/stock.go", "/home/vmc/vender-1/internal/engine/config/engine_config.go", "/home/vmc/vender-1/hardware/mdb/evend/config/evend_config.go", "/home/vmc/vender-1/hardware/mdb/config/mdb_config.go", "/home/vmc/vender-1/internal/ui/config/ui_config.go"}
	pkgPaths := []string{"github.com/AlexTransit/vender/internal/config", "github.com/AlexTransit/vender/internal/engine/inventory", "github.com/AlexTransit/vender/internal/engine/inventory", "github.com/AlexTransit/vender/internal/engine/config", "github.com/AlexTransit/vender/hardware/mdb/evend/config", "github.com/AlexTransit/vender/hardware/mdb/config", "github.com/AlexTransit/vender/internal/ui/config"}
	comments := parseComments(files, pkgPaths)
	cfg := new(config_global.Config)
	generateDoc(reflect.TypeOf(cfg).Elem(), "", comments)
}

func parseComments(files []string, pkgPaths []string) map[string]map[string]FieldInfo {
	comments := make(map[string]map[string]FieldInfo)
	for i, filename := range files {
		fset := token.NewFileSet()
		src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		pkgPath := pkgPaths[i]
		ast.Inspect(src, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				typeName := pkgPath + "." + ts.Name.Name
				if st, ok := ts.Type.(*ast.StructType); ok {
					if comments[typeName] == nil {
						comments[typeName] = make(map[string]FieldInfo)
					}
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						fieldName := field.Names[0].Name
						if field.Doc != nil {
							info := parseDescAndExample(strings.TrimSpace(field.Doc.Text()))
							comments[typeName][fieldName] = info
						}
					}
				}
			}
			return true
		})
	}
	return comments
}

type FieldInfo struct {
	DescRU  string
	DescEN  string
	Example string
}

func parseDescAndExample(text string) FieldInfo {
	lines := strings.Split(text, "\n")
	var descRULines, descENLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "RU:"):
			descRULines = append(descRULines, strings.TrimSpace(strings.TrimPrefix(line, "RU:")))
		case strings.HasPrefix(line, "EN:"):
			descENLines = append(descENLines, strings.TrimSpace(strings.TrimPrefix(line, "EN:")))
		case strings.HasPrefix(line, "Example:"):
			return FieldInfo{DescRU: strings.Join(descRULines, " "), DescEN: strings.Join(descENLines, " "), Example: strings.TrimSpace(strings.TrimPrefix(line, "Example:"))}
		default:
			if hasCyrillic(line) {
				descRULines = append(descRULines, line)
			} else {
				descENLines = append(descENLines, line)
			}
		}
	}
	return FieldInfo{DescRU: strings.Join(descRULines, " "), DescEN: strings.Join(descENLines, " ")}
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if r >= '\u0400' && r <= '\u04FF' {
			return true
		}
	}
	return false
}

func generateDoc(t reflect.Type, prefix string, comments map[string]map[string]FieldInfo) {
	typeName := t.PkgPath() + "." + t.Name()
	if typeName == "." && t.Kind() == reflect.Ptr {
		elem := t.Elem()
		typeName = elem.PkgPath() + "." + elem.Name()
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("hcl")
		if tag == "" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fmt.Printf("## %s%s\n\n", prefix, name)
		fmt.Printf("Type: %s\n\n", field.Type.String())
		if typeComments, ok := comments[typeName]; ok {
			if info, ok2 := typeComments[field.Name]; ok2 {
				if info.DescRU != "" {
					fmt.Printf("Description (RU): %s\n\n", info.DescRU)
				}
				if info.DescEN != "" {
					fmt.Printf("Description (EN): %s\n\n", info.DescEN)
				}
				if info.Example != "" {
					fmt.Printf("Example: %s\n\n", info.Example)
				}
			}
		}
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			generateDoc(ft, prefix+name+".", comments)
		}
	}
}
