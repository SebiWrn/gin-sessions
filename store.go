// This file contains code from https://github.com/gorilla/sessions/blob/main/store.go

package sessions

import (
	"github.com/gin-gonic/gin"
)

type Store interface {
	// To return a cached session
	Get(c *gin.Context, name string) (*Session, error)

	// To create a new session
	New(c *gin.Context, name string) (*Session, error)

	// To save a changed session
	Save(c *gin.Context, s *Session) error

	// To check if a session exists
	//Exists(c *gin.Context, name string) bool

	// To delete a session
	Delete(c *gin.Context, s *Session) error
}
