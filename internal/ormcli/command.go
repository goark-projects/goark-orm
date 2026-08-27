package ormcli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"goark.dev/orm/ormgen"
)

// Version 是开发期版本号，正式发布时由构建流程覆盖。
const Version = "0.1.0-dev"

// Command 封装 goark-orm CLI 的输入输出边界。
type Command struct {
	Out     io.Writer
	Err     io.Writer
	Version string
}

type stringList []string

func (l *stringList) String() string {
	return strings.Join(*l, ",")
}

func (l *stringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// Main 执行 goark-orm CLI 主流程，并返回进程退出码。
func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	cmd := Command{
		Out:     stdout,
		Err:     stderr,
		Version: Version,
	}
	return cmd.Run(args)
}

// Run 根据首个参数分发命令。
func (c Command) Run(args []string) int {
	if c.Out == nil {
		c.Out = io.Discard
	}
	if c.Err == nil {
		c.Err = io.Discard
	}
	if c.Version == "" {
		c.Version = Version
	}
	if len(args) == 0 {
		c.printHelp(c.Out)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		c.printHelp(c.Out)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintf(c.Out, "goark-orm %s\n", c.Version)
		return 0
	case "generate", "gen":
		return c.runGenerate(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知命令: %s\n\n", args[0])
		c.printHelp(c.Err)
		return 2
	}
}

func (c Command) runGenerate(args []string) int {
	if len(args) == 0 {
		c.printGenerateHelp(c.Err)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		c.printGenerateHelp(c.Out)
		return 0
	case "orm":
		return c.runGenerateORM(args[1:])
	default:
		_, _ = fmt.Fprintf(c.Err, "未知生成器: %s\n\n", args[0])
		c.printGenerateHelp(c.Err)
		return 2
	}
}

func (c Command) runGenerateORM(args []string) int {
	var output string
	var configPath string
	var dryRun bool
	var validate bool
	var check bool
	var diff bool
	var typeHandlers stringList
	var buildTags stringList
	spec := ormgen.GenerateSpec{Dir: "."}
	flags := flag.NewFlagSet("goark-orm generate orm", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&configPath, "config", "", "JSON 配置文件路径")
	flags.StringVar(&spec.Dir, "dir", ".", "待扫描 Go package 目录")
	flags.StringVar(&spec.PackageName, "package", "", "待扫描 package 名称，默认自动推导")
	flags.StringVar(&output, "output", "", "输出文件路径，留空时输出到 stdout；扫描 ./... 时不允许指定")
	flags.BoolVar(&dryRun, "dry-run", false, "只扫描和渲染，不写入生成文件")
	flags.BoolVar(&validate, "validate", false, "只验证元数据和生成结果能否渲染")
	flags.BoolVar(&check, "check", false, "检查生成文件是否与当前元数据一致")
	flags.BoolVar(&diff, "diff", false, "输出生成文件与当前元数据的差异")
	flags.Var(&typeHandlers, "type-handler", "额外已注册 TypeHandler 名称，可重复")
	flags.Var(&buildTags, "build-tag", "额外 Go build tag，可重复")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.printGenerateORMHelp(c.Out)
			return 0
		}
		_, _ = fmt.Fprintf(c.Err, "%v\n\n", err)
		c.printGenerateORMHelp(c.Err)
		return 2
	}
	spec.TypeHandlers = append(spec.TypeHandlers, typeHandlers...)
	spec.BuildTags = append(spec.BuildTags, buildTags...)
	mode, err := generateORMModeFromFlags(dryRun, validate, check, diff)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	if configPath != "" {
		if flags.NArg() != 0 {
			_, _ = fmt.Fprint(c.Err, "--config 不能与 pattern 参数同时使用\n")
			return 2
		}
		return c.runGenerateORMConfig(configPath, mode)
	}
	switch flags.NArg() {
	case 0:
		return c.runGenerateORMSingle(spec, output, mode)
	case 1:
		pattern := flags.Arg(0)
		if strings.HasSuffix(pattern, "...") {
			if output != "" {
				_, _ = fmt.Fprint(c.Err, "扫描 ./... 时不能指定 --output\n")
				return 2
			}
			return c.runGenerateORMPattern(pattern, spec, mode)
		}
		spec.Dir = pattern
		return c.runGenerateORMSingle(spec, output, mode)
	default:
		_, _ = fmt.Fprintf(c.Err, "多余参数: %s\n\n", strings.Join(flags.Args(), " "))
		c.printGenerateORMHelp(c.Err)
		return 2
	}
}

func (c Command) runGenerateORMConfig(path string, mode generateORMMode) int {
	config, err := ormgen.LoadGenerateConfig(path)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	items, err := config.Resolve(filepath.Dir(path))
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	for _, item := range items {
		model, err := ormgen.ScanPackage(item.Spec)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n", err)
			return 2
		}
		source, err := ormgen.Render(model)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n", err)
			return 2
		}
		output := item.Output
		if output == "" {
			output = filepath.Join(model.Dir, ormgen.DefaultOutputName(model.PackageName))
		}
		if code := c.finishGeneratedFile(output, source, mode); code != 0 {
			return code
		}
	}
	return 0
}

func (c Command) runGenerateORMSingle(spec ormgen.GenerateSpec, output string, mode generateORMMode) int {
	model, err := ormgen.ScanPackage(spec)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	source, err := ormgen.Render(model)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	if output == "" && mode.requiresGeneratedFile() {
		output = filepath.Join(model.Dir, ormgen.DefaultOutputName(model.PackageName))
	}
	if output == "" {
		if mode == generateORMModeValidate {
			_, _ = fmt.Fprintf(c.Err, "validated %s\n", model.Dir)
			return 0
		}
		_, _ = c.Out.Write(source)
		return 0
	}
	return c.finishGeneratedFile(output, source, mode)
}

func (c Command) runGenerateORMPattern(pattern string, spec ormgen.GenerateSpec, mode generateORMMode) int {
	dirs, err := ormgen.DiscoverPackages(pattern)
	if err != nil {
		_, _ = fmt.Fprintf(c.Err, "%v\n", err)
		return 2
	}
	generated := 0
	for _, dir := range dirs {
		itemSpec := spec
		itemSpec.Dir = dir
		itemSpec.PackageName = ""
		model, err := ormgen.ScanPackage(itemSpec)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n", err)
			return 2
		}
		if len(model.Entities) == 0 && len(model.Mappers) == 0 {
			continue
		}
		source, err := ormgen.Render(model)
		if err != nil {
			_, _ = fmt.Fprintf(c.Err, "%v\n", err)
			return 2
		}
		output := filepath.Join(model.Dir, ormgen.DefaultOutputName(model.PackageName))
		if code := c.finishGeneratedFile(output, source, mode); code != 0 {
			return code
		}
		generated++
	}
	if generated == 0 {
		_, _ = fmt.Fprintf(c.Err, "no goark-orm metadata found for %s\n", pattern)
	}
	return 0
}

func writeFile(path string, data []byte) error {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c Command) printHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `goark-orm is the standalone command-line tool for Goark ORM.

Usage:
  goark-orm <command> [arguments]

Available commands:
  help              Show command help.
  version           Print the CLI version.
  generate, gen     Run ORM code generators.

`)
}

func (c Command) printGenerateHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark-orm generate <generator> [flags]
  goark-orm gen <generator> [flags]

Available generators:
  orm               Scan //goark-orm metadata and generate ORM code.

`)
}

func (c Command) printGenerateORMHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, `Usage:
  goark-orm generate orm [pattern] [flags]

Flags:
  --config path              JSON 配置文件路径。配置模式下会按配置批量生成文件。
  --dir path                 Go package directory to scan. Defaults to current directory.
  --package string           Package name to scan when directory contains multiple packages.
  --output path              Output file path for single package generation. Defaults to stdout.
  --dry-run                  Render but do not write generated files.
  --validate                 Validate metadata and generated source without writing files.
  --check                    Fail if generated files are missing or out of date.
  --diff                     Print a unified diff when generated files are out of date.
  --type-handler string      Extra registered TypeHandler name. Repeatable.
  --build-tag string         Extra Go build tag for scanning. Repeatable.

Examples:
  goark-orm generate orm --dir .
  goark-orm generate orm --config goark-orm.json
  goark-orm generate orm --dir internal/user --output internal/user/zz_goark_orm_user_gen.go
  goark-orm generate orm ./...

`)
}
