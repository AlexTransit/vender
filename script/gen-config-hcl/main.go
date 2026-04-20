package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	config_global "github.com/AlexTransit/vender/internal/config"
)

type FieldInfo struct {
	DescRU  string
	DescEN  string
	Example string
}

func main() {
	base, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root := base
	if filepath.Base(root) != "vender-1" {
		root = filepath.Join(root, "..")
	}
	basePkg := "github.com/AlexTransit/vender"
	configPath := filepath.Join(root, "internal", "config", "config_type.go")
	configPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(configPath), root)
	inventoryPath := filepath.Join(root, "internal", "engine", "inventory", "inventory.go")
	inventoryPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(inventoryPath), root)
	stockPath := filepath.Join(root, "internal", "engine", "inventory", "stock.go")
	stockPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(stockPath), root)
	engineConfigPath := filepath.Join(root, "internal", "engine", "config", "engine_config.go")
	engineConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(engineConfigPath), root)
	evendConfigPath := filepath.Join(root, "hardware", "mdb", "evend", "config", "evend_config.go")
	evendConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(evendConfigPath), root)
	mdbConfigPath := filepath.Join(root, "hardware", "mdb", "config", "mdb_config.go")
	mdbConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(mdbConfigPath), root)
	uiConfigPath := filepath.Join(root, "internal", "ui", "config", "ui_config.go")
	uiConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(uiConfigPath), root)
	menuConfigPath := filepath.Join(root, "internal", "menu", "menu_config", "config_menu.go")
	menuConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(menuConfigPath), root)
	soundConfigPath := filepath.Join(root, "internal", "sound", "config", "sound_config.go")
	soundConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(soundConfigPath), root)
	watchdogConfigPath := filepath.Join(root, "internal", "watchdog", "config", "watchdog_config.go")
	watchdogConfigPkgPath := basePkg + strings.TrimPrefix(filepath.Dir(watchdogConfigPath), root)
	comments, err := parseComments([]string{configPath, inventoryPath, stockPath, engineConfigPath, evendConfigPath, mdbConfigPath, uiConfigPath, menuConfigPath, soundConfigPath, watchdogConfigPath}, []string{configPkgPath, inventoryPkgPath, stockPkgPath, engineConfigPkgPath, evendConfigPkgPath, mdbConfigPkgPath, uiConfigPkgPath, menuConfigPkgPath, soundConfigPkgPath, watchdogConfigPkgPath})
	if err != nil {
		panic(err)
	}
	pathComments := buildPathComments(comments)
	config_global.WriteConfigToFile()
	hclPath := filepath.Join(root, "defaultConfig.hcl")
	// Add example blocks
	if err := addExamples(hclPath); err != nil {
		panic(err)
	}
	if err := insertComments(hclPath, pathComments); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %s with comments\n", hclPath)
}

func addExamples(hclPath string) error {
	text, err := os.ReadFile(hclPath)
	if err != nil {
		return err
	}
	content := string(text)
	// Insert examples inside inventory
	re1 := regexp.MustCompile(`(inventory\s*\{[^}]*stock_file\s*=\s*[^}]+)\}`)
	replacement1 := `${1}
  ingredient "sugar" {
    min = 50 
    spend_rate = 0.86 
    level = "1(330) 2(880)" 
    tuning_key = "sugar"
    cost = 0.080
  }
  ingredient "amaretto" {
    min = 50
    spend_rate = 0.42 
    level = "1(262) 2(700)"
    cost = 0.735
  }		
  stock "1" {
    code = 1
    ingredient = "sugar"
    register_add = "h18_position evend.hopper1.run(?) h1_shake " 	
  }
  stock "2" {
    code = 2
    ingredient = "amaretto"
    register_add = "h27_position evend.hopper2.run(?) h27_shake "  
  }	
}`
	content = re1.ReplaceAllString(content, replacement1)
	// Add device block in hardware
	re2 := regexp.MustCompile(`(hardware\s*\{)`)
	replacement2 := `${1}
  device "example" {
    required = true
    disabled = false
  }`
	content = re2.ReplaceAllString(content, replacement2)
	// Add alias block in engine
	re4 := regexp.MustCompile(`(engine\s*\{)`)
	replacement4 := `${1}
  alias "example" {
    scenario = "example_scenario"
    onError "3[78]" { // this will match error codes 37 and 38
      scenario = "error_scenario"
    }
  }
  alias "example1" {
    scenario = "example_scenario1"
    onError "3" {
      scenario = "error_scenario1"
    }
    onError "4" {
      scenario = "error_scenario2"
    }
    onError "\d{2}" { // this will match any 2 digit error code, but if there are more specific regexes for 30-39, they will take precedence over this one
      scenario = "error_scenario2"
    }
  }
  menu {
    item "43." {
      name = "горячий шоколад со сливками и орешками" 
      price = 80 
      creamMax = 4 
      sugarMax = 4 
      scenario = " preset add.sugar(5) add.chocolate(40) cream20 mix_strong w_hot70 cup_serve_p "
    }
    item "4" { disabled = true}
    item "31" {
      name = "кофе с шоколадом и орешками" 
      price = 60 
      scenario = " preset add.coffee(5) add.chocolate(30) mix_midle w_hot85 add.peanut(7) cup_serve_p "
    }
  }`
	content = re4.ReplaceAllString(content, replacement4)
	// Remove duplicate empty menu block generated by gohcl in engine
	content = regexp.MustCompile(`\n\s*menu\s*\{\s*\}\n`).ReplaceAllString(content, "\n")
	// Add test block in ui.service
	re3 := regexp.MustCompile(`(service\s*\{)`)
	replacement3 := `${1}
    test "boiler-fill" { scenario="evend.valve.reserved_on evend.valve.pump_start sleep(2s) evend.valve.pump_stop evend.valve.reserved_off"}
    test "conveyor" { scenario=" conveyor_test"}
    test "lift" { scenario="elevator_test" }
    test "mixer" { scenario="mix_poorly(10)" }
`
	content = re3.ReplaceAllString(content, replacement3)
	// Reformat test blocks to multiline
	content = regexp.MustCompile(`(test\s+"([^"]+)"\s*\{\s*scenario="([^"]*)"\s*\})`).ReplaceAllString(content, `test "$2" {
    scenario = "$3"
  }`)
	// Reformat onError one-line blocks to multiline
	content = regexp.MustCompile(`onError\s+"([^"]+)"\s*\{\s*scenario\s*=\s*"([^"]*)"\s*\}`).ReplaceAllString(content, `onError "$1" {
      scenario = "$2"
    }`)
	// Reformat inline item blocks with disabled attr
	content = regexp.MustCompile(`item\s+"([^"]+)"\s*\{\s*disabled\s*=\s*(true|false)\s*\}`).ReplaceAllString(content, `item "$1" {
      disabled = $2
    }`)
	return os.WriteFile(hclPath, []byte(content), 0o644)
}

func parseComments(files []string, pkgPaths []string) (map[string]map[string]FieldInfo, error) {
	comments := make(map[string]map[string]FieldInfo)
	for i, filename := range files {
		fset := token.NewFileSet()
		src, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		pkgPath := pkgPaths[i]
		ast.Inspect(src, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				typeName := pkgPath + "." + ts.Name.Name
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						fieldName := field.Names[0].Name
						if cg := field.Doc; cg != nil {
							info := parseDescAndExample(strings.TrimSpace(cg.Text()))
							if comments[typeName] == nil {
								comments[typeName] = make(map[string]FieldInfo)
							}
							comments[typeName][fieldName] = info
						}
					}
				}
			}
			return true
		})
	}
	return comments, nil
}

func buildPathComments(comments map[string]map[string]FieldInfo) map[string]FieldInfo {
	paths := make(map[string]FieldInfo)
	walkType(reflect.TypeOf(config_global.Config{}), "", comments, paths)
	return paths
}

func walkType(t reflect.Type, prefix string, comments map[string]map[string]FieldInfo, paths map[string]FieldInfo) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	typeComments := comments[t.PkgPath()+"."+t.Name()]
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("hcl")
		if tag == "" {
			continue
		}
		tagParts := strings.Split(tag, ",")
		name := tagParts[0]
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		isLabel := false
		for _, opt := range tagParts[1:] {
			if opt == "label" {
				isLabel = true
				break
			}
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if info, ok := typeComments[field.Name]; ok {
			if isLabel {
				labelPath := prefix
				if labelPath == "" {
					labelPath = name
				}
				paths[labelPath+".label"] = info
			} else {
				paths[path] = info
			}
		}
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			walkType(ft, path, comments, paths)
		} else if ft.Kind() == reflect.Slice {
			elem := ft.Elem()
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				walkType(elem, path, comments, paths)
			}
		} else if ft.Kind() == reflect.Map {
			elem := ft.Elem()
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct {
				walkType(elem, path, comments, paths)
			}
		}
	}
}

func parseDescAndExample(text string) FieldInfo {
	lines := strings.Split(text, "\n")
	var descRULines, descENLines []string
	info := FieldInfo{}
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
			info.Example = strings.TrimSpace(strings.TrimPrefix(line, "Example:"))
		default:
			if hasCyrillic(line) {
				descRULines = append(descRULines, line)
			} else {
				descENLines = append(descENLines, line)
			}
		}
	}
	info.DescRU = strings.Join(descRULines, " ")
	info.DescEN = strings.Join(descENLines, " ")
	return info
}

func hasCyrillic(s string) bool {
	for _, r := range s {
		if r >= '\u0400' && r <= '\u04FF' {
			return true
		}
	}
	return false
}

func insertComments(hclPath string, comments map[string]FieldInfo) error {
	text, err := os.ReadFile(hclPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(text), "\n")
	stack := []string{}
	result := make([]string, 0, len(lines)*2)
	attrRe := regexp.MustCompile(`^\s*([a-zA-Z0-9_]+)\s*=\s*`)
	blockRe := regexp.MustCompile(`^\s*([a-zA-Z0-9_]+)(\s+"[^"]+")?\s*\{\s*(?:#.*|//.*)?$`)
	closeRe := regexp.MustCompile(`^\s*\}\s*(?:#.*|//.*)?$`)
	for _, line := range lines {
		if closeRe.MatchString(line) {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			result = append(result, line)
			continue
		}
		if m := blockRe.FindStringSubmatch(line); m != nil {
			blockName := m[1]
			path := strings.Join(append(stack, blockName), ".")
			if info, ok := comments[path]; ok {
				result = append(result, buildCommentLines(&info)...) // comment before block
			}
			if info, ok := comments[path+".label"]; ok {
				result = append(result, buildCommentLines(&info)...) // comment for label
			}
			result = append(result, line)
			stack = append(stack, blockName)
			continue
		}
		if m := attrRe.FindStringSubmatch(line); m != nil {
			attrName := m[1]
			path := strings.Join(append(stack, attrName), ".")
			if info, ok := comments[path]; ok {
				result = append(result, buildCommentLines(&info)...) // comment before attribute
			}
		}
		result = append(result, line)
	}
	return os.WriteFile(hclPath, []byte(strings.Join(result, "\n")), 0o644)
}

func buildCommentLines(info *FieldInfo) []string {
	lines := []string{}
	if info.DescRU != "" {
		lines = append(lines, "# RU: "+info.DescRU)
	}
	if info.DescEN != "" {
		lines = append(lines, "# EN: "+info.DescEN)
	}
	if info.Example != "" {
		lines = append(lines, "# Example: "+info.Example)
	}
	return lines
}
