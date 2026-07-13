package store

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func revisionPtr(revision int) *int {
	return &revision
}

func casObservationParams(content string, expectedRevision int) AddObservationParams {
	return AddObservationParams{
		SessionID:        "s-cas",
		Type:             "architecture",
		Title:            "Canonical architecture",
		Content:          content,
		Project:          "engram",
		Scope:            "project",
		TopicKey:         "SDD / Canonical Head",
		ExpectedRevision: revisionPtr(expectedRevision),
	}
}

func TestAddObservationCASCreateIfAbsent(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("s-cas", "engram", "/tmp/engram"); err != nil {
		t.Fatalf("create session: %v", err)
	}

	id, err := s.AddObservation(casObservationParams("revision one", 0))
	if err != nil {
		t.Fatalf("create absent topic: %v", err)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("get created observation: %v", err)
	}
	if obs.RevisionCount != 1 {
		t.Fatalf("expected revision 1, got %d", obs.RevisionCount)
	}

	_, err = s.AddObservation(casObservationParams("must not replace", 0))
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected RevisionConflictError, got %v", err)
	}
	if conflict.ExpectedRevision != 0 || conflict.Current == nil {
		t.Fatalf("unexpected conflict metadata: %#v", conflict)
	}
	if conflict.Current.ID != id || conflict.Current.SyncID != obs.SyncID || conflict.Current.RevisionCount != 1 {
		t.Fatalf("unexpected current observation metadata: %#v", conflict.Current)
	}
}

func TestAddObservationCASMatchingRevisionUpdatesAndIncrements(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("s-cas", "engram", "/tmp/engram"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	id, err := s.AddObservation(casObservationParams("revision one", 0))
	if err != nil {
		t.Fatalf("create observation: %v", err)
	}

	updated, err := s.AddObservationWithResult(casObservationParams("revision two", 1))
	if err != nil {
		t.Fatalf("update matching revision: %v", err)
	}
	if updated.ID != id {
		t.Fatalf("expected update to reuse id %d, got %d", id, updated.ID)
	}
	if updated.SyncID == "" || updated.RevisionCount != 2 {
		t.Fatalf("unexpected committed metadata: %#v", updated)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("get updated observation: %v", err)
	}
	if obs.RevisionCount != 2 || obs.Content != "revision two" {
		t.Fatalf("unexpected updated observation: %#v", obs)
	}
}

func TestAddObservationCASStaleRevisionDoesNotMutateObservation(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession("s-cas", "engram", "/tmp/engram"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	id, err := s.AddObservation(casObservationParams("revision one", 0))
	if err != nil {
		t.Fatalf("create observation: %v", err)
	}
	if _, err := s.AddObservation(casObservationParams("revision two", 1)); err != nil {
		t.Fatalf("advance observation: %v", err)
	}

	_, err = s.AddObservation(casObservationParams("stale overwrite", 1))
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) || conflict.Current == nil || conflict.Current.RevisionCount != 2 {
		t.Fatalf("expected conflict at current revision 2, got %#v (%v)", conflict, err)
	}
	obs, err := s.GetObservation(id)
	if err != nil {
		t.Fatalf("get observation after conflict: %v", err)
	}
	if obs.RevisionCount != 2 || obs.Content != "revision two" {
		t.Fatalf("stale write mutated observation: %#v", obs)
	}
}

func TestAddObservationCASConcurrentWritersFromSameRevision(t *testing.T) {
	dataDir := t.TempDir()
	cfg := mustDefaultConfig(t)
	cfg.DataDir = dataDir
	cfg.DedupeWindow = time.Hour
	stores := make([]*Store, 2)
	for i := range stores {
		var err error
		stores[i], err = New(cfg)
		if err != nil {
			t.Fatalf("new store %d: %v", i+1, err)
		}
		store := stores[i]
		t.Cleanup(func() { _ = store.Close() })
	}

	if err := stores[0].CreateSession("s-cas", "engram", "/tmp/engram"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	id, err := stores[0].AddObservation(casObservationParams("revision one", 0))
	if err != nil {
		t.Fatalf("create observation: %v", err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, content := range []string{"writer one", "writer two"} {
		wg.Add(1)
		go func(s *Store, content string) {
			defer wg.Done()
			<-start
			_, err := s.AddObservation(casObservationParams(content, 1))
			errs <- err
		}(stores[i], content)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, conflicts int
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		var conflict *RevisionConflictError
		if errors.As(err, &conflict) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent write error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got successes=%d conflicts=%d", successes, conflicts)
	}
	obs, err := stores[0].GetObservation(id)
	if err != nil {
		t.Fatalf("get observation: %v", err)
	}
	if obs.RevisionCount != 2 || (obs.Content != "writer one" && obs.Content != "writer two") {
		t.Fatalf("unexpected final observation: %#v", obs)
	}
}

func TestAddObservationCASValidation(t *testing.T) {
	s := newTestStore(t)

	tests := []struct {
		name    string
		params  AddObservationParams
		wantErr error
	}{
		{name: "topic key required", params: AddObservationParams{ExpectedRevision: revisionPtr(0)}, wantErr: ErrExpectedRevisionTopic},
		{name: "non-negative revision required", params: AddObservationParams{TopicKey: "topic", ExpectedRevision: revisionPtr(-1)}, wantErr: ErrInvalidExpectedRevision},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.AddObservation(tt.params); !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}
