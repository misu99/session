// Copyright 2014 beego Author. All Rights Reserved.
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

package memory

import (
	"container/list"
	"errors"
	"github.com/misu99/session/store"
	"sync"
	"time"
)

//var memPdr = &ProviderMem{list: list.New(), sessions: make(map[string]*list.Element)}

// SessionStoreMem memory session store.
// it saved sessions in a map in memory.
type SessionStoreMem struct {
	sid          string                      //session id
	timeAccessed time.Time                   //last access time
	values       map[interface{}]interface{} //session store
	lock         sync.RWMutex
}

// Set values to memory session
func (st *SessionStoreMem) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values[key] = value
	return nil
}

// Get values from memory session by key
func (st *SessionStoreMem) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.values[key]; ok {
		return v
	}
	return nil
}

// Delete in memory session by key
func (st *SessionStoreMem) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.values, key)
	return nil
}

// Flush clear all values in memory session
func (st *SessionStoreMem) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values = make(map[interface{}]interface{})
	return nil
}

// SessionID get this id of memory session store
func (st *SessionStoreMem) SessionID() string {
	return st.sid
}

// SessionRelease Implement method, no used.
func (st *SessionStoreMem) SessionRelease() {
}

// ProviderMem Implement the provider interface
type ProviderMem struct {
	lock     sync.RWMutex             // locker
	sessions map[string]*list.Element // map in memory
	list     *list.List               // for gc
	lifetime int64
	savePath string
}

// SessionInit init memory session
func (pdr *ProviderMem) SessionInit(lifetime int64, savePath string) error {
	pdr.lifetime = lifetime
	pdr.savePath = savePath
	return nil
}

// create new memory session by sid
func (pdr *ProviderMem) SessionNew(sid string) (store.Store, error) {
	pdr.lock.RLock()
	if element, ok := pdr.sessions[sid]; ok {
		go pdr.SessionUpdate(sid)
		pdr.lock.RUnlock()
		return element.Value.(*SessionStoreMem), nil
	}
	pdr.lock.RUnlock()
	pdr.lock.Lock()
	newSess := &SessionStoreMem{sid: sid, timeAccessed: time.Now(), values: make(map[interface{}]interface{})}
	element := pdr.list.PushFront(newSess)
	pdr.sessions[sid] = element
	pdr.lock.Unlock()
	return newSess, nil
}

// SessionRead get memory session store by sid
func (pdr *ProviderMem) SessionRead(sid string) (store.Store, error) {
	pdr.lock.RLock()
	if element, ok := pdr.sessions[sid]; ok {
		go pdr.SessionUpdate(sid)
		pdr.lock.RUnlock()
		return element.Value.(*SessionStoreMem), nil
	}
	pdr.lock.RUnlock()
	//pdr.lock.Lock()
	//newsess := &SessionStoreMem{sid: sid, timeAccessed: time.Now(), values: make(map[interface{}]interface{})}
	//element := pdr.list.PushFront(newsess)
	//pdr.sessions[sid] = element
	//pdr.lock.Unlock()
	//return newsess, nil

	return nil, errors.New("the sid's session not found")
}

// SessionExist check session store exist in memory session by sid
func (pdr *ProviderMem) SessionExist(sid string) bool {
	pdr.lock.RLock()
	defer pdr.lock.RUnlock()
	if _, ok := pdr.sessions[sid]; ok {
		return true
	}
	return false
}

// SessionRegenerate generate new sid for session store in memory session
func (pdr *ProviderMem) SessionRegenerate(oldSid, sid string) (store.Store, error) {
	pdr.lock.RLock()
	if element, ok := pdr.sessions[oldSid]; ok {
		go pdr.SessionUpdate(oldSid)
		pdr.lock.RUnlock()
		pdr.lock.Lock()
		element.Value.(*SessionStoreMem).sid = sid
		pdr.sessions[sid] = element

		if oldSid != sid {
			delete(pdr.sessions, oldSid)
		}

		pdr.lock.Unlock()
		return element.Value.(*SessionStoreMem), nil
	}
	pdr.lock.RUnlock()
	pdr.lock.Lock()
	newSess := &SessionStoreMem{sid: sid, timeAccessed: time.Now(), values: make(map[interface{}]interface{})}
	element := pdr.list.PushFront(newSess)
	pdr.sessions[sid] = element
	pdr.lock.Unlock()
	return newSess, nil
}

// SessionDestroy delete session store in memory session by id
func (pdr *ProviderMem) SessionDestroy(sid string) error {
	pdr.lock.Lock()
	defer pdr.lock.Unlock()
	if element, ok := pdr.sessions[sid]; ok {
		delete(pdr.sessions, sid)
		pdr.list.Remove(element)
		return nil
	}
	return nil
}

// SessionGC clean expired session stores in memory session
func (pdr *ProviderMem) SessionGC() {
	pdr.lock.RLock()
	for {
		element := pdr.list.Back()
		if element == nil {
			break
		}
		if (element.Value.(*SessionStoreMem).timeAccessed.Unix() + pdr.lifetime) < time.Now().Unix() {
			pdr.lock.RUnlock()
			pdr.lock.Lock()
			pdr.list.Remove(element)
			delete(pdr.sessions, element.Value.(*SessionStoreMem).sid)
			pdr.lock.Unlock()
			pdr.lock.RLock()
		} else {
			break
		}
	}
	pdr.lock.RUnlock()
}

// SessionAll id values in mysql session
func (pdr *ProviderMem) SessionAll() ([]string, error) {
	var keys []string
	for key := range pdr.sessions {
		keys = append(keys, key)
	}
	return keys, nil
}

// SessionUpdate expand time of session store by id in memory session
func (pdr *ProviderMem) SessionUpdate(sid string) {
	pdr.lock.Lock()
	defer pdr.lock.Unlock()
	if element, ok := pdr.sessions[sid]; ok {
		element.Value.(*SessionStoreMem).timeAccessed = time.Now()
		pdr.list.MoveToFront(element)
	}
}

//func init() {
//	session.Register("memory", memPdr)
//}

func NewProvider() *ProviderMem {
	return &ProviderMem{list: list.New(), sessions: make(map[string]*list.Element)}
}
