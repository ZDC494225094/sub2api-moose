package customize

import "sync"

// Lifecycle hooks run after the persisted switch changes. Background modules
// also reconcile the database to observe changes from other server processes.
var lifecycle = struct {
	sync.Mutex
	next  int
	hooks map[int]func(string)
}{hooks: make(map[int]func(string))}

func RegisterLifecycle(hook func(string)) func() {
	lifecycle.Lock()
	lifecycle.next++
	id := lifecycle.next
	lifecycle.hooks[id] = hook
	lifecycle.Unlock()
	return func() { lifecycle.Lock(); delete(lifecycle.hooks, id); lifecycle.Unlock() }
}

func notifyLifecycle(id string) {
	lifecycle.Lock()
	hooks := make([]func(string), 0, len(lifecycle.hooks))
	for _, hook := range lifecycle.hooks {
		hooks = append(hooks, hook)
	}
	lifecycle.Unlock()
	for _, hook := range hooks {
		hook(id)
	}
}
