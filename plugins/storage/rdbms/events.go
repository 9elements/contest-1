// Copyright (c) Facebook, Inc. and its affiliates.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package rdbms

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/linuxboot/contest/pkg/event"
	"github.com/linuxboot/contest/pkg/event/frameworkevent"
	"github.com/linuxboot/contest/pkg/event/testevent"
	"github.com/linuxboot/contest/pkg/job"
	"github.com/linuxboot/contest/pkg/target"
	"github.com/linuxboot/contest/pkg/types"
	"github.com/linuxboot/contest/pkg/xcontext"

	"github.com/google/go-safeweb/safesql"
)

func assembleQuery(baseQuery safesql.TrustedSQLString, selectClauses []safesql.TrustedSQLString) (safesql.TrustedSQLString, error) {
	if len(selectClauses) == 0 {
		return safesql.New(""), fmt.Errorf("no select clauses available, the query should specify at least one clause")
	}
	initialClause := true
	for _, clause := range selectClauses {
		if initialClause {
			baseQuery = safesql.TrustedSQLStringConcat(baseQuery, safesql.New(" where "), clause)
			initialClause = false
		} else {
			baseQuery = safesql.TrustedSQLStringConcat(baseQuery, safesql.New(" and "), clause)
		}
	}
	return baseQuery, nil
}

func buildEventQuery(baseQuery safesql.TrustedSQLString, eventQuery *event.Query) ([]safesql.TrustedSQLString, []interface{}) {
	selectClauses := []safesql.TrustedSQLString{}
	fields := []interface{}{}

	if eventQuery != nil && eventQuery.JobID != 0 {
		selectClauses = append(selectClauses, safesql.New("job_id=?"))
		fields = append(fields, eventQuery.JobID)
	}

	if eventQuery != nil && len(eventQuery.EventNames) != 0 {
		if len(eventQuery.EventNames) == 1 {
			selectClauses = append(selectClauses, safesql.New("event_name=?"))
		} else {
			var queryStr safesql.TrustedSQLString
			queryStr = safesql.New("event_name in")
			for i := 0; i < len(eventQuery.EventNames); i++ {
				if i == 0 {
					queryStr = safesql.TrustedSQLStringConcat(queryStr, safesql.New(" (?"))
				} else if i < len(eventQuery.EventNames)-1 {
					queryStr = safesql.TrustedSQLStringConcat(queryStr, safesql.New(", ?"))
				} else {
					queryStr = safesql.TrustedSQLStringConcat(queryStr, safesql.New(", ?)"))
				}
			}
			selectClauses = append(selectClauses, queryStr)
		}
		for i := 0; i < len(eventQuery.EventNames); i++ {
			fields = append(fields, eventQuery.EventNames[i])
		}
	}
	if eventQuery != nil && !eventQuery.EmittedStartTime.IsZero() {
		selectClauses = append(selectClauses, safesql.New("emit_time>=?"))
		fields = append(fields, eventQuery.EmittedStartTime)
	}
	if eventQuery != nil && !eventQuery.EmittedEndTime.IsZero() {
		selectClauses = append(selectClauses, safesql.New("emit_time<=?"))
		fields = append(fields, eventQuery.EmittedStartTime)
	}
	return selectClauses, fields
}

func buildFrameworkEventQuery(baseQuery safesql.TrustedSQLString, frameworkEventQuery *frameworkevent.Query) (safesql.TrustedSQLString, []interface{}, error) {
	selectClauses, fields := buildEventQuery(baseQuery, &frameworkEventQuery.Query)
	query, err := assembleQuery(baseQuery, selectClauses)
	if err != nil {
		return safesql.New(""), nil, fmt.Errorf("could not assemble query for framework events: %v", err)

	}
	return query, fields, nil
}

func buildTestEventQuery(baseQuery safesql.TrustedSQLString, testEventQuery *testevent.Query) (safesql.TrustedSQLString, []interface{}, error) {

	if testEventQuery == nil {
		return safesql.New(""), nil, fmt.Errorf("cannot build empty testevent query")
	}
	selectClauses, fields := buildEventQuery(baseQuery, &testEventQuery.Query)

	if testEventQuery.RunID != types.RunID(0) {
		selectClauses = append(selectClauses, safesql.New("run_id=?"))
		fields = append(fields, testEventQuery.RunID)
	}

	if testEventQuery.TestName != "" {
		selectClauses = append(selectClauses, safesql.New("test_name=?"))
		fields = append(fields, testEventQuery.TestName)
	}
	if testEventQuery.TestStepLabel != "" {
		selectClauses = append(selectClauses, safesql.New("test_step_label=?"))
		fields = append(fields, testEventQuery.TestStepLabel)
	}
	query, err := assembleQuery(baseQuery, selectClauses)
	if err != nil {
		return safesql.New(""), nil, fmt.Errorf("could not assemble query for framework events: %v", err)

	}
	return query, fields, nil
}

// TestEventField is a function type which retrieves information from a TestEvent object.
type TestEventField func(ev testevent.Event) interface{}

// TestEventJobID returns the JobID from an events.TestEvent object
func TestEventJobID(ev testevent.Event) interface{} {
	if ev.Header == nil {
		return nil
	}
	return ev.Header.JobID
}

// TestEventRunID returns the RunID from a
func TestEventRunID(ev testevent.Event) interface{} {
	if ev.Header == nil {
		return nil
	}
	return ev.Header.RunID
}

// TestEventTestName returns the test name from an events.TestEvent object
func TestEventTestName(ev testevent.Event) interface{} {
	if ev.Header == nil {
		return nil
	}
	return ev.Header.TestName
}

// TestEventTestAttempt returns the test retry from an events.TestEvent object
func TestEventTestAttempt(ev testevent.Event) interface{} {
	if ev.Header == nil {
		return nil
	}
	return ev.Header.TestAttempt
}

// TestEventTestStepLabel returns the test step label from an events.TestEvent object
func TestEventTestStepLabel(ev testevent.Event) interface{} {
	if ev.Header == nil {
		return nil
	}
	return ev.Header.TestStepLabel
}

// TestEventName returns the event name from an events.TestEvent object
func TestEventName(ev testevent.Event) interface{} {
	if ev.Data == nil {
		return nil
	}
	return ev.Data.EventName
}

// TestEventTargetID returns the target id from an events.TestEvent object
func TestEventTargetID(ev testevent.Event) interface{} {
	if ev.Data == nil || ev.Data.Target == nil {
		return nil
	}
	return ev.Data.Target.ID
}

// TestEventPayload returns the payload from an events.TestEvent object
func TestEventPayload(ev testevent.Event) interface{} {
	if ev.Data == nil {
		return nil
	}
	return ev.Data.Payload
}

// TestEventEmitTime returns the emission timestamp from an events.TestEvent object
func TestEventEmitTime(ev testevent.Event) interface{} {
	return ev.EmitTime
}

// StoreTestEvent appends an event to the internal buffer and triggers a flush
// when the internal storage utilization goes beyond `testEventsFlushSize`
func (r *RDBMS) StoreTestEvent(_ xcontext.Context, event testevent.Event) error {

	defer r.testEventsLock.Unlock()
	r.testEventsLock.Lock()

	r.buffTestEvents = append(r.buffTestEvents, event)
	if len(r.buffTestEvents) >= r.testEventsFlushSize {
		return r.flushTestEventsLocked()
	}
	return nil
}

// flushTestEventsLocked forces a flush of the pending test events to the database.
// Requires that the caller has already locked the corresponding buffer.
func (r *RDBMS) flushTestEventsLocked() error {

	r.lockTx()
	defer r.unlockTx()

	insertStatement := safesql.New("insert into test_events (job_id, run_id, test_name, test_attempt, test_step_label, event_name, target_id, payload, emit_time) values (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	for _, event := range r.buffTestEvents {
		_, err := r.db.Exec(
			insertStatement,
			TestEventJobID(event),
			TestEventRunID(event),
			TestEventTestName(event),
			TestEventTestAttempt(event),
			TestEventTestStepLabel(event),
			TestEventName(event),
			TestEventTargetID(event),
			TestEventPayload(event),
			TestEventEmitTime(event))
		if err != nil {
			return fmt.Errorf("could not store event in database: %v", err)
		}
	}
	r.buffTestEvents = nil

	return nil
}

// flushTestEvents forces a flush of the pending test events to the database.
func (r *RDBMS) flushTestEvents() error {
	r.testEventsLock.Lock()
	defer r.testEventsLock.Unlock()
	return r.flushTestEventsLocked()
}

// GetTestEvents retrieves test events matching the query fields provided
func (r *RDBMS) GetTestEvents(ctx xcontext.Context, eventQuery *testevent.Query) ([]testevent.Event, error) {

	// Flush pending events before Get operations
	err := r.flushTestEvents()

	if err != nil {
		return nil, fmt.Errorf("could not flush events before reading events: %v", err)
	}

	r.lockTx()
	defer r.unlockTx()

	const baseQuery = "select event_id, job_id, run_id, test_name, test_attempt, test_step_label, event_name, target_id, payload, emit_time from test_events"
	query, fields, err := buildTestEventQuery(safesql.New(baseQuery), eventQuery)
	if err != nil {
		return nil, fmt.Errorf("could not execute select query for test events: %v", err)
	}

	var results []testevent.Event
	ctx.Debugf("Executing query: %s, fields: %v", query, fields)
	rows, err := r.db.Query(query, fields...)
	if err != nil {
		return nil, err
	}

	// TargetID might be null, so a type which supports null should be used with Scan
	var (
		targetID sql.NullString
		payload  sql.NullString
	)

	defer func() {
		err := rows.Close()
		if err != nil {
			ctx.Warnf("could not close rows for test events: %v", err)
		}
	}()
	for rows.Next() {
		data := testevent.Data{}
		header := testevent.Header{}
		event := testevent.New(&header, &data)

		var eventID int
		err := rows.Scan(
			&eventID,
			&header.JobID,
			&header.RunID,
			&header.TestName,
			&header.TestAttempt,
			&header.TestStepLabel,
			&data.EventName,
			&targetID,
			&payload,
			&event.EmitTime,
		)
		if err != nil {
			return nil, fmt.Errorf("could not read results from db: %v", err)
		}
		if targetID.Valid {
			t := target.Target{ID: targetID.String}
			data.Target = &t
		}

		if payload.Valid {
			rawPayload := json.RawMessage(payload.String)
			data.Payload = &rawPayload

		}

		results = append(results, event)
	}
	return results, nil
}

// FrameworkEventField is a function type which retrieves information from a FrameworkEvent object
type FrameworkEventField func(ev frameworkevent.Event) interface{}

// FrameworkEventJobID returns the JobID from a events.TestEvent object
func FrameworkEventJobID(ev frameworkevent.Event) interface{} {
	return ev.JobID
}

// FrameworkEventName returns the name for the FrameworkEvent object
func FrameworkEventName(ev frameworkevent.Event) interface{} {
	return ev.EventName
}

// FrameworkEventPayload returns the payload from a events.FrameworkEvent object
func FrameworkEventPayload(ev frameworkevent.Event) interface{} {
	return ev.Payload
}

// FrameworkEventEmitTime returns the emission timestamp from a events.FrameworkEvent object
func FrameworkEventEmitTime(ev frameworkevent.Event) interface{} {
	return ev.EmitTime
}

// StoreFrameworkEvent appends an event to the internal buffer and triggers a flush
// when the internal storage utilization goes beyond `frameworkEventsFlushSize`
func (r *RDBMS) StoreFrameworkEvent(ctx xcontext.Context, event frameworkevent.Event) error {

	defer r.frameworkEventsLock.Unlock()
	r.frameworkEventsLock.Lock()

	r.buffFrameworkEvents = append(r.buffFrameworkEvents, event)
	if len(r.buffFrameworkEvents) >= r.frameworkEventsFlushSize {
		return r.flushFrameworkEventsLocked()
	}

	return nil
}

const (
	insertFEStmt       = "INSERT INTO framework_events (job_id, event_name, payload, emit_time) VALUES (?, ?, ?, ?)"
	updateJobStateStmt = "UPDATE jobs SET state = ? WHERE job_id = ?"
)

// flushFrameworkEventsLocked forces a flush of the pending frameworks events to the database
// Requires that the caller has already locked the corresponding buffer.
func (r *RDBMS) flushFrameworkEventsLocked() error {
	r.lockTx()
	defer r.unlockTx()

	// TODO: put this into a transaction.
	jobStateUpdates := map[types.JobID]job.State{}
	for _, event := range r.buffFrameworkEvents {
		_, err := r.db.Exec(
			safesql.New(insertFEStmt),
			FrameworkEventJobID(event),
			FrameworkEventName(event),
			FrameworkEventPayload(event),
			FrameworkEventEmitTime(event))
		if err != nil {
			return fmt.Errorf("could not store event in database: %v", err)
		}
		if sn, err := job.EventNameToJobState(event.EventName); err == nil {
			jobStateUpdates[event.JobID] = sn
		}
	}
	for jobID, state := range jobStateUpdates {
		if _, err := r.db.Exec(safesql.New(updateJobStateStmt), state, jobID); err != nil {
			return fmt.Errorf("could not update state of job %d: %w", jobID, err)
		}
	}
	r.buffFrameworkEvents = nil
	return nil
}

// flushFrameworkEvents forces a flush of the pending frameworks events to the database
func (r *RDBMS) flushFrameworkEvents() error {
	r.frameworkEventsLock.Lock()
	defer r.frameworkEventsLock.Unlock()
	return r.flushFrameworkEventsLocked()
}

// GetFrameworkEvent retrieves framework events matching the query fields provided
func (r *RDBMS) GetFrameworkEvent(ctx xcontext.Context, eventQuery *frameworkevent.Query) ([]frameworkevent.Event, error) {

	// Flush pending events before Get operations
	err := r.flushFrameworkEvents()
	if err != nil {
		return nil, fmt.Errorf("could not flush events before reading events: %v", err)
	}

	r.lockTx()
	defer r.unlockTx()

	baseQuery := safesql.New("select event_id, job_id, event_name, payload, emit_time from framework_events")
	query, fields, err := buildFrameworkEventQuery(baseQuery, eventQuery)
	if err != nil {
		return nil, fmt.Errorf("could not execute select query for test events: %v", err)
	}
	results := []frameworkevent.Event{}
	ctx.Debugf("Executing query: %s, fields: %v", query, fields)
	rows, err := r.db.Query(query, fields...)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			ctx.Warnf("could not close rows for framework events: %v", err)
		}
	}()

	for rows.Next() {
		event := frameworkevent.New()
		err := rows.Scan(&event.ID, &event.JobID, &event.EventName, &event.Payload, &event.EmitTime)
		if err != nil {
			return nil, fmt.Errorf("could not read results from db: %v", err)
		}
		results = append(results, event)
	}
	return results, nil
}
