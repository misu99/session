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

package mysql

import (
	"database/sql"
	"github.com/misu99/session/store"
	"github.com/misu99/session/utils"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	TableName = "session"
	sqlInit   = `
		CREATE TABLE ` + TableName + ` (
		session_key char(64) NOT NULL,
		session_data blob,
		session_expiry int(11) unsigned NOT NULL,
		PRIMARY KEY (session_key)
		) ENGINE=MyISAM DEFAULT CHARSET=utf8;
	`
)

//var mysqlPdr = &ProviderMySQL{}

// SessionStoreMySQL mysql session store
type SessionStoreMySQL struct {
	conn   *sql.DB
	sid    string
	lock   sync.RWMutex
	values map[interface{}]interface{}
}

// Set value in mysql session.
// it is temp value in map.
func (st *SessionStoreMySQL) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values[key] = value
	return nil
}

// Get value from mysql session
func (st *SessionStoreMySQL) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.values[key]; ok {
		return v
	}
	return nil
}

// Delete value in mysql session
func (st *SessionStoreMySQL) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.values, key)
	return nil
}

// Flush clear all values in mysql session
func (st *SessionStoreMySQL) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values = make(map[interface{}]interface{})
	return nil
}

// SessionID get session id of this mysql session store
func (st *SessionStoreMySQL) SessionID() string {
	return st.sid
}

// SessionDelay Implement method, no used.
func (st *SessionStoreMySQL) SessionDelay() {
}

// SessionRelease save mysql session values to database.
// must call this method to save values to database.
func (st *SessionStoreMySQL) SessionRelease() {
	defer func() {
		err := st.conn.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	b, err := utils.EncodeGob(st.values)
	if err != nil {
		utils.SLogger.Println(err)
		return
	}
	_, err = st.conn.Exec("UPDATE "+TableName+" set `session_data`=?, `session_expiry`=? where session_key=?",
		b, time.Now().Unix(), st.sid)
	if err != nil {
		utils.SLogger.Println(err)
		return
	}
}

// ProviderMySQL mysql session provider
type ProviderMySQL struct {
	lifetime int64
	savePath string
}

// connect to mysql
func (pdr *ProviderMySQL) connectInit() *sql.DB {
	db, e := sql.Open("mysql", pdr.savePath)
	if e != nil {
		return nil
	}
	return db
}

// SessionInit init mysql session.
func (pdr *ProviderMySQL) SessionInit(lifetime int64, savePath string) error {
	pdr.lifetime = lifetime
	pdr.savePath = savePath

	c := pdr.connectInit()
	_, err := c.Exec(sqlInit)
	if err == nil || strings.ContainsAny(err.Error(), "already exists") {
		return nil
	}

	return err
}

// create new mysql session by sid
func (pdr *ProviderMySQL) SessionNew(sid string, lifetime int64) (store.Store, error) {
	c := pdr.connectInit()
	row := c.QueryRow("select session_data from "+TableName+" where session_key=?", sid)
	var data []byte
	err := row.Scan(&data)
	if err == sql.ErrNoRows {
		_, err = c.Exec("insert into "+TableName+"(`session_key`,`session_data`,`session_expiry`) values(?,?,?)",
			sid, "", time.Now().Unix())
		if err != nil {
			return nil, err
		}
	}

	var kv map[interface{}]interface{}
	if len(data) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = utils.DecodeGob(data)
		if err != nil {
			return nil, err
		}
	}
	rs := &SessionStoreMySQL{conn: c, sid: sid, values: kv}
	return rs, nil
}

// SessionRead get mysql session by sid
func (pdr *ProviderMySQL) SessionRead(sid string) (store.Store, error) {
	c := pdr.connectInit()
	row := c.QueryRow("select session_data from "+TableName+" where session_key=?", sid)
	var data []byte
	err := row.Scan(&data)
	//if err == sql.ErrNoRows {
	//	conn.Exec("insert into "+TableName+"(`session_key`,`session_data`,`session_expiry`) values(?,?,?)",
	//		sid, "", time.Now().Unix())
	//}
	if err != nil {
		return nil, err
	}

	var kv map[interface{}]interface{}
	if len(data) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = utils.DecodeGob(data)
		if err != nil {
			return nil, err
		}
	}
	rs := &SessionStoreMySQL{conn: c, sid: sid, values: kv}
	return rs, nil
}

// SessionExist check mysql session exist
func (pdr *ProviderMySQL) SessionExist(sid string) bool {
	c := pdr.connectInit()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	row := c.QueryRow("select session_data from "+TableName+" where session_key=?", sid)
	var data []byte
	err := row.Scan(&data)
	return err != sql.ErrNoRows
}

// SessionRegenerate generate new sid for mysql session
func (pdr *ProviderMySQL) SessionRegenerate(oldSid, sid string) (store.Store, error) {
	c := pdr.connectInit()
	row := c.QueryRow("select session_data from "+TableName+" where session_key=?", oldSid)
	var data []byte
	err := row.Scan(&data)
	if err == sql.ErrNoRows {
		_, err = c.Exec("insert into "+TableName+"(`session_key`,`session_data`,`session_expiry`) values(?,?,?)", oldSid, "", time.Now().Unix())
		if err != nil {
			return nil, err
		}
	}

	_, err = c.Exec("update "+TableName+" set `session_key`=?, `session_expiry`=? where session_key=?", sid, time.Now().Unix(), oldSid)
	if err != nil {
		return nil, err
	}

	var kv map[interface{}]interface{}
	if len(data) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		kv, err = utils.DecodeGob(data)
		if err != nil {
			return nil, err
		}
	}
	rs := &SessionStoreMySQL{conn: c, sid: sid, values: kv}
	return rs, nil
}

// SessionDestroy delete mysql session by sid
func (pdr *ProviderMySQL) SessionDestroy(sid string) error {
	c := pdr.connectInit()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_, err := c.Exec("DELETE FROM "+TableName+" where session_key=?", sid)
	if err != nil {
		return err
	}

	return nil
}

// SessionGC delete expired values in mysql session
func (pdr *ProviderMySQL) SessionGC() {
	c := pdr.connectInit()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	_, err := c.Exec("DELETE from "+TableName+" where session_expiry < ?", time.Now().Unix()-pdr.lifetime)
	if err != nil {
		utils.SLogger.Println(err)
	}
}

// SessionAll id values in mysql session
func (pdr *ProviderMySQL) SessionAll() ([]string, error) {
	c := pdr.connectInit()
	defer func() {
		err := c.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	rows, err := c.Query("select session_key from " + TableName)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			utils.SLogger.Println(err)
		}
	}()

	var sids []string
	for rows.Next() {
		var sid string
		err = rows.Scan(&sid)
		if err != nil {
			return nil, err
		}
		sids = append(sids, sid)
	}

	return sids, nil
}

//func init() {
//	session.Register("mysql", mysqlPdr)
//}

func NewProvider() *ProviderMySQL {
	return &ProviderMySQL{}
}
