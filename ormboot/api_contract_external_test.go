package ormboot_test

import (
	"database/sql"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/ormboot"
)

func TestPublicAPIContract_shouldCompileExternalUsage(t *testing.T) {
	t.Parallel()

	var _ ormboot.MetadataRegistrar = func(*orm.Registry) error { return nil }
	var _ = ormboot.Config{
		Name:               ormboot.DefaultName,
		BeanNames:          ormboot.BeanNames{SessionFactory: ormboot.BeanNameSessionFactory},
		DB:                 (*sql.DB)(nil),
		MyBatisConfig:      orm.DefaultMyBatisConfig(),
		MetadataRegistrars: []ormboot.MetadataRegistrar{func(*orm.Registry) error { return nil }},
	}
	var _ = ormboot.BeanRegistration{Name: ormboot.BeanNameRegistry, Instance: orm.NewRegistry()}
	var _ func(ormboot.Config) (*ormboot.Assembler, error) = ormboot.New

	if ormboot.BeanNameRuntime == "" || ormboot.BeanNameConfiguration == "" {
		t.Fatalf("default bean names must be stable")
	}
}
