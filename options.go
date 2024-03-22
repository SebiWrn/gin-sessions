// This file contains code from https://github.com/gorilla/sessions/blob/main/options.go

package sessions

type Options struct {
	Path     string
	Domain   string
	MaxAge   int
	Secure   bool
	HttpOnly bool
}
