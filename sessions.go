// This file contains code from https://github.com/gorilla/sessions/blob/main/sessions.go

package sessions

import (
	"encoding/gob"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type Session struct {
	// ID should be generated by store
	ID string
	// User data of session
	Values  map[interface{}]interface{}
	Options *Options
	IsNew   bool
	store   Store
	name    string
}

func NewSession(store Store, name string) *Session {
	return &Session{
		Values:  make(map[interface{}]interface{}),
		Options: &Options{},
		store:   store,
		name:    name,
	}
}

func (s *Session) Save(c *gin.Context) error {
	return s.store.Save(c, s)
}

func (s *Session) Name() string {
	return s.name
}

func (s *Session) Store() Store {
	return s.store
}

type sessionInfo struct {
	s   *Session
	err error
}

const registryKey string = "registry"

type Registry struct {
	context  *gin.Context
	sessions map[string]sessionInfo
}

func GetRegistry(c *gin.Context) *Registry {
	registry := c.Value(registryKey)
	if registry != nil {
		return registry.(*Registry)
	}
	newRegistry := &Registry{
		context:  c,
		sessions: make(map[string]sessionInfo),
	}
	c.Set(registryKey, newRegistry)
	return newRegistry
}

func (s *Registry) Get(store Store, name string) (session *Session, err error) {
	if !isCookieNameValid(name) {
		return nil, fmt.Errorf("gin-sessions: invalid character in cookie name: %s", name)
	}
	if info, ok := s.sessions[name]; ok {
		session, err = info.s, info.err
	} else {
		session, err = store.New(s.context, name)
		session.name = name
		s.sessions[name] = sessionInfo{s: session, err: err}
	}
	session.store = store
	return
}

func (s *Registry) Save(c *gin.Context) error {
	var errMulti MultiError
	for name, info := range s.sessions {
		session := info.s
		if session.store == nil {
			errMulti = append(errMulti, fmt.Errorf(
				"gin-sessions: missing store for session %q", name))
		} else if err := session.store.Save(c, session); err != nil {
			errMulti = append(errMulti, fmt.Errorf(
				"gin-sessions: error saving session %q -- %v", name, err))
		}
	}
	if errMulti != nil {
		return errMulti
	}
	return nil
}

func init() {
	gob.Register([]interface{}{})
}

func Save(c *gin.Context) error {
	return GetRegistry(c).Save(c)
}

func NewCookie(name, value string, options *Options) *http.Cookie {
	cookie := newCookieFromOptions(name, value, options)
	if options.MaxAge > 0 {
		d := time.Duration(options.MaxAge) * time.Second
		cookie.Expires = time.Now().Add(d)
	} else if options.MaxAge < 0 {
		cookie.Expires = time.Unix(1, 0)
	}
	return cookie
}

// Error ----------------------------------------------------------------------

// MultiError stores multiple errors.
//
// Borrowed from the App Engine SDK.
type MultiError []error

func (m MultiError) Error() string {
	s, n := "", 0
	for _, e := range m {
		if e != nil {
			if n == 0 {
				s = e.Error()
			}
			n++
		}
	}
	switch n {
	case 0:
		return "(0 errors)"
	case 1:
		return s
	case 2:
		return s + " (and 1 other error)"
	}
	return fmt.Sprintf("%s (and %d other errors)", s, n-1)
}