package wwgo

import (
	"context"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"sync"
)

type Events[T any] struct {
	log     zerolog.Logger
	watched map[uuid.UUID]*WatchedEventsGroup[T]
	mut     sync.RWMutex
}

func NewEvents[T any](log zerolog.Logger) *Events[T] {
	return &Events[T]{
		log:     log,
		watched: map[uuid.UUID]*WatchedEventsGroup[T]{},
		mut:     sync.RWMutex{},
	}
}

type WatchedEventsGroup[T any] struct {
	eventNames []string
	done       chan *T
}

func (f *Events[T]) WatchForEvents(events []string) uuid.UUID {
	group := &WatchedEventsGroup[T]{
		eventNames: events,
		done:       make(chan *T),
	}
	id := uuid.New()
	f.mut.Lock()
	defer f.mut.Unlock()
	f.watched[id] = group
	return id
}

func (f *Events[T]) WatchForEvent(event string) uuid.UUID {
	return f.WatchForEvents([]string{event})
}

func (f *Events[T]) WaitForGroup(ctx context.Context, id uuid.UUID) *T {
	f.mut.RLock()
	group, ok := f.watched[id]
	f.mut.RUnlock()
	if !ok {
		return nil
	}

	select {
	case res := <-group.done:
		return res
	case <-ctx.Done():
		return nil
	}
}

func (f *Events[T]) CancelWatch(id uuid.UUID) {
	f.mut.RLock()
	group, ok := f.watched[id]
	f.mut.RUnlock()
	if !ok {
		return
	}

	f.mut.Lock()
	defer f.mut.Unlock()
	delete(f.watched, id)
	close(group.done)
}

func (f *Events[T]) TriggerEvent(eventName string, event *T) {
	f.mut.Lock()
	defer f.mut.Unlock()
	for id, group := range f.watched {
		if !SliceIncludes(group.eventNames, eventName) {
			continue
		}

		// Remove the file from the watchlist, and the whole group if it was the
		// last one.
		group.eventNames = DiffSlice(group.eventNames, []string{eventName})
		if len(group.eventNames) == 0 {
			select {
			case group.done <- event:
			default:
				// If nobody has started listening yet, we just carry on & close the
				// channel. They will get an immediate 'true' from the WaitForGroup.
			}

			delete(f.watched, id)
			close(group.done)
		}
	}
}

func (f *Events[T]) EventIsBeingWatched(eventName string) bool {
	f.mut.RLock()
	defer f.mut.RUnlock()
	for _, group := range f.watched {
		if SliceIncludes(group.eventNames, eventName) {
			return true
		}
	}
	return false
}
