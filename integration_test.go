package orm_test

import (
	"testing"

	"goark.dev/orm/ormtest"
)

func TestIntegrationDatabaseSuite_whenConfigured_shouldRunCompatibilityChecks(t *testing.T) {
	ormtest.RunDatabaseSuiteFromEnv(t)
}
