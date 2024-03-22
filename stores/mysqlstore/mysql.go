// This file contains code from https://github.com/srinathgs/mysqlstore/blob/master/mysqlstore.go

package mysqlstore

import (
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	sessions "github.com/SebiWrn/gin-sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/gorilla/securecookie"
	"log"
	"strings"
	"time"
)

type MySQLStore struct {
	db         *sql.DB
	stmtInsert *sql.Stmt
	stmtDelete *sql.Stmt
	stmtUpdate *sql.Stmt
	stmtSelect *sql.Stmt

	Codecs  []securecookie.Codec
	Options *sessions.Options
	table   string
}

type sessionRow struct {
	id         string
	data       string
	createdOn  time.Time
	modifiedOn time.Time
	expiresOn  time.Time
}

func init() {
	gob.Register(time.Time{})
}

func NewMySQLStoreFromConnection(db *sql.DB, tableName string, path string, maxAge int, keyPairs ...[]byte) (*MySQLStore, error) {
	tableName = " " + strings.Trim(tableName, " ") + " "

	cTableQ := "CREATE TABLE IF NOT EXISTS " +
		tableName + " (id INT NOT NULL AUTO_INCREMENT, " +
		"session_data LONGBLOB, " +
		"created_on TIMESTAMP DEFAULT NOW(), " +
		"modified_on TIMESTAMP NOT NULL DEFAULT NOW() ON UPDATE CURRENT_TIMESTAMP, " +
		"expires_on TIMESTAMP DEFAULT NOW(), PRIMARY KEY(`id`)) ENGINE=InnoDB;"

	if _, err := db.Exec(cTableQ); err != nil {
		switch err.(type) {
		case *mysql.MySQLError:
			if err.(*mysql.MySQLError).Number == 1142 {
				break
			} else {
				return nil, err
			}
		default:
			return nil, err
		}
	}

	insQ := "INSERT INTO " + tableName +
		"(id, session_data, created_on, modified_on, expires_on) VALUES (NULL, ?, ?, ?, ?)"
	stmtInsert, stmtErr := db.Prepare(insQ)
	if stmtErr != nil {
		return nil, stmtErr
	}

	delQ := "DELETE FROM " + tableName + " WHERE id = ?"
	stmtDelete, stmtErr := db.Prepare(delQ)
	if stmtErr != nil {
		return nil, stmtErr
	}

	updQ := "UPDATE " + tableName + " SET session_data = ?, created_on = ?, expires_on = ? " +
		"WHERE id = ?"
	stmtUpdate, stmtErr := db.Prepare(updQ)
	if stmtErr != nil {
		return nil, stmtErr
	}

	selQ := "SELECT id, session_data, created_on, modified_on, expires_on from " +
		tableName + " WHERE id = ?"
	stmtSelect, stmtErr := db.Prepare(selQ)
	if stmtErr != nil {
		return nil, stmtErr
	}

	var codecs [1]securecookie.Codec
	cookieCodec := securecookie.New(securecookie.GenerateRandomKey(64), securecookie.GenerateRandomKey(32))
	cookieCodec.MaxLength(4096 * 4)
	codecs[0] = cookieCodec

	return &MySQLStore{
		db:         db,
		stmtInsert: stmtInsert,
		stmtDelete: stmtDelete,
		stmtUpdate: stmtUpdate,
		stmtSelect: stmtSelect,
		Codecs:     codecs[:],
		//Codecs:     securecookie.CodecsFromPairs(keyPairs...),
		Options: &sessions.Options{
			Path:   path,
			MaxAge: maxAge,
		},
		table: tableName,
	}, nil
}

func (m *MySQLStore) Close() {
	m.stmtSelect.Close()
	m.stmtUpdate.Close()
	m.stmtDelete.Close()
	m.stmtInsert.Close()
	m.db.Close()
}

func (m *MySQLStore) Get(c *gin.Context, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(c).Get(m, name)
}

func (m *MySQLStore) New(c *gin.Context, name string) (*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &sessions.Options{
		Path:     m.Options.Path,
		Domain:   m.Options.Domain,
		MaxAge:   m.Options.MaxAge,
		Secure:   m.Options.Secure,
		HttpOnly: m.Options.HttpOnly,
	}
	session.IsNew = true
	var err error
	if cook, errCookie := c.Cookie(name); errCookie == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, m.Codecs...)
		if err == nil {
			err = m.load(session)
			if err == nil {
				session.IsNew = false
			} else {
				err = nil
			}
		}
	}
	return session, err
}

func (m *MySQLStore) Save(c *gin.Context, session *sessions.Session) error {
	var err error
	if session.ID == "" {
		if err = m.insert(session); err != nil {
			return err
		}
	} else if err = m.save(session); err != nil {
		return err
	}
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, m.Codecs...)
	if err != nil {
		return err
	}
	sessions.SetCookieToContext(c, sessions.NewCookie(session.Name(), encoded, session.Options))
	return nil
}

func (m *MySQLStore) insert(session *sessions.Session) error {
	var createdOn time.Time
	var modifiedOn time.Time
	var expiresOn time.Time
	crOn := session.Values["created_on"]
	if crOn == nil {
		createdOn = time.Now()
	} else {
		createdOn = crOn.(time.Time)
	}
	modifiedOn = createdOn
	exOn := session.Values["expires_on"]
	if exOn == nil {
		expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
	} else {
		expiresOn = exOn.(time.Time)
	}
	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")

	encoded, encErr := securecookie.EncodeMulti(session.Name(), session.Values, m.Codecs...)
	if encErr != nil {
		return encErr
	}
	res, insErr := m.stmtInsert.Exec(encoded, createdOn, modifiedOn, expiresOn)
	if insErr != nil {
		return insErr
	}
	lastInserted, lInsErr := res.LastInsertId()
	if lInsErr != nil {
		return lInsErr
	}
	session.ID = fmt.Sprintf("%d", lastInserted)
	return nil
}

func (m *MySQLStore) Delete(c *gin.Context, session *sessions.Session) error {
	options := *session.Options
	options.MaxAge = -1
	sessions.SetCookieToContext(c, sessions.NewCookie(session.Name(), "", &options))
	for k := range session.Values {
		delete(session.Values, k)
	}

	_, delErr := m.stmtDelete.Exec(session.ID)
	if delErr != nil {
		return delErr
	}
	return nil
}

func (m *MySQLStore) save(session *sessions.Session) error {
	if session.IsNew == true {
		return m.insert(session)
	}
	var createdOn time.Time
	var expiresOn time.Time
	crOn := session.Values["created_on"]
	if crOn == nil {
		createdOn = time.Now()
	} else {
		createdOn = crOn.(time.Time)
	}

	exOn := session.Values["expires_on"]
	if exOn == nil {
		expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
		log.Print("nil")
	} else {
		expiresOn = exOn.(time.Time)
		if expiresOn.Sub(time.Now().Add(time.Second*time.Duration(session.Options.MaxAge))) < 0 {
			expiresOn = time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
		}
	}

	delete(session.Values, "created_on")
	delete(session.Values, "expires_on")
	delete(session.Values, "modified_on")

	encoded, encErr := securecookie.EncodeMulti(session.Name(), session.Values, m.Codecs...)
	if encErr != nil {
		return encErr
	}
	_, updErr := m.stmtUpdate.Exec(encoded, createdOn, expiresOn, session.ID)
	if updErr != nil {
		return updErr
	}
	return nil

}

func (m *MySQLStore) load(session *sessions.Session) error {
	row := m.stmtSelect.QueryRow(session.ID)
	sess := sessionRow{}
	scanErr := row.Scan(&sess.id, &sess.data, &sess.createdOn, &sess.modifiedOn, &sess.expiresOn)
	if scanErr != nil {
		return scanErr
	}
	if sess.expiresOn.Sub(time.Now()) < 0 {
		log.Printf("Session expired on %s, but it is %s now.", sess.expiresOn, time.Now())
		return errors.New("Session expired")
	}
	err := securecookie.DecodeMulti(session.Name(), sess.data, &session.Values, m.Codecs...)
	if err != nil {
		return err
	}
	session.Values["created_on"] = sess.createdOn
	session.Values["modified_on"] = sess.modifiedOn
	session.Values["expires_on"] = sess.expiresOn
	return nil

}
