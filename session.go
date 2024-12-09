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

package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/misu99/session/provider/file"
	"github.com/misu99/session/provider/memory"
	"github.com/misu99/session/provider/mysql"
	"github.com/misu99/session/provider/redis"
	"github.com/misu99/session/store"
	"net/http"
	"net/textproto"
	"net/url"
	"time"
)

//var (
// provides = make(map[string]Provider)
//)

// Provider contains global session methods and saved SessionStores.
// it can operate a SessionStore by its id.
type Provider interface {
	SessionInit(lifetime int64, config string) error
	SessionNew(sid string, lifetime int64) (store.Store, error)
	SessionRead(sid string) (store.Store, error)
	SessionExist(sid string) bool
	SessionRegenerate(oldsid, sid string) (store.Store, error)
	SessionDestroy(sid string) error
	SessionAll() ([]string, error) //get all active session
	SessionGC()
}

// Register makes a session provide available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.
//func Register(name string, provide Provider) {
//	if provide == nil {
//		panic("session: Register provide is nil")
//	}
//	if _, dup := provides[name]; dup {
//		panic("session: Register called twice for provider " + name)
//	}
//	provides[name] = provide
//}

// GetProvider
func GetProvider(name string) (Provider, error) {
	//provider, ok := provides[name]
	//if !ok {
	//	return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", name)
	//}
	//return provider, nil

	switch name {
	case "file":
		return file.NewProvider(), nil
	case "memory":
		return memory.NewProvider(), nil
	case "mysql":
		return mysql.NewProvider(), nil
	case "redis":
		return redis.NewProvider(), nil
	default:
		return nil, fmt.Errorf("session: unknown provider %s", name)
	}

}

// ManagerConfig define the session config
type ManagerConfig struct {
	CookieName              string `json:"cookieName"`
	EnableSetCookie         bool   `json:"enableSetCookie,omitempty"`
	Gclifetime              int64  `json:"gclifetime"`
	Maxlifetime             int64  `json:"maxLifetime"`
	DisableHTTPOnly         bool   `json:"disableHTTPOnly"`
	Secure                  bool   `json:"secure"`
	CookieLifeTime          int    `json:"cookieLifeTime"`
	ProviderConfig          string `json:"providerConfig"`
	ProviderConfigMgr       string `json:"providerConfigMgr,omitempty"`
	Domain                  string `json:"domain"`
	SessionIDLength         int64  `json:"sessionIDLength"`
	EnableSidInHTTPHeader   bool   `json:"EnableSidInHTTPHeader"`
	SessionNameInHTTPHeader string `json:"SessionNameInHTTPHeader"`
	EnableSidInURLQuery     bool   `json:"EnableSidInURLQuery"`
	SessionIDPrefix         string `json:"sessionIDPrefix"`
}

// Manager contains Provider and its configuration.
type Manager struct {
	provider    Provider
	providerMgr Provider
	config      *ManagerConfig
}

// NewManager Create new Manager with provider name and json config string.
// provider name:
// 1. cookie
// 2. file
// 3. memory
// 4. redis
// 5. mysql
// json config:
// 1. is https  default false
// 2. hashfunc  default sha1
// 3. hashkey default beegosessionkey
// 4. maxage default is none
func NewManager(provideName string, cf *ManagerConfig) (*Manager, error) {
	if cf.Maxlifetime == 0 {
		cf.Maxlifetime = cf.Gclifetime
	}
	if cf.EnableSidInHTTPHeader {
		if cf.SessionNameInHTTPHeader == "" {
			panic(errors.New("SessionNameInHTTPHeader is empty"))
		}

		strMimeHeader := textproto.CanonicalMIMEHeaderKey(cf.SessionNameInHTTPHeader)
		if cf.SessionNameInHTTPHeader != strMimeHeader {
			strErrMsg := "SessionNameInHTTPHeader (" + cf.SessionNameInHTTPHeader + ") has the wrong format, it should be like this : " + strMimeHeader
			panic(errors.New(strErrMsg))
		}
	}
	if cf.SessionIDLength == 0 {
		cf.SessionIDLength = 16
	}

	provider, err := GetProvider(provideName)
	if err != nil {
		return nil, err
	}

	err = provider.SessionInit(cf.Maxlifetime, cf.ProviderConfig)
	if err != nil {
		return nil, err
	}

	var providerMgr Provider
	if cf.ProviderConfigMgr != "" {
		providerMgr, err = GetProvider(provideName)
		if err != nil {
			return nil, err
		}

		err = providerMgr.SessionInit(cf.Maxlifetime, cf.ProviderConfigMgr)
		if err != nil {
			return nil, err
		}
	}

	return &Manager{
		provider:    provider,
		providerMgr: providerMgr,
		config:      cf,
	}, nil
}

// GetProvider return current manager's provider
func (manager *Manager) GetProvider() Provider {
	return manager.provider
}

// getSid retrieves session identifier from HTTP Request.
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters.
//
// error is not nil when there is anything wrong.
// sid is empty when need to generate a new session id
// otherwise return an valid session id.
func (manager *Manager) getSid(r *http.Request) (string, error) {
	cookie, errs := r.Cookie(manager.config.CookieName)
	if errs != nil || cookie.Value == "" {
		var sid string
		if manager.config.EnableSidInURLQuery {
			errs := r.ParseForm()
			if errs != nil {
				return "", errs
			}

			sid = r.FormValue(manager.config.CookieName)
		}

		// if not found in Cookie / param, then read it from request headers
		if manager.config.EnableSidInHTTPHeader && sid == "" {
			sids, isFound := r.Header[manager.config.SessionNameInHTTPHeader]
			if isFound && len(sids) != 0 {
				return sids[0], nil
			}
		}

		return sid, nil
	}

	// HTTP Request contains cookie for sessionid info.
	return url.QueryUnescape(cookie.Value)
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session store.Store, err error) {
	sid, errs := manager.getSid(r)
	if errs != nil {
		return nil, errs
	}

	if sid != "" && manager.provider.SessionExist(sid) {
		return manager.provider.SessionRead(sid)
	}

	// Generate a new session
	sid, errs = manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	session, err = manager.provider.SessionNew(sid, 0)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     manager.config.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: !manager.config.DisableHTTPOnly,
		Secure:   manager.isSecure(r),
		Domain:   manager.config.Domain,
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// SessionDestroy Destroy session by its id in http request cookie.
func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	if manager.config.EnableSidInHTTPHeader {
		r.Header.Del(manager.config.SessionNameInHTTPHeader)
		w.Header().Del(manager.config.SessionNameInHTTPHeader)
	}

	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	_ = manager.provider.SessionDestroy(sid)
	if manager.config.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Expires:  expiration,
			MaxAge:   -1}

		http.SetCookie(w, cookie)
	}
}

// 生成token
func (manager *Manager) TokenStart() (session store.Store, err error) {
	// Generate a new session
	sid, errs := manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	session, err = manager.provider.SessionNew(sid, 0)
	if err != nil {
		return nil, err
	}

	return
}

// 生成token(自定义时效)
func (manager *Manager) TokenStartExpired(ttl time.Duration) (session store.Store, err error) {
	// Generate a new session
	sid, errs := manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	session, err = manager.provider.SessionNew(sid, int64(ttl.Seconds()))
	if err != nil {
		return nil, err
	}

	return
}

// 销毁token
func (manager *Manager) TokenDestroy(sid string) error {
	return manager.provider.SessionDestroy(sid)
}

// GetSessionStore Get SessionStore by its id.
func (manager *Manager) GetSessionStore(sid string) (sessions store.Store, err error) {
	sessions, err = manager.provider.SessionRead(sid)
	return
}

// 生成token与用户映射
func (manager *Manager) TokenMgrCreate(userId, token string) (session store.Store, err error) {
	session, err = manager.providerMgr.SessionNew(userId, 0)
	if err != nil {
		return nil, err
	}

	err = session.Set("token", token)
	if err != nil {
		return nil, err
	}

	session.SessionRelease()
	return
}
func (manager *Manager) MgrDestroyToken(userId string) (err error) {
	session, err := manager.providerMgr.SessionRead(userId)
	if err != nil {
		return
	}

	val := session.Get("token")
	if val != nil {
		manager.provider.SessionDestroy(val.(string)) // 销毁token
		manager.providerMgr.SessionDestroy(userId)    // 销毁token与用户映射
	}

	return
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (manager *Manager) GC() {
	manager.provider.SessionGC()
	time.AfterFunc(time.Duration(manager.config.Gclifetime)*time.Second, func() { manager.GC() })
}

// SessionRegenerateID Regenerate a session id for this SessionStore who's id is saving in http request.
func (manager *Manager) SessionRegenerateID(w http.ResponseWriter, r *http.Request) (session store.Store) {
	sid, err := manager.sessionID()
	if err != nil {
		return
	}
	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		//delete old cookie
		session, _ = manager.provider.SessionNew(sid, 0)
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Secure:   manager.isSecure(r),
			Domain:   manager.config.Domain,
		}
	} else {
		oldsid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRegenerate(oldsid, sid)
		cookie.Value = url.QueryEscape(sid)
		cookie.HttpOnly = true
		cookie.Path = "/"
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// GetActiveSession Get all active sessions id.
func (manager *Manager) GetActiveSession() ([]string, error) {
	return manager.provider.SessionAll()
}

// SetSecure Set cookie with https.
func (manager *Manager) SetSecure(secure bool) {
	manager.config.Secure = secure
}

// Generate a session id
func (manager *Manager) sessionID() (string, error) {
	b := make([]byte, manager.config.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", errors.New("could not successfully read from the system CSPRNG")
	}
	return manager.config.SessionIDPrefix + hex.EncodeToString(b), nil
}

// Set cookie with https.
func (manager *Manager) isSecure(req *http.Request) bool {
	if !manager.config.Secure {
		return false
	}
	if req.URL.Scheme != "" {
		return req.URL.Scheme == "https"
	}
	if req.TLS == nil {
		return false
	}
	return true
}
