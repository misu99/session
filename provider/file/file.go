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

package file

import (
	"errors"
	"github.com/misu99/session/store"
	"github.com/misu99/session/utils"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	filePdr    = &ProviderFile{}
	gcLifeTime int64
)

// SessionStoreFile File session store
type SessionStoreFile struct {
	sid    string
	lock   sync.RWMutex
	values map[interface{}]interface{}
}

// Set value to file session
func (st *SessionStoreFile) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values[key] = value
	return nil
}

// Get value from file session
func (st *SessionStoreFile) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.values[key]; ok {
		return v
	}
	return nil
}

// Delete value in file session by given key
func (st *SessionStoreFile) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.values, key)
	return nil
}

// Flush Clean all values in file session
func (st *SessionStoreFile) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values = make(map[interface{}]interface{})
	return nil
}

// SessionID Get file session store id
func (st *SessionStoreFile) SessionID() string {
	return st.sid
}

// SessionRelease Write file session to local file with Gob string
func (st *SessionStoreFile) SessionRelease() {
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()
	b, err := utils.EncodeGob(st.values)
	if err != nil {
		utils.SLogger.Println(err)
		return
	}
	_, err = os.Stat(path.Join(filePdr.savePath, string(st.sid[0]), string(st.sid[1]), st.sid))
	var f *os.File
	if err == nil {
		f, err = os.OpenFile(path.Join(filePdr.savePath, string(st.sid[0]), string(st.sid[1]), st.sid), os.O_RDWR, 0777)
		if err != nil {
			utils.SLogger.Println(err)
			return
		}
	} else if os.IsNotExist(err) {
		f, err = os.Create(path.Join(filePdr.savePath, string(st.sid[0]), string(st.sid[1]), st.sid))
		if err != nil {
			utils.SLogger.Println(err)
			return
		}
	} else {
		return
	}
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = f.Write(b)
	_ = f.Close()
}

// ProviderFile File session provider
type ProviderFile struct {
	lock     sync.RWMutex
	lifeTime int64
	savePath string
}

// SessionInit Init file session provider.
// savePath sets the session files path.
func (pdr *ProviderFile) SessionInit(lifetime int64, savePath string) error {
	pdr.lifeTime = lifetime
	pdr.savePath = savePath
	return nil
}

// create new file session by sid.
// if file is not exist, create it.
// the file path is generated from sid string.
func (pdr *ProviderFile) SessionNew(sid string) (store.Store, error) {
	if strings.ContainsAny(sid, "./") {
		return nil, nil
	}
	if len(sid) < 2 {
		return nil, errors.New("length of the sid is less than 2")
	}
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()

	err := os.MkdirAll(path.Join(pdr.savePath, string(sid[0]), string(sid[1])), 0777)
	if err != nil {
		utils.SLogger.Println(err.Error())
	}
	_, err = os.Stat(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	var f *os.File
	if err == nil {
		f, err = os.OpenFile(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid), os.O_RDWR, 0777)
	} else if os.IsNotExist(err) {
		f, err = os.Create(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	} else {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_ = os.Chtimes(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid), time.Now(), time.Now())
	var kv map[interface{}]interface{}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = utils.DecodeGob(b)
		if err != nil {
			return nil, err
		}
	}

	ss := &SessionStoreFile{sid: sid, values: kv}
	return ss, nil
}

// SessionRead Read file session by sid.
// the file path is generated from sid string.
func (pdr *ProviderFile) SessionRead(sid string) (store.Store, error) {
	if strings.ContainsAny(sid, "./") {
		return nil, nil
	}
	if len(sid) < 2 {
		return nil, errors.New("length of the sid is less than 2")
	}
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()

	err := os.MkdirAll(path.Join(pdr.savePath, string(sid[0]), string(sid[1])), 0777)
	if err != nil {
		utils.SLogger.Println(err.Error())
	}
	_, err = os.Stat(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	var f *os.File
	if err == nil {
		f, err = os.OpenFile(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid), os.O_RDWR, 0777)
		//} else if os.IsNotExist(err) {
		//	f, err = os.Create(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	} else {
		return nil, err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_ = os.Chtimes(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid), time.Now(), time.Now())
	var kv map[interface{}]interface{}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = utils.DecodeGob(b)
		if err != nil {
			return nil, err
		}
	}

	ss := &SessionStoreFile{sid: sid, values: kv}
	return ss, nil
}

// SessionExist Check file session exist.
// it checks the file named from sid exist or not.
func (pdr *ProviderFile) SessionExist(sid string) bool {
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()

	_, err := os.Stat(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	return err == nil
}

// SessionDestroy Remove all files in this save path
func (pdr *ProviderFile) SessionDestroy(sid string) error {
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()
	_ = os.Remove(path.Join(pdr.savePath, string(sid[0]), string(sid[1]), sid))
	return nil
}

// SessionGC Recycle files in save path
func (pdr *ProviderFile) SessionGC() {
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()

	gcLifeTime = pdr.lifeTime
	_ = filepath.Walk(pdr.savePath, gcDir)
}

// SessionAll id values in mysql session
func (pdr *ProviderFile) SessionAll() ([]string, error) {
	a := &activeSession{}
	err := filepath.Walk(pdr.savePath, func(path string, f os.FileInfo, err error) error {
		return a.visit(path, f, err)
	})
	if err != nil {
		//utils.SLogger.Printf("filepath.Walk() returned %v\n", err)
		return nil, err
	}
	return a.sids, nil
}

// SessionRegenerate Generate new sid for file session.
// it delete old file and create new file named from new sid.
func (pdr *ProviderFile) SessionRegenerate(oldSid, sid string) (store.Store, error) {
	filePdr.lock.Lock()
	defer filePdr.lock.Unlock()

	oldPath := path.Join(pdr.savePath, string(oldSid[0]), string(oldSid[1]))
	oldSidFile := path.Join(oldPath, oldSid)
	newPath := path.Join(pdr.savePath, string(sid[0]), string(sid[1]))
	newSidFile := path.Join(newPath, sid)

	// new sid file is exist
	//_, err := os.Stat(newSidFile)
	//if err == nil {
	//	return nil, fmt.Errorf("newsid %s exist", newSidFile)
	//}

	err := os.MkdirAll(newPath, 0777)
	if err != nil {
		utils.SLogger.Println(err.Error())
	}

	// if old sid file exist
	// 1.read and parse file content
	// 2.write content to new sid file
	// 3.remove old sid file, change new sid file atime and ctime
	// 4.return SessionStoreFile
	_, err = os.Stat(oldSidFile)
	if err == nil {
		b, err := ioutil.ReadFile(oldSidFile)
		if err != nil {
			return nil, err
		}

		var kv map[interface{}]interface{}
		if len(b) == 0 {
			kv = make(map[interface{}]interface{})
		} else {
			kv, err = utils.DecodeGob(b)
			if err != nil {
				return nil, err
			}
		}

		if oldSid != sid {
			_ = ioutil.WriteFile(newSidFile, b, 0777)
			_ = os.Remove(oldSidFile)
		}

		_ = os.Chtimes(newSidFile, time.Now(), time.Now())
		ss := &SessionStoreFile{sid: sid, values: kv}
		return ss, nil
	}

	// if old sid file not exist, just create new sid file and return
	newf, err := os.Create(newSidFile)
	if err != nil {
		return nil, err
	}
	_ = newf.Close()
	ss := &SessionStoreFile{sid: sid, values: make(map[interface{}]interface{})}
	return ss, nil
}

// remove file in save path if expired
func gcDir(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if (info.ModTime().Unix() + gcLifeTime) < time.Now().Unix() {
		_ = os.Remove(path)
	}
	return nil
}

type activeSession struct {
	total int
	sids  []string
}

func (as *activeSession) visit(paths string, f os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if f.IsDir() {
		return nil
	}
	as.total = as.total + 1
	as.sids = append(as.sids, f.Name())
	return nil
}

//func init() {
//	session.Register("file", filePdr)
//}

func NewProvider() *ProviderFile {
	return filePdr
}
