package ormboot

import (
	"context"
	"fmt"

	orm "goark.dev/orm"
)

// Assembler 将静态 ORM 元数据、运行期配置和数据源装配为可注册运行时。
type Assembler struct {
	config Config
}

// New 创建 ORM 装配器。
func New(config Config) (*Assembler, error) {
	normalized, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	return &Assembler{config: normalized}, nil
}

// Name 返回装配单元名称。
func (a *Assembler) Name() string {
	if a == nil {
		return ""
	}
	return a.config.Name
}

// Order 返回装配顺序，数值越小越先注册。
func (a *Assembler) Order() int {
	if a == nil {
		return 0
	}
	return a.config.Order
}

// Assemble 执行元数据注册并创建 ORM 运行时。
func (a *Assembler) Assemble(ctx context.Context) (*Runtime, error) {
	if a == nil {
		return nil, fmt.Errorf("goark-orm/ormboot: assembler is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("goark-orm/ormboot: context is nil")
	}
	for index, registrar := range a.config.MetadataRegistrars {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if registrar == nil {
			continue
		}
		if err := registrar(a.config.Registry); err != nil {
			return nil, fmt.Errorf("goark-orm/ormboot: register metadata %d failed: %w", index+1, err)
		}
	}
	result, err := orm.AssembleMyBatisConfig(orm.MyBatisAssembly{
		Config:         a.config.MyBatisConfig,
		Registry:       a.config.Registry,
		DB:             a.config.DB,
		TypeHandlers:   a.config.TypeHandlers,
		Plugins:        a.config.Plugins,
		SessionOptions: a.config.SessionOptions,
	})
	if err != nil {
		return nil, err
	}
	return &Runtime{
		name:      a.config.Name,
		beanNames: a.config.BeanNames,
		result:    result,
	}, nil
}
