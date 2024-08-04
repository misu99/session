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

package redis

import (
	"github.com/misu99/session/store"
	"github.com/misu99/session/utils"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

const MaxPoolSize = 100

//var redisPdr = &ProviderRedis{}

// SessionStoreRedis redis session store
type SessionStoreRedis struct {
	pl       *redis.Pool
	sid      string
	lock     sync.RWMutex
	values   map[interface{}]interface{}
	lifetime int64
}

// Set value in redis session
func (st *SessionStoreRedis) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values[key] = value
	return nil
}

// Get value in redis session
func (st *SessionStoreRedis) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.values[key]; ok {
		return v
	}
	return nil
}

// Delete value in redis session
func (st *SessionStoreRedis) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.values, key)
	return nil
}

// Flush clear all values in redis session
func (st *SessionStoreRedis) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values = make(map[interface{}]interface{})
	return nil
}

// SessionID get redis session id
func (st *SessionStoreRedis) SessionID() string {
	return st.sid
}

// SessionRelease save session values to redis
func (st *SessionStoreRedis) SessionRelease() {
	b, err := utils.EncodeGob(st.values)
	if err != nil {
		return
	}
	c := st.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_, err = c.Do("SETEX", st.sid, st.lifetime, string(b))
	if err != nil {
		utils.SLogger.Println(err)
	}
}

// ProviderRedis redis session provider
type ProviderRedis struct {
	lifetime int64
	savePath string
	poolSize int
	password string
	dbIndex  int
	pl       *redis.Pool
}

// SessionInit init redis session
// savepath like redis server addr,pool size,password,dbnum,IdleTimeout second
// e.g. 127.0.0.1:6379,100,astaxie,0,30
func (pdr *ProviderRedis) SessionInit(lifetime int64, savePath string) error {
	pdr.lifetime = lifetime
	configs := strings.Split(savePath, ",")
	if len(configs) > 0 {
		pdr.savePath = configs[0]
	}
	if len(configs) > 1 {
		poolSize, err := strconv.Atoi(configs[1])
		if err != nil || poolSize < 0 {
			pdr.poolSize = MaxPoolSize
		} else {
			pdr.poolSize = poolSize
		}
	} else {
		pdr.poolSize = MaxPoolSize
	}
	if len(configs) > 2 {
		pdr.password = configs[2]
	}
	if len(configs) > 3 {
		index, err := strconv.Atoi(configs[3])
		if err != nil || index < 0 {
			pdr.dbIndex = 0
		} else {
			pdr.dbIndex = index
		}
	} else {
		pdr.dbIndex = 0
	}
	var idleTimeout time.Duration = 0
	if len(configs) > 4 {
		timeout, err := strconv.Atoi(configs[4])
		if err == nil && timeout > 0 {
			idleTimeout = time.Duration(timeout) * time.Second
		}
	}
	pdr.pl = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", pdr.savePath)
			if err != nil {
				return nil, err
			}
			if pdr.password != "" {
				if _, err = c.Do("AUTH", pdr.password); err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			// some redis proxy such as twemproxy is not support select command
			if pdr.dbIndex > 0 {
				_, err = c.Do("SELECT", pdr.dbIndex)
				if err != nil {
					_ = c.Close()
					return nil, err
				}
			}
			return c, err
		},
		MaxIdle: pdr.poolSize,
	}

	pdr.pl.IdleTimeout = idleTimeout

	return pdr.pl.Get().Err()
}

// create new redis session by sid
func (pdr *ProviderRedis) SessionNew(sid string, lifetime int64) (store.Store, error) {
	if lifetime != 0 {
		pdr.lifetime = lifetime
	}

	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	var kv map[interface{}]interface{}

	kvs, err := redis.String(c.Do("GET", sid))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}
	if len(kvs) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		if kv, err = utils.DecodeGob([]byte(kvs)); err != nil {
			return nil, err
		}
	}

	st := &SessionStoreRedis{pl: pdr.pl, sid: sid, values: kv, lifetime: pdr.lifetime}
	return st, nil
}

// read redis session by sid
func (pdr *ProviderRedis) SessionRead(sid string) (store.Store, error) {
	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	var kv map[interface{}]interface{}

	kvs, err := redis.String(c.Do("GET", sid))
	//if err != nil && err != redis.ErrNil {
	if err != nil {
		return nil, err
	}
	if len(kvs) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		if kv, err = utils.DecodeGob([]byte(kvs)); err != nil {
			return nil, err
		}
	}

	st := &SessionStoreRedis{pl: pdr.pl, sid: sid, values: kv, lifetime: pdr.lifetime}
	return st, nil
}

// SessionExist check redis session exist by sid
func (pdr *ProviderRedis) SessionExist(sid string) bool {
	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	if existed, err := redis.Int(c.Do("EXISTS", sid)); err != nil || existed == 0 {
		return false
	}
	return true
}

// SessionRegenerate generate new sid for redis session
func (pdr *ProviderRedis) SessionRegenerate(oldSid, sid string, lifetime int64) (store.Store, error) {
	if lifetime != 0 {
		pdr.lifetime = lifetime
	}

	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	if existed, _ := redis.Int(c.Do("EXISTS", oldSid)); existed == 0 {
		// oldSid doesn't exists, set the new sid directly
		// ignore error here, since if it return error
		// the existed value will be 0
		_, err := c.Do("SET", sid, "", "EX", pdr.lifetime)
		if err != nil {
			utils.SLogger.Println(err)
		}
	} else {
		_, err := c.Do("RENAME", oldSid, sid)
		if err != nil {
			utils.SLogger.Println(err)
		}
		_, err = c.Do("EXPIRE", sid, pdr.lifetime)
		if err != nil {
			utils.SLogger.Println(err)
		}
	}
	return pdr.SessionRead(sid)
}

// SessionDestroy delete redis session by id
func (pdr *ProviderRedis) SessionDestroy(sid string) error {
	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_, err := c.Do("DEL", sid)
	return err
}

// SessionGC impl method, no used.
func (pdr *ProviderRedis) SessionGC() {
}

// SessionAll id values in mysql session
func (pdr *ProviderRedis) SessionAll() ([]string, error) {
	c := pdr.pl.Get()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	values, err := redis.Strings(c.Do("KEYS", "*"))
	if err != nil {
		return nil, err
	}

	return values, nil
}

//func init() {
//	session.Register("redis", redisPdr)
//}

func NewProvider() *ProviderRedis {
	return &ProviderRedis{}
}
