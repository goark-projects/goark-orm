package runtime

import (
	"context"
	"testing"
)

func TestService_Upsert_whenRowsAffected_shouldReturnTrue(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	ok, err := service.Upsert(
		context.Background(),
		&baseMapperUser{ID: 7, Name: "Alice", Status: "ACTIVE"},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName, baseMapperUserStatus},
	)
	if err != nil {
		t.Fatalf("service upsert failed: %v", err)
	}

	if !ok {
		t.Fatalf("expected service upsert to report success")
	}
}

func TestService_UpsertBatchSize_whenRowsAffected_shouldReturnRows(t *testing.T) {
	state := openTestSQLState(t)
	state.execResult = testResult{rowsAffected: 1}
	session, err := NewSQLSession(NewRegistry(), state.db, NewPostgresDialect())
	if err != nil {
		t.Fatalf("new SQL session failed: %v", err)
	}
	mapper, err := NewBaseMapper[baseMapperUser, int64](session, baseMapperUserEntity())
	if err != nil {
		t.Fatalf("new base mapper failed: %v", err)
	}
	service, err := NewService[baseMapperUser, int64](mapper)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	rows, err := service.UpsertBatchSize(
		context.Background(),
		[]baseMapperUser{{ID: 7, Name: "Alice"}, {ID: 8, Name: "Bob"}},
		[]Field[baseMapperUser]{baseMapperUserID},
		[]Field[baseMapperUser]{baseMapperUserName},
		2,
	)
	if err != nil {
		t.Fatalf("service upsert batch failed: %v", err)
	}

	if rows != 2 {
		t.Fatalf("expected two affected rows, got %d", rows)
	}
}
