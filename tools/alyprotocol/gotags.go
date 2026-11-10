package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// GoStruct 是 dto.go 中一个结构体的输出字段视图。
type GoStruct struct {
	Name   string
	Fields []*GoField
	byTag  map[string]*GoField
	byName map[string]*GoField
}

// GoField 是一个带 json tag 的结构体字段。
type GoField struct {
	Name      string
	Tag       string // json 名称；空表示无 tag
	OmitEmpty bool
	Type      GoType
}

// GoType 是字段类型的简化描述（只覆盖协议用到的形态）。
type GoType struct {
	Base        string
	IsArray     bool
	Elem        string
	IsPointer   bool
	IsInterface bool
}

func (t GoType) String() string {
	s := t.Base
	if t.IsInterface {
		s = "interface{}"
	}
	if t.IsArray {
		s = "[]" + t.Elem
	}
	if t.IsPointer {
		s = "*" + s
	}
	return s
}

// ParseGoStructs 解析 Go 源码里的结构体与 json tag。
func ParseGoStructs(src []byte) (map[string]*GoStruct, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "dto.go", src, 0)
	if err != nil {
		return nil, fmt.Errorf("解析 dto.go: %v", err)
	}

	structs := make(map[string]*GoStruct)
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			gs := &GoStruct{
				Name:   ts.Name.Name,
				byTag:  make(map[string]*GoField),
				byName: make(map[string]*GoField),
			}
			for _, fld := range st.Fields.List {
				if len(fld.Names) == 0 {
					continue // 内嵌字段不参与协议
				}
				var tagRaw string
				if fld.Tag != nil {
					tagRaw, err = strconv.Unquote(fld.Tag.Value)
					if err != nil {
						return nil, fmt.Errorf("%s 的字段 tag 不是合法字符串: %v", ts.Name.Name, err)
					}
				}
				typ, err := parseGoType(fld.Type)
				if err != nil {
					return nil, fmt.Errorf("%s.%s: %v", ts.Name.Name, fld.Names[0].Name, err)
				}
				for _, n := range fld.Names {
					name, omitEmpty, hasJSON := parseJSONTag(tagRaw, n.Name)
					if hasJSON && name == "-" {
						continue
					}
					gf := &GoField{Name: n.Name, OmitEmpty: omitEmpty, Type: typ}
					if hasJSON {
						gf.Tag = name
						if prev := gs.byTag[name]; prev != nil {
							return nil, fmt.Errorf("%s 中 json:%q 重复（%s 与 %s）", gs.Name, name, prev.Name, n.Name)
						}
						gs.byTag[name] = gf
					}
					gs.Fields = append(gs.Fields, gf)
					gs.byName[n.Name] = gf
				}
			}
			structs[gs.Name] = gs
		}
	}
	return structs, nil
}

func parseJSONTag(raw, fieldName string) (name string, omitEmpty bool, ok bool) {
	if raw == "" {
		return "", false, false
	}
	val, found := reflect.StructTag(raw).Lookup("json")
	if !found || val == "" {
		return "", false, false
	}
	if val == "-" {
		return "-", false, true
	}
	parts := strings.Split(val, ",")
	name = parts[0]
	if name == "" {
		name = fieldName
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, true
}

func parseGoType(expr ast.Expr) (GoType, error) {
	switch e := expr.(type) {
	case *ast.Ident:
		return GoType{Base: e.Name}, nil
	case *ast.StarExpr:
		inner, err := parseGoType(e.X)
		if err != nil {
			return GoType{}, err
		}
		inner.IsPointer = true
		return inner, nil
	case *ast.ArrayType:
		return GoType{IsArray: true, Elem: exprString(e.Elt)}, nil
	case *ast.InterfaceType:
		if e.Methods != nil && len(e.Methods.List) > 0 {
			return GoType{}, fmt.Errorf("不支持带方法的 interface 字段")
		}
		return GoType{IsInterface: true, Base: "interface{}"}, nil
	case *ast.SelectorExpr:
		return GoType{Base: exprString(e)}, nil
	default:
		return GoType{}, fmt.Errorf("不支持的 Go 类型 %T", expr)
	}
}

func exprString(e ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, token.NewFileSet(), e)
	return buf.String()
}

// FieldByTag 按 json 名称查找字段。
func (s *GoStruct) FieldByTag(tag string) *GoField {
	return s.byTag[tag]
}

// FieldByName 按 Go 字段名查找字段。
func (s *GoStruct) FieldByName(name string) *GoField {
	return s.byName[name]
}

// Verify 双向校验 schema 与 Go 客户端字段：名称、类型、必填/可选。
func Verify(s *Schema, cfg *Config, structs map[string]*GoStruct) []error {
	var errs []error
	for _, defName := range sortedKeys(cfg.Go.Structs) {
		obj := s.Def(defName)
		if obj == nil {
			errs = append(errs, fmt.Errorf("配置引用了 schema 中不存在的定义 %q", defName))
			continue
		}
		structName := cfg.Go.Structs[defName]
		gs := structs[structName]
		if gs == nil {
			errs = append(errs, fmt.Errorf("dto.go 中找不到结构体 %q（定义 %q）", structName, defName))
			continue
		}

		for _, p := range obj.Properties {
			gf := gs.FieldByTag(p.Name)
			if gf == nil {
				errs = append(errs, fmt.Errorf("schema 属性 %q 未被 Go 结构体 %s 输出", defName+"."+p.Name, structName))
				continue
			}
			required := obj.IsRequired(p.Name)
			switch {
			case required && gf.OmitEmpty:
				errs = append(errs, fmt.Errorf("schema 属性 %q 是必填，但 Go 字段 %s.%s 带 omitempty", defName+"."+p.Name, structName, gf.Name))
			case required && gf.Type.IsPointer:
				errs = append(errs, fmt.Errorf("schema 属性 %q 是必填，但 Go 字段 %s.%s 是指针（可能输出 null）", defName+"."+p.Name, structName, gf.Name))
			case !required && !gf.OmitEmpty:
				errs = append(errs, fmt.Errorf("schema 属性 %q 可选，但 Go 字段 %s.%s 没有 omitempty（会始终输出）", defName+"."+p.Name, structName, gf.Name))
			}
			if err := typeCompatible(s, cfg, p, gf); err != nil {
				errs = append(errs, fmt.Errorf("schema 属性 %q 类型 %s 与 Go 字段 %s.%s（%s）不兼容: %v",
					defName+"."+p.Name, schemaTypeLabel(p), structName, gf.Name, gf.Type.String(), err))
			}
		}

		for _, gf := range gs.Fields {
			if gf.Tag == "" {
				continue
			}
			if obj.Property(gf.Tag) == nil {
				errs = append(errs, fmt.Errorf("Go 字段 %s.%s（json:%q）未在 schema 中登记", structName, gf.Name, gf.Tag))
			}
		}
	}
	return errs
}

func schemaTypeLabel(p *Property) string {
	switch {
	case p.Type == "":
		return "any"
	case p.Type == "array" && p.ItemsRef != "":
		return "array<" + strings.TrimPrefix(p.ItemsRef, "#/$defs/") + ">"
	case p.Type == "array":
		return "array<" + p.ItemsType + ">"
	case p.Ref != "":
		return strings.TrimPrefix(p.Ref, "#/$defs/")
	default:
		return p.Type
	}
}

func typeCompatible(s *Schema, cfg *Config, p *Property, gf *GoField) error {
	switch p.Type {
	case "":
		if !gf.Type.IsInterface {
			return fmt.Errorf("schema 无 type，Go 字段必须是 interface{}")
		}
		return nil
	case "string":
		return expectBase(gf, "string")
	case "boolean":
		return expectBase(gf, "bool")
	case "integer":
		return expectBase(gf, "int", "int32", "int64")
	case "number":
		return expectBase(gf, "float32", "float64")
	case "object":
		want := goStructNameForRef(cfg, p.Ref)
		return expectBase(gf, want)
	case "array":
		if !gf.Type.IsArray {
			return fmt.Errorf("schema 是数组，Go 字段不是 slice")
		}
		if p.ItemsRef != "" {
			want := goStructNameForRef(cfg, p.ItemsRef)
			if gf.Type.Elem != want {
				return fmt.Errorf("数组元素应为 %s，实际为 %s", want, gf.Type.Elem)
			}
			return nil
		}
		switch p.ItemsType {
		case "string":
			if gf.Type.Elem != "string" {
				return fmt.Errorf("数组元素应为 string，实际为 %s", gf.Type.Elem)
			}
		case "boolean":
			if gf.Type.Elem != "bool" {
				return fmt.Errorf("数组元素应为 bool，实际为 %s", gf.Type.Elem)
			}
		case "integer":
			if gf.Type.Elem != "int" && gf.Type.Elem != "int32" && gf.Type.Elem != "int64" {
				return fmt.Errorf("数组元素应为整数，实际为 %s", gf.Type.Elem)
			}
		case "number":
			if gf.Type.Elem != "float32" && gf.Type.Elem != "float64" {
				return fmt.Errorf("数组元素应为浮点数，实际为 %s", gf.Type.Elem)
			}
		default:
			return fmt.Errorf("不支持的数组元素类型 %q", p.ItemsType)
		}
		return nil
	}
	return fmt.Errorf("不支持的 schema 类型 %q", p.Type)
}

func expectBase(gf *GoField, allowed ...string) error {
	for _, a := range allowed {
		if gf.Type.Base == a {
			return nil
		}
	}
	return fmt.Errorf("期望 %s", strings.Join(allowed, " 或 "))
}

func goStructNameForRef(cfg *Config, ref string) string {
	name := strings.TrimPrefix(ref, "#/$defs/")
	if mapped, ok := cfg.Go.Structs[name]; ok {
		return mapped
	}
	return name
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}