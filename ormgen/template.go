package ormgen

import (
	"context"
	"fmt"
)

// TemplateRenderer 渲染已经扫描或反向工程得到的包模型。
type TemplateRenderer interface {
	RenderPackage(model *PackageModel) ([]byte, error)
}

// TemplateRendererFunc 将函数适配为 TemplateRenderer。
type TemplateRendererFunc func(model *PackageModel) ([]byte, error)

// RenderPackage 执行函数式模板渲染器。
func (f TemplateRendererFunc) RenderPackage(model *PackageModel) ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("goark-orm: template renderer is nil")
	}
	return f(model)
}

// DefaultTemplateRenderer 返回内置 Go 源码模板渲染器。
func DefaultTemplateRenderer() TemplateRenderer {
	return TemplateRendererFunc(Render)
}

// GenerateWithRenderer 使用自定义模板渲染器生成源码。
func GenerateWithRenderer(spec GenerateSpec, renderer TemplateRenderer) ([]byte, error) {
	model, err := ScanPackage(spec)
	if err != nil {
		return nil, err
	}
	if renderer == nil {
		renderer = DefaultTemplateRenderer()
	}
	return renderer.RenderPackage(model)
}

// ReverseEngineerWithRenderer 读取 schema 并使用指定模板渲染。
func ReverseEngineerWithRenderer(ctx context.Context, introspector SchemaIntrospector, spec ReverseEngineerSpec, renderer TemplateRenderer) ([]byte, error) {
	model, err := ReverseEngineer(ctx, introspector, spec)
	if err != nil {
		return nil, err
	}
	if renderer == nil {
		renderer = DefaultTemplateRenderer()
	}
	return renderer.RenderPackage(model)
}
