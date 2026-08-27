package audit_test

import (
	"context"
	"errors"
	"testing"

	orm "goark.dev/orm"
	"goark.dev/orm/audit"
)

func TestMiddleware_whenExecSucceeds_shouldRecordDataChangeEvent(t *testing.T) {
	recorder := &recordingRecorder{}
	next := &fakeStatementExecutor{
		result: orm.Result{RowsAffected: 3, LastInsertID: 9},
	}
	executor := audit.NewMiddleware(recorder).WrapStatementExecutor(next)
	meta := auditTestStatement(orm.StatementCommandUpdate)

	result, err := executor.Exec(context.Background(), nil, meta, orm.NamedArgs{"id": int64(7)})
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if result.RowsAffected != 3 || result.LastInsertID != 9 {
		t.Fatalf("unexpected result %#v", result)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %d", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Operation != audit.OperationExec ||
		event.Command != orm.StatementCommandUpdate ||
		event.Namespace != "audit.UserMapper" ||
		event.ID != "UpdateName" ||
		event.FullName != "audit.UserMapper.UpdateName" {
		t.Fatalf("unexpected event metadata %#v", event)
	}
	if event.RowsAffected != 3 || event.LastInsertID != 9 {
		t.Fatalf("unexpected event result %#v", event)
	}
	if event.StartedAt.IsZero() || event.Duration < 0 {
		t.Fatalf("expected timing fields to be populated: %#v", event)
	}
	if event.Err != nil || !event.Success() {
		t.Fatalf("expected success event: %#v", event)
	}
}

func TestMiddleware_whenPlainQueryByDefault_shouldSkipEvent(t *testing.T) {
	recorder := &recordingRecorder{}
	executor := audit.NewMiddleware(recorder).WrapStatementExecutor(&fakeStatementExecutor{})
	meta := auditTestStatement(orm.StatementCommandSelect)

	if err := executor.Query(context.Background(), nil, meta, nil, &[]auditUser{}); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("expected no events, got %#v", recorder.events)
	}
}

func TestMiddleware_whenAffectDataQuery_shouldRecordByDefault(t *testing.T) {
	recorder := &recordingRecorder{}
	executor := audit.NewMiddleware(recorder).WrapStatementExecutor(&fakeStatementExecutor{})
	meta := auditTestStatement(orm.StatementCommandSelect)
	meta.AffectData = true

	if err := executor.QueryOne(context.Background(), nil, meta, nil, &auditUser{}); err != nil {
		t.Fatalf("query one failed: %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %d", len(recorder.events))
	}
	if recorder.events[0].Operation != audit.OperationQueryOne || !recorder.events[0].AffectData {
		t.Fatalf("unexpected event %#v", recorder.events[0])
	}
}

func TestMiddleware_whenQueryEventsEnabled_shouldRecordReadEvent(t *testing.T) {
	recorder := &recordingRecorder{}
	executor := audit.NewMiddleware(recorder, audit.WithQueryEvents(true)).WrapStatementExecutor(&fakeStatementExecutor{})
	meta := auditTestStatement(orm.StatementCommandSelect)

	if err := executor.Query(context.Background(), nil, meta, nil, &[]auditUser{}); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %d", len(recorder.events))
	}
	if recorder.events[0].Operation != audit.OperationQuery {
		t.Fatalf("unexpected operation %q", recorder.events[0].Operation)
	}
}

func TestMiddleware_whenRecorderFails_shouldReturnRecorderError(t *testing.T) {
	recorderErr := errors.New("record failed")
	recorder := &recordingRecorder{err: recorderErr}
	executor := audit.NewMiddleware(recorder).WrapStatementExecutor(&fakeStatementExecutor{})
	meta := auditTestStatement(orm.StatementCommandDelete)

	_, err := executor.Exec(context.Background(), nil, meta, nil)
	if !errors.Is(err, recorderErr) {
		t.Fatalf("expected recorder error, got %v", err)
	}
}

func TestMiddleware_whenOperationAndRecorderFail_shouldJoinErrors(t *testing.T) {
	operationErr := errors.New("exec failed")
	recorderErr := errors.New("record failed")
	recorder := &recordingRecorder{err: recorderErr}
	executor := audit.NewMiddleware(recorder).WrapStatementExecutor(&fakeStatementExecutor{execErr: operationErr})
	meta := auditTestStatement(orm.StatementCommandDelete)

	_, err := executor.Exec(context.Background(), nil, meta, nil)
	if !errors.Is(err, operationErr) || !errors.Is(err, recorderErr) {
		t.Fatalf("expected joined operation and recorder errors, got %v", err)
	}
	if len(recorder.events) != 1 || recorder.events[0].Err == nil || recorder.events[0].Success() {
		t.Fatalf("expected failure event, got %#v", recorder.events)
	}
}

func TestMiddleware_whenRecorderErrorIgnored_shouldKeepOperationResult(t *testing.T) {
	recorderErr := errors.New("record failed")
	recorder := &recordingRecorder{err: recorderErr}
	executor := audit.NewMiddleware(recorder, audit.WithIgnoreRecorderError(true)).WrapStatementExecutor(&fakeStatementExecutor{
		result: orm.Result{RowsAffected: 1},
	})
	meta := auditTestStatement(orm.StatementCommandInsert)

	result, err := executor.Exec(context.Background(), nil, meta, nil)
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if result.RowsAffected != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestMiddleware_whenErrorEventsDisabled_shouldSkipFailedOperation(t *testing.T) {
	operationErr := errors.New("exec failed")
	recorder := &recordingRecorder{}
	executor := audit.NewMiddleware(recorder, audit.WithErrorEvents(false)).WrapStatementExecutor(&fakeStatementExecutor{execErr: operationErr})
	meta := auditTestStatement(orm.StatementCommandUpdate)

	_, err := executor.Exec(context.Background(), nil, meta, nil)
	if !errors.Is(err, operationErr) {
		t.Fatalf("expected operation error, got %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("expected no events, got %#v", recorder.events)
	}
}

func TestMiddleware_whenSkipFuncMatches_shouldSkipEvent(t *testing.T) {
	recorder := &recordingRecorder{}
	executor := audit.NewMiddleware(recorder, audit.WithSkipFunc(func(event audit.Event) bool {
		return event.Namespace == "audit.UserMapper"
	})).WrapStatementExecutor(&fakeStatementExecutor{})
	meta := auditTestStatement(orm.StatementCommandUpdate)

	if _, err := executor.Exec(context.Background(), nil, meta, nil); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("expected no events, got %#v", recorder.events)
	}
}

func TestRecorderFunc_shouldAdaptFunction(t *testing.T) {
	called := false
	recorder := audit.RecorderFunc(func(context.Context, audit.Event) error {
		called = true
		return nil
	})

	if err := recorder.Record(context.Background(), audit.Event{}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	if !called {
		t.Fatalf("expected recorder function to be called")
	}
}

type recordingRecorder struct {
	events []audit.Event
	err    error
}

func (r *recordingRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return r.err
}

type fakeStatementExecutor struct {
	result      orm.Result
	queryErr    error
	queryOneErr error
	execErr     error
}

func (e *fakeStatementExecutor) Query(context.Context, *orm.SQLSession, orm.StatementMeta, orm.NamedArgs, any) error {
	return e.queryErr
}

func (e *fakeStatementExecutor) QueryOne(context.Context, *orm.SQLSession, orm.StatementMeta, orm.NamedArgs, any) error {
	return e.queryOneErr
}

func (e *fakeStatementExecutor) Exec(context.Context, *orm.SQLSession, orm.StatementMeta, orm.NamedArgs) (orm.Result, error) {
	return e.result, e.execErr
}

type auditUser struct {
	ID int64
}

func auditTestStatement(command orm.StatementCommand) orm.StatementMeta {
	return orm.StatementMeta{
		ID:        "UpdateName",
		Namespace: "audit.UserMapper",
		FullName:  "audit.UserMapper.UpdateName",
		Command:   command,
	}
}
