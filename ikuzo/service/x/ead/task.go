// Copyright 2020 Delving B.V.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ead

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/delving/hub3/ikuzo/domain/domainpb"
	"github.com/delving/hub3/ikuzo/service/x/revision"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskAlreadySubmitted = errors.New("task already submitted")
)

type ProcessingState string

const (
	StateSubmitted             ProcessingState = "submitted source EAD"
	StatePending                               = "pending processing"
	StateStarted                               = "started processing"
	StateProcessingDescription                 = "processing description"
	StateProcessingMetsFiles                   = "processing METS files"
	StateProcessingInventories                 = "processing and indexing inventories"
	StateInError                               = "stopped processing with error"
	StateCanceled                              = "canceled processing"
	StateFinished                              = "finished processing EAD"
	StateDeleted                               = "deleted EAD"
)

type Transition struct {
	State       ProcessingState   `json:"state"`
	Started     time.Time         `json:"started"`
	Finished    time.Time         `json:"finished"`
	Metrics     map[string]uint64 `json:"metrics,omitempty"`
	Duration    time.Duration     `json:"duration"`
	DurationFmt string            `json:"durationFmt"`
}

type Task struct {
	ID          string `json:"id"`
	Meta        *Meta
	InState     ProcessingState `json:"inState"`
	ErrorMsg    string          `json:"errorMsg"`
	Transitions []*Transition   `json:"transitions"`
	Interrupted bool
	s           *Service
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *zerolog.Logger
}

func (t *Task) finishState() *Transition {
	last := t.Transitions[len(t.Transitions)-1]
	last.Finished = time.Now()
	last.Duration = last.Finished.Sub(last.Started)
	last.DurationFmt = last.Duration.String()

	return last
}

func (t *Task) isActive() bool {
	inActiveStates := []ProcessingState{StateInError, StateCanceled, StateFinished}
	for _, state := range inActiveStates {
		if state == t.InState {
			return false
		}
	}

	return true
}

func (t *Task) dropOrphans(revision int32) error {
	m := &domainpb.IndexMessage{
		OrganisationID: t.Meta.OrgID,
		DatasetID:      t.Meta.DatasetID,
		Revision:       &domainpb.Revision{Number: revision},
		ActionType:     domainpb.ActionType_DROP_ORPHANS,
	}

	// publish message
	if err := t.s.index.Publish(context.Background(), m); err != nil {
		return err
	}

	return nil
}

func (t *Task) finishTask() {
	last := t.Transitions[len(t.Transitions)-1]
	last.Finished = time.Now()

	var startProcessing *Transition

	for _, transition := range t.Transitions {
		if transition.State == StatePending {
			startProcessing = transition
			break
		}
	}

	t.Meta.ProcessingDuration = last.Finished.Sub(startProcessing.Finished)
	t.Meta.ProcessingDurationFmt = t.Meta.ProcessingDuration.String()

	if err := t.Meta.repo.Add("rsc"); err != nil {
		t.finishWithError(err)
		return
	}

	hash, err := t.Meta.repo.Commit(fmt.Sprintf("finish processing task %s", t.ID), nil)
	if err != nil {
		t.finishWithError(err)
		return
	}

	if hash.String() != t.Meta.PublishedCommitID {
		// TODO(kiivihal): publish directly for now
		if err := t.indexChanged(t.Meta.PublishedCommitID, hash.String()); err != nil {
			t.finishWithError(err)
			return
		}

		t.Meta.PublishedCommitID = hash.String()
	}

	t.log().Info().
		Dur("processing", t.Meta.ProcessingDuration).
		Uint64("inventories", t.Meta.Clevels).
		Uint64("metsFiles", t.Meta.DaoLinks).
		Uint64("publishedToIndex", t.Meta.TotalRecordsPublished).
		Uint64("digitalObjects", t.Meta.DigitalObjects).
		Bool("created", t.Meta.Created).
		Uint64("metsRetrieveErrors", t.Meta.DaoErrors).
		Strs("metsErrorLinks ", t.Meta.DaoErrorLinks).
		Uint64("fileSize", t.Meta.FileSize).
		Str("revisionHash", t.Meta.PublishedCommitID).
		Uint64("recordsUpdated", t.Meta.RecordsUpdated).
		Uint64("recordsDeleted", t.Meta.RecordsDeleted).
		Msg("finished processing")

	t.moveState(StateFinished)
	t.s.m.incFinished()
	t.finishState()
}

func (t *Task) indexChanged(from, until string) error {
	stats, err := t.Meta.repo.PublishChanged(from, until, t.s.index)
	if err != nil {
		log.Printf("publish error")
		return err
	}

	atomic.AddUint64(&t.Meta.RecordsDeleted, uint64(stats.Deleted))
	atomic.AddUint64(&t.Meta.RecordsUpdated, uint64(stats.Updated))
	return nil
}

func (t *Task) log() *zerolog.Logger {
	if t.logger == nil {
		logger := log.With().
			Str("component", "hub3").
			Str("svc", "ead").
			Str("datasetID", t.Meta.DatasetID).
			Str("taskID", t.ID).
			Str("orgID", t.Meta.OrgID).
			Logger()

		t.logger = &logger
	}

	return t.logger
}

func (t *Task) moveState(state ProcessingState) {
	current := t.finishState()
	t.log().Info().Str("oldState", string(t.InState)).Str("newState", string(state)).Dur("dur", current.Duration).Msg("EAD state transition")

	t.InState = state
	t.Transitions = append(t.Transitions, &Transition{State: state, Started: time.Now()})
}

func (t *Task) finishWithError(err error) error {
	t.finishState()
	t.log().Error().Err(err).
		Str("taskState", string(t.InState)).Msg("stopped EAD task with error")
	t.moveState(StateInError)
	t.ErrorMsg = err.Error()

	t.s.m.incFailed()

	// expected errors so just log them and move on
	// returning an error here stops the worker
	return nil
}

func (t *Task) currentTransition() *Transition {
	return t.Transitions[len(t.Transitions)-1]
}

func (t *Task) Next() {
	switch t.InState {
	case StateSubmitted:
		t.moveState(StatePending)
	case StatePending:
		t.moveState(StateStarted)
	case StateStarted:
		t.moveState(StateProcessingDescription)
	case StateProcessingDescription:
		t.moveState(StateProcessingInventories)
	case StateProcessingInventories:
		t.moveState(StateFinished)
		t.finishTask()
	case StateInError:
		t.finishState()
	case StateCanceled:
		t.finishState()
		atomic.AddUint64(&t.s.m.Canceled, 1)
	}
}

func (s *Service) openRepository(orgID, datasetID string) (*revision.Repository, error) {
	repo, err := s.revision.OpenRepository(orgID, datasetID)
	if err != nil {
		if !errors.Is(err, revision.ErrRepositoryNotExists) {
			return nil, err
		}

		repo, err = s.revision.InitRepository(orgID, datasetID)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func (s *Service) NewTask(meta *Meta) (*Task, error) {
	task := &Task{
		ID:      xid.New().String(),
		s:       s,
		Meta:    meta,
		InState: StateSubmitted,
	}

	entry := &Transition{
		State:   StateSubmitted,
		Started: time.Now(),
		Metrics: map[string]uint64{
			"fileSize": meta.FileSize,
		},
	}
	task.Transitions = append(task.Transitions, entry)
	task.Next()

	task.ctx, task.cancel = context.WithCancel(context.Background())

	if _, err := s.findTask("", meta.DatasetID, true); !errors.Is(err, ErrTaskNotFound) {
		return nil, ErrTaskAlreadySubmitted
	}

	s.rw.Lock()
	s.tasks[task.ID] = task
	s.rw.Unlock()

	return task, nil
}
