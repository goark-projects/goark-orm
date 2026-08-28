package ormgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

type scanContext struct {
	fset        *token.FileSet
	interfaces  map[string]scanInterface
	fileSources map[*ast.File]scanSource
	imports     map[string]ImportModel
}

type scanSource struct {
	PackagePath string
	Imports     map[string]string
}

type scanInterface struct {
	Source    scanSource
	Interface *ast.InterfaceType
}

func newScanContext(fset *token.FileSet, pkg *ast.Package, sources []scanPackageSource) *scanContext {
	ctx := &scanContext{
		fset:        fset,
		interfaces:  make(map[string]scanInterface),
		fileSources: make(map[*ast.File]scanSource),
		imports:     make(map[string]ImportModel),
	}
	if len(sources) == 0 {
		sources = []scanPackageSource{{PackageName: pkg.Name, Files: sortedFiles(fset, pkg)}}
	}
	for index, source := range sources {
		ctx.indexPackageSource(source, index == 0)
	}
	return ctx
}

func (ctx *scanContext) indexPackageSource(source scanPackageSource, root bool) {
	for _, file := range source.Files {
		fileSource := scanSource{
			PackagePath: source.PackagePath,
			Imports:     sourceFileImports(source, file),
		}
		ctx.fileSources[file] = fileSource
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, rawSpec := range genDecl.Specs {
				typeSpec, ok := rawSpec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				item := scanInterface{Source: fileSource, Interface: interfaceType}
				if root {
					ctx.interfaces[typeSpec.Name.Name] = item
				}
				if source.PackagePath != "" {
					ctx.interfaces[interfaceKey(source.PackagePath, typeSpec.Name.Name)] = item
				}
			}
		}
	}
}

func sourceFileImports(source scanPackageSource, file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, item := range file.Imports {
		if item.Path == nil {
			continue
		}
		importPath := strings.Trim(item.Path.Value, `"`)
		alias := ""
		if item.Name != nil {
			alias = strings.TrimSpace(item.Name.Name)
			if alias == "." || alias == "_" {
				continue
			}
		}
		if alias == "" {
			alias = strings.TrimSpace(source.ImportNames[importPath])
		}
		if alias == "" {
			alias = importNameForPath(importPath)
		}
		if alias != "" && importPath != "" {
			imports[alias] = importPath
		}
	}
	return imports
}

func (ctx *scanContext) sourceForFile(file *ast.File) scanSource {
	if source, ok := ctx.fileSources[file]; ok {
		return source
	}
	return scanSource{}
}

func (ctx *scanContext) exprString(source scanSource, expr ast.Expr) string {
	out := exprString(ctx.fset, expr)
	for alias, importPath := range source.Imports {
		if !fieldTypeContainsQualifier(out, alias) {
			continue
		}
		switch importPath {
		case "context":
			out = replaceTypeQualifier(out, alias, "context")
		case "goark.dev/orm":
			out = replaceTypeQualifier(out, alias, "orm")
		default:
			ctx.addImport(alias, importPath)
		}
	}
	return out
}

func (ctx *scanContext) addImport(name string, importPath string) {
	importPath = strings.TrimSpace(importPath)
	if importPath == "" || importPath == "context" || importPath == "goark.dev/orm" {
		return
	}
	name = strings.TrimSpace(name)
	if _, exists := ctx.imports[importPath]; exists {
		return
	}
	item := ImportModel{Path: importPath}
	if name != "" && name != importNameForPath(importPath) {
		item.Name = name
	}
	ctx.imports[importPath] = item
}

func (ctx *scanContext) importList() []ImportModel {
	if len(ctx.imports) == 0 {
		return nil
	}
	out := make([]ImportModel, 0, len(ctx.imports))
	for _, item := range ctx.imports {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func interfaceKey(packagePath string, name string) string {
	packagePath = strings.TrimSpace(packagePath)
	name = strings.TrimSpace(name)
	if packagePath == "" {
		return name
	}
	return packagePath + "." + name
}

func replaceTypeQualifier(value string, from string, to string) string {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || from == to {
		return value
	}
	token := from + "."
	var builder strings.Builder
	offset := 0
	for {
		index := strings.Index(value[offset:], token)
		if index < 0 {
			break
		}
		index += offset
		if index > 0 && isGoIdentifierPart(rune(value[index-1])) {
			offset = index + len(token)
			continue
		}
		builder.WriteString(value[offset:index])
		builder.WriteString(to)
		builder.WriteByte('.')
		offset = index + len(token)
	}
	if offset == 0 {
		return value
	}
	builder.WriteString(value[offset:])
	return builder.String()
}
