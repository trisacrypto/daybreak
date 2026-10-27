package db

import (
	"database/sql"
	"encoding/base32"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/trisacrypto/daybreak/pkg/db/fields"
	"github.com/trisacrypto/daybreak/pkg/errors"

	//cSpell:ignore gsessions
	"github.com/gorilla/securecookie"
	gsessions "github.com/gorilla/sessions"
)

type SessionStore struct {
	db       *DB
	insert   *sql.Stmt
	update   *sql.Stmt
	delete   *sql.Stmt
	retrieve *sql.Stmt
	options  *sessions.Options
	codecs   []securecookie.Codec
}

var _ sessions.Store = (*SessionStore)(nil)

type Session struct {
	ID       string
	Data     string
	Created  fields.Timestamp
	Modified fields.Timestamp
	Expires  fields.Timestamp
}

const (
	insertSessionSQL   = `INSERT INTO http_sessions (id, data, created, modified, expires) VALUES ($1, $2, $3, $4, $5)`
	updateSessionSQL   = `UPDATE http_sessions SET data = $2, modified = $3, expires = $4 WHERE id = $1`
	deleteSessionSQL   = `DELETE FROM http_sessions WHERE id = $1`
	retrieveSessionSQL = `SELECT id, data, created, modified, expires FROM http_sessions WHERE id = $1`
	allSessionsSQL     = `SELECT id, expires FROM http_sessions`
)

// Keys are defined in pairs to allow key rotation, but the common case is to set a single
// authentication key and optionally an encryption key.
//
// The first key in a pair is used for authentication and the second for encryption. The
// encryption key can be set to nil or omitted in the last pair, but the authentication key
// is required in all pairs.
//
// It is recommended to use an authentication key with 32 or 64 bytes. The encryption key,
// if set, must be either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256 modes.
func (db *DB) Sessions(keyPairs ...[]byte) (_ sessions.Store, err error) {
	store := &SessionStore{
		db:     db,
		codecs: securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{
			Path:     "/",                  // Scope the session cooke to the root path
			MaxAge:   3600,                 // Session expires in 1 hour (seconds)
			HttpOnly: true,                 // Prevent JavaScript access to the cookie
			Secure:   true,                 // Only send the cookie over HTTPS
			SameSite: http.SameSiteLaxMode, // Prevent CSRF attacks
		},
	}

	// Ensure the codecs are updated with the max age.
	store.MaxAge(store.options.MaxAge)

	if store.insert, err = db.Prepare(insertSessionSQL); err != nil {
		return nil, err
	}

	if store.update, err = db.Prepare(updateSessionSQL); err != nil {
		return nil, err
	}

	if store.delete, err = db.Prepare(deleteSessionSQL); err != nil {
		return nil, err
	}

	if store.retrieve, err = db.Prepare(retrieveSessionSQL); err != nil {
		return nil, err
	}

	return store, nil
}

// Get should return a cached session.
// Implements the gorilla Store interface for session management.
func (db *SessionStore) Get(r *http.Request, name string) (*gsessions.Session, error) {
	fmt.Printf("Getting session: %s\n", name)
	return gsessions.GetRegistry(r).Get(db, name)
}

// New should create and return a new session.
//
// Note that New should never return a nil session, even in the case of
// an error if using the Registry infrastructure to cache the session.
// Implements the gorilla Store interface for session management.
func (db *SessionStore) New(r *http.Request, name string) (session *gsessions.Session, err error) {
	session = gsessions.NewSession(db, name)
	session.IsNew = true
	session.Options = db.options.ToGorillaOptions()

	if c, errCookie := r.Cookie(name); errCookie == nil {
		if err = securecookie.DecodeMulti(name, c.Value, &session.ID, db.codecs...); err == nil {
			if err = db.load(session); err == nil {
				fmt.Printf("DB record found for session, do not create a new session\n")
				session.IsNew = false
			} else if errors.Is(err, sql.ErrNoRows) {
				fmt.Printf("No rows found for session\n")
				err = nil
			}
		}
	}

	return session, err
}

// Save should persist session to the underlying store implementation.
// Implements the gorilla Store interface for session management.
func (db *SessionStore) Save(r *http.Request, w http.ResponseWriter, s *gsessions.Session) (err error) {
	// Delete if max-age is < 0
	if s.Options.MaxAge < 0 {
		if err = db.destroy(s); err != nil {
			return err
		}

		http.SetCookie(w, gsessions.NewCookie(s.Name(), "", s.Options))
		return nil
	}

	// Generate a random session ID if one isn't set.
	if s.ID == "" {
		s.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32),
			), "=")
	}

	// Save the session to the database.
	if err = db.save(s); err != nil {
		return err
	}

	// Keep the session ID key in a cookie so it can be looked up in the DB later.
	var encoded string
	if encoded, err = securecookie.EncodeMulti(s.Name(), s.ID, db.codecs...); err != nil {
		return err
	}

	http.SetCookie(w, gsessions.NewCookie(s.Name(), encoded, s.Options))
	return nil
}

func (db *SessionStore) Options(options sessions.Options) {
	db.options = &options
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
func (db *SessionStore) MaxLength(maxLength int) {
	for _, codec := range db.codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxLength(maxLength)
		}
	}
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting Options.MaxAge
// = -1 for that session.
func (db *SessionStore) MaxAge(maxAge int) {
	db.options.MaxAge = maxAge
	for _, codec := range db.codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(maxAge)
		}
	}
}

func (db *SessionStore) load(s *gsessions.Session) (err error) {
	row := &Session{}
	if err = row.Scan(db.retrieve.QueryRow(s.ID)); err != nil {
		return err
	}

	s.Values = make(map[any]any)
	return securecookie.DecodeMulti(s.Name(), row.Data, &s.Values, db.codecs...)
}

func (db *SessionStore) save(s *gsessions.Session) (err error) {
	// Create the record to insert or update.
	row := &Session{ID: s.ID, Modified: fields.TimeNow()}

	if row.Data, err = securecookie.EncodeMulti(s.Name(), s.Values, db.codecs...); err != nil {
		return err
	}

	if created, ok := s.Values["created_on"].(time.Time); ok {
		row.Created = fields.Time(created)
	} else {
		row.Created = row.Modified
	}

	if expires, ok := s.Values["expires_on"].(time.Time); ok {
		if expires.Sub(time.Now().Add(time.Second*time.Duration(s.Options.MaxAge))) < 0 {
			expires = time.Now().Add(time.Second * time.Duration(s.Options.MaxAge))
		}
		row.Expires = fields.Time(expires)
	} else {
		row.Expires = fields.Time(time.Now().Add(time.Second * time.Duration(s.Options.MaxAge)))
	}

	// Insert if the session is new.
	if s.IsNew {
		_, err = db.insert.Exec(row.InsertParams()...)
		return err
	}

	// Update if the session is not new.
	_, err = db.update.Exec(row.UpdateParams()...)
	return err
}

func (db *SessionStore) destroy(s *gsessions.Session) (err error) {
	_, err = db.delete.Exec(s.ID)
	return err
}

//============================================================================
// Session Methods
//============================================================================

func (s *Session) Scan(scanner Scanner) error {
	return scanner.Scan(
		&s.ID,
		&s.Data,
		&s.Created,
		&s.Modified,
		&s.Expires,
	)
}

func (s *Session) InsertParams() []any {
	return []any{
		s.ID,
		s.Data,
		s.Created,
		s.Modified,
		s.Expires,
	}
}

func (s *Session) UpdateParams() []any {
	return []any{
		s.ID,
		s.Data,
		s.Modified,
		s.Expires,
	}
}
