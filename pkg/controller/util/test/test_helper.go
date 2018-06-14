/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testutil

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	core "k8s.io/client-go/testing"

	"github.com/golang/glog"
)

const (
	pushTimeout = 5 * time.Second
)

// A structure that distributes events to multiple watchers.
type WatcherDispatcher struct {
	sync.Mutex
	watchers       []*watch.RaceFreeFakeWatcher
	eventsSoFar    []*watch.Event
	orderExecution chan func()
	stopChan       chan struct{}
}

func (wd *WatcherDispatcher) register(watcher *watch.RaceFreeFakeWatcher) {
	wd.Lock()
	defer wd.Unlock()
	wd.watchers = append(wd.watchers, watcher)
	for _, event := range wd.eventsSoFar {
		watcher.Action(event.Type, event.Object)
	}
}

func (wd *WatcherDispatcher) Stop() {
	wd.Lock()
	defer wd.Unlock()
	close(wd.stopChan)
	glog.Infof("Stopping WatcherDispatcher")
	for _, watcher := range wd.watchers {
		watcher.Stop()
	}
}

// Add sends an add event.
func (wd *WatcherDispatcher) Add(obj runtime.Object) {
	wd.Lock()
	defer wd.Unlock()
	wd.eventsSoFar = append(wd.eventsSoFar, &watch.Event{Type: watch.Added, Object: obj.DeepCopyObject()})
	for _, watcher := range wd.watchers {
		if !watcher.IsStopped() {
			watcher.Add(obj.DeepCopyObject())
		}
	}
}

// Modify sends a modify event.
func (wd *WatcherDispatcher) Modify(obj runtime.Object) {
	wd.Lock()
	defer wd.Unlock()
	glog.V(4).Infof("->WatcherDispatcher.Modify(%v)", obj)
	wd.eventsSoFar = append(wd.eventsSoFar, &watch.Event{Type: watch.Modified, Object: obj.DeepCopyObject()})
	for i, watcher := range wd.watchers {
		if !watcher.IsStopped() {
			glog.V(4).Infof("->Watcher(%d).Modify(%v)", i, obj)
			watcher.Modify(obj.DeepCopyObject())
		} else {
			glog.V(4).Infof("->Watcher(%d) is stopped.  Not calling Modify(%v)", i, obj)
		}
	}
}

// Delete sends a delete event.
func (wd *WatcherDispatcher) Delete(lastValue runtime.Object) {
	wd.Lock()
	defer wd.Unlock()
	wd.eventsSoFar = append(wd.eventsSoFar, &watch.Event{Type: watch.Deleted, Object: lastValue.DeepCopyObject()})
	for _, watcher := range wd.watchers {
		if !watcher.IsStopped() {
			watcher.Delete(lastValue.DeepCopyObject())
		}
	}
}

// Error sends an Error event.
func (wd *WatcherDispatcher) Error(errValue runtime.Object) {
	wd.Lock()
	defer wd.Unlock()
	wd.eventsSoFar = append(wd.eventsSoFar, &watch.Event{Type: watch.Error, Object: errValue.DeepCopyObject()})
	for _, watcher := range wd.watchers {
		if !watcher.IsStopped() {
			watcher.Error(errValue.DeepCopyObject())
		}
	}
}

// Action sends an event of the requested type, for table-based testing.
func (wd *WatcherDispatcher) Action(action watch.EventType, obj runtime.Object) {
	wd.Lock()
	defer wd.Unlock()
	wd.eventsSoFar = append(wd.eventsSoFar, &watch.Event{Type: action, Object: obj.DeepCopyObject()})
	for _, watcher := range wd.watchers {
		if !watcher.IsStopped() {
			watcher.Action(action, obj.DeepCopyObject())
		}
	}
}

// RegisterFakeWatch adds a new fake watcher for the specified resource in the given fake client.
// All subsequent requests for a watch on the client will result in returning this fake watcher.
func RegisterFakeWatch(resource string, client *core.Fake) *WatcherDispatcher {
	dispatcher := &WatcherDispatcher{
		watchers:       make([]*watch.RaceFreeFakeWatcher, 0),
		eventsSoFar:    make([]*watch.Event, 0),
		orderExecution: make(chan func(), 100),
		stopChan:       make(chan struct{}),
	}
	go func() {
		for {
			select {
			case fun := <-dispatcher.orderExecution:
				fun()
			case <-dispatcher.stopChan:
				return
			}
		}
	}()

	client.AddWatchReactor(resource, func(action core.Action) (bool, watch.Interface, error) {
		watcher := watch.NewRaceFreeFake()
		dispatcher.register(watcher)
		return true, watcher, nil
	})
	return dispatcher
}

// RegisterFakeList registers a list response for the specified resource inside the given fake client.
// The passed value will be returned with every list call.
func RegisterFakeList(resource string, client *core.Fake, obj runtime.Object) {
	client.AddReactor("list", resource, func(action core.Action) (bool, runtime.Object, error) {
		return true, obj, nil
	})
}

// RegisterFakeOnCreate registers a reactor in the given fake client that passes
// all created objects to the given watcher.
func RegisterFakeOnCreate(resource string, client *core.Fake, watcher *WatcherDispatcher) {
	client.AddReactor("create", resource, func(action core.Action) (bool, runtime.Object, error) {
		createAction := action.(core.CreateAction)
		originalObj := createAction.GetObject()
		// Create a copy of the object here to prevent data races while reading the object in go routine.
		obj := originalObj.DeepCopyObject()
		watcher.orderExecution <- func() {
			glog.V(4).Infof("Object created: %v", obj)
			watcher.Add(obj)
		}
		return true, originalObj, nil
	})
}

// RegisterFakeOnUpdate registers a reactor in the given fake client that passes
// all updated objects to the given watcher.
func RegisterFakeOnUpdate(resource string, client *core.Fake, watcher *WatcherDispatcher) {
	client.AddReactor("update", resource, func(action core.Action) (bool, runtime.Object, error) {
		updateAction := action.(core.UpdateAction)
		originalObj := updateAction.GetObject()
		glog.V(7).Infof("Updating %s: %v", resource, updateAction.GetObject())

		// Create a copy of the object here to prevent data races while reading the object in go routine.
		obj := originalObj.DeepCopyObject()
		operation := func() {
			glog.V(4).Infof("Object updated %v", obj)
			watcher.Modify(obj)
		}
		select {
		case watcher.orderExecution <- operation:
			break
		case <-time.After(pushTimeout):
			glog.Errorf("Fake client execution channel blocked")
			glog.Errorf("Tried to push %v", updateAction)
		}
		return true, originalObj, nil
	})
	return
}

// RegisterFakeOnDelete registers a reactor in the given fake client that passes
// all deleted objects to the given watcher. Since we could get only name of the
// deleted object from DeleteAction, this register function relies on the getObject
// function passed to get the object by name and pass it watcher.
func RegisterFakeOnDelete(resource string, client *core.Fake, watcher *WatcherDispatcher, getObject func(name, namespace string) runtime.Object) {
	client.AddReactor("delete", resource, func(action core.Action) (bool, runtime.Object, error) {
		deleteAction := action.(core.DeleteAction)
		obj := getObject(deleteAction.GetName(), deleteAction.GetNamespace())
		glog.V(7).Infof("Deleting %s: %v", resource, obj)

		operation := func() {
			glog.V(4).Infof("Object deleted %v", obj)
			watcher.Delete(obj)
		}
		select {
		case watcher.orderExecution <- operation:
			break
		case <-time.After(pushTimeout):
			glog.Errorf("Fake client execution channel blocked")
			glog.Errorf("Tried to push %v", deleteAction)
		}
		return true, obj, nil
	})
	return
}
