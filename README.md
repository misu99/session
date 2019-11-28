session
==============

session is a Go session manager. It can use many session providers. Just like the `database/sql` and `database/sql/driver`.  
该版本从beego/session改动而来，基于原版根据实际使用做部分改动，[原版地址](https://github.com/astaxie/beego/tree/master/session)

## How to install?

	go get github.com/misu99/session


## What providers are supported?

As of now this session manager support memory, file, Redis and MySQL.


## How to use it?

First you must import it

	import (
		"github.com/misu99/session"
	)

Then in you web app init the global session manager
	
	var globalSessions *session.Manager

* Use **memory** as provider:

		import _ "github.com/misu99/session/memory"
		
		func init() {
			globalSessions, _ = session.NewManager("memory", `{"cookieName":"gosessionid","gclifetime":3600}`)
			go globalSessions.GC()
		}

* Use **file** as provider, the last param is the path where you want file to be stored:

		import _ "github.com/misu99/session/file"
		
		func init() {
			globalSessions, _ = session.NewManager("file",`{"cookieName":"gosessionid","gclifetime":3600,"ProviderConfig":"./tmp"}`)
			go globalSessions.GC()
		}

* Use **Redis** as provider, the last param is the Redis conn address,poolsize,password:

		import _ "github.com/misu99/session/redis"
		
		func init() {
			globalSessions, _ = session.NewManager("redis", `{"cookieName":"gosessionid","gclifetime":3600,"ProviderConfig":"127.0.0.1:6379,100,astaxie"}`)
			go globalSessions.GC()
		}
		
* Use **MySQL** as provider, the last param is the DSN, learn more from [mysql](https://github.com/go-sql-driver/mysql#dsn-data-source-name):

		import _ "github.com/misu99/session/mysql"
		
		func init() {
			globalSessions, _ = session.NewManager(
				"mysql", `{"cookieName":"gosessionid","gclifetime":3600,"ProviderConfig":"username:password@protocol(address)/dbname?param=value"}`)
			go globalSessions.GC()
		}

* Use **Cookie** as provider:

		import _ "github.com/misu99/session/cookie"
		
		func init() {
			globalSessions, _ = session.NewManager(
				"cookie", `{"cookieName":"gosessionid","enableSetCookie":false,"gclifetime":3600,"ProviderConfig":"{\"cookieName\":\"gosessionid\",\"securityKey\":\"beegocookiehashkey\"}"}`)
			go globalSessions.GC()
		}


Finally in the handlerfunc you can use it like this
* session(cookie)
    ```
    func login(w http.ResponseWriter, r *http.Request) {
        sess := globalSessions.SessionStart(w, r)
        defer sess.SessionRelease(w)
        username := sess.Get("username")
        fmt.Println(username)
        if r.Method == "GET" {
            t, _ := template.ParseFiles("login.gtpl")
            t.Execute(w, nil)
        } else {
            fmt.Println("username:", r.Form["username"])
            sess.Set("username", r.Form["username"])
            fmt.Println("password:", r.Form["password"])
        }
    }
    ```
* token
    ```
    func login(w http.ResponseWriter, r *http.Request) {
    	sess := globalSessions.TokenStart()
    	defer sess.SessionRelease()
    	username := sess.Get("username")
    	fmt.Println(username)
    	if r.Method == "GET" {
    		t, _ := template.ParseFiles("login.gtpl")
    		t.Execute(w, nil)
    	} else {
    		fmt.Println("username:", r.Form["username"])
    		sess.Set("username", r.Form["username"])
    		fmt.Println("password:", r.Form["password"])
    	}
    }
    ```

## How to write own provider?

When you develop a web app, maybe you want to write own provider because you must meet the requirements.

Writing a provider is easy. You only need to define two struct types 
(Session and Provider), which satisfy the interface definition. 
Maybe you will find the **memory** provider is a good example.

	type SessionStore interface {
		Set(key, value interface{}) error     //set session value
		Get(key interface{}) interface{}      //get session value
		Delete(key interface{}) error         //delete session value
		SessionID() string                    //back current sessionID
		SessionRelease()                      // release the resource & save data to provider & return the data
		Flush() error                         //delete all data
	}
	
	type Provider interface {
		SessionInit(gclifetime int64, config string) error
		SessionNew(sid string) (Store, error)
		SessionRead(sid string) (SessionStore, error)
		SessionExist(sid string) bool
		SessionRegenerate(oldsid, sid string) (SessionStore, error)
		SessionDestroy(sid string) error
		SessionAll() int //get all active session
		SessionGC()
	}

## Improve
- 增加token形式的session管理（独立存在，不依赖cookie）。  
详见 ```TokenStart()``` 与 ```SessionStart(w http.ResponseWriter, r *http.Request)``` 两个函数之间的差异部分

- 适配器增加接口 ```SessionNew(sid string) (Store, error)``` 接口，将新建session的动作从 ```SessionRead(sid string) (Store, error)``` 中分离。  
通过 ```sess, err := session.GetSessionStore(token)``` 方法的err即可判断是否存在对应的session

- 增加token续期函数 ```TokenRegenerateID、TokenExtension```  
```TokenRegenerateID```与```SessionRegenerateID```作用相同，销毁旧的session产生一个新的session。  
```TokenRegenerateID``` 的作用是将旧的session进行续期，session id保持不变，续期时长是参数配置中的存活时长。

- 改写SessionAll接口为获取所有session id ```SessionAll() ([]string, error)``` ，并在已有适配器中实现该方法。

- 改写持久化接口 ```SessionRelease()``` ，移除未曾使用的参数。

- 适配器修改：
  - **mysql**  
  自动创建session表  
  