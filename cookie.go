// This file contains code from https://github.com/gorilla/sessions/blob/main/cookie.go

package sessions

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// newCookieFromOptions returns an http.Cookie with the options set.
func newCookieFromOptions(name, value string, options *Options) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}

// Set httpCookie to gin.Context
func SetCookieToContext(c *gin.Context, cookie *http.Cookie) {
	c.SetCookie(cookie.Name, cookie.Value, cookie.MaxAge, cookie.Path, cookie.Domain, cookie.Secure, cookie.HttpOnly)
}
