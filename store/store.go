package store

// Store contains all data for one session process with specific id.
type Store interface {
	Set(key, value interface{}) error //set session value
	Get(key interface{}) interface{}  //get session value
	Delete(key interface{}) error     //delete session value
	SessionID() string                //back current sessionID
	SessionDelay()                    //session延期
	SessionRelease()                  //release the resource & save data to provider & return the data
	Flush() error                     //delete all data
}
