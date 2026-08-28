package ormgen

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type loadedScanPackage struct {
	fset    *token.FileSet
	pkg     *ast.Package
	sources []scanPackageSource
}

type scanPackageSource struct {
	PackagePath string
	PackageName string
	Files       []*ast.File
	ImportNames map[string]string
}

func loadScanPackage(spec GenerateSpec) (*loadedScanPackage, error) {
	loaded, err := loadPackageWithTypes(spec)
	if err == nil {
		return loaded, nil
	}
	fallback, fallbackErr := loadPackageWithParser(spec)
	if fallbackErr != nil {
		return nil, err
	}
	return fallback, nil
}

func loadPackageWithTypes(spec GenerateSpec) (*loadedScanPackage, error) {
	fset := token.NewFileSet()
	config := &packages.Config{
		Dir:        spec.Dir,
		Fset:       fset,
		Tests:      false,
		BuildFlags: packageBuildFlags(spec.BuildTags),
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedDeps,
	}
	pkgs, err := packages.Load(config, ".")
	if err != nil {
		return nil, err
	}
	root, err := chooseLoadedPackage(pkgs, spec.PackageName, spec.Dir)
	if err != nil {
		return nil, err
	}
	pkg := astPackageFromLoaded(fset, root)
	if len(pkg.Files) == 0 {
		return nil, packageLoadError(root, spec.Dir)
	}
	sources := make([]scanPackageSource, 0, 8)
	visited := make(map[*packages.Package]struct{})
	collectLoadedSources(root, visited, &sources)
	return &loadedScanPackage{fset: fset, pkg: pkg, sources: sources}, nil
}

func loadPackageWithParser(spec GenerateSpec) (*loadedScanPackage, error) {
	fset := token.NewFileSet()
	buildContext := build.Default
	buildContext.BuildTags = uniqueConfigStrings(spec.BuildTags)
	var matchErr error
	packagesByName, err := parser.ParseDir(fset, spec.Dir, func(info os.FileInfo) bool {
		if matchErr != nil {
			return false
		}
		name := info.Name()
		if !scanSourceFileNameAllowed(name) {
			return false
		}
		ok, err := buildContext.MatchFile(spec.Dir, name)
		if err != nil {
			matchErr = err
			return false
		}
		return ok
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if matchErr != nil {
		return nil, matchErr
	}
	pkg, err := chooseParsedPackage(packagesByName, spec.PackageName, spec.Dir)
	if err != nil {
		return nil, err
	}
	files := sortedFiles(fset, pkg)
	source := scanPackageSource{
		PackageName: pkg.Name,
		Files:       files,
	}
	return &loadedScanPackage{fset: fset, pkg: pkg, sources: []scanPackageSource{source}}, nil
}

func chooseLoadedPackage(pkgs []*packages.Package, packageName string, dir string) (*packages.Package, error) {
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("goark-orm: no Go package found in %s", dir)
	}
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		return pkgs[0], nil
	}
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Name == packageName {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("goark-orm: package %q not found in %s", packageName, dir)
}

func chooseParsedPackage(packagesByName map[string]*ast.Package, packageName string, dir string) (*ast.Package, error) {
	if len(packagesByName) == 0 {
		return nil, fmt.Errorf("goark-orm: no Go package found in %s", dir)
	}
	names := make([]string, 0, len(packagesByName))
	for name := range packagesByName {
		names = append(names, name)
	}
	sort.Strings(names)
	packageName = strings.TrimSpace(packageName)
	if packageName == "" {
		if len(names) != 1 {
			return nil, fmt.Errorf("goark-orm: multiple Go packages found in %s: %s", dir, strings.Join(names, ", "))
		}
		packageName = names[0]
	}
	pkg := packagesByName[packageName]
	if pkg == nil {
		return nil, fmt.Errorf("goark-orm: package %q not found in %s", packageName, dir)
	}
	return pkg, nil
}

func astPackageFromLoaded(fset *token.FileSet, pkg *packages.Package) *ast.Package {
	files := make(map[string]*ast.File, len(pkg.Syntax))
	for _, file := range pkg.Syntax {
		if !scanSourceFileAllowed(fset, file) {
			continue
		}
		filename := fset.Position(file.Package).Filename
		files[filename] = file
	}
	return &ast.Package{Name: pkg.Name, Files: files}
}

func collectLoadedSources(pkg *packages.Package, visited map[*packages.Package]struct{}, out *[]scanPackageSource) {
	if pkg == nil {
		return
	}
	if _, ok := visited[pkg]; ok {
		return
	}
	visited[pkg] = struct{}{}
	files := make([]*ast.File, 0, len(pkg.Syntax))
	for _, file := range pkg.Syntax {
		if scanSourceFileAllowed(pkg.Fset, file) {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return pkg.Fset.Position(files[i].Package).Filename < pkg.Fset.Position(files[j].Package).Filename
	})
	importNames := make(map[string]string, len(pkg.Imports))
	for importPath, imported := range pkg.Imports {
		if imported != nil {
			importNames[importPath] = imported.Name
		}
	}
	if len(files) > 0 {
		*out = append(*out, scanPackageSource{
			PackagePath: pkg.PkgPath,
			PackageName: pkg.Name,
			Files:       files,
			ImportNames: importNames,
		})
	}
	for _, imported := range pkg.Imports {
		collectLoadedSources(imported, visited, out)
	}
}

func packageBuildFlags(buildTags []string) []string {
	buildTags = uniqueConfigStrings(buildTags)
	if len(buildTags) == 0 {
		return nil
	}
	return []string{"-tags=" + strings.Join(buildTags, ",")}
}

func packageLoadError(pkg *packages.Package, dir string) error {
	if pkg != nil && len(pkg.Errors) > 0 {
		return fmt.Errorf("goark-orm: load package %s failed: %s", dir, pkg.Errors[0].Msg)
	}
	return fmt.Errorf("goark-orm: no Go package found in %s", dir)
}

func scanSourceFileAllowed(fset *token.FileSet, file *ast.File) bool {
	if file == nil {
		return false
	}
	filename := fset.Position(file.Package).Filename
	return scanSourceFileNameAllowed(filepath.Base(filename))
}

func scanSourceFileNameAllowed(name string) bool {
	return strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasPrefix(name, "zz_goark_orm_")
}

func importNameForPath(importPath string) string {
	base := path.Base(strings.Trim(importPath, `"`))
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	base = strings.ReplaceAll(base, "-", "_")
	return base
}
