// Package cache provides Redis store for gin sessions.
package cache

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/gob"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	gorillasessions "github.com/gorilla/sessions"
	"github.com/gorilla/securecookie"
	"github.com/redis/go-redis/v9"
)

const (
	defaultMaxAge = 86400 * 7 // 7 days
)

// RedisStore stores sessions in Redis.
type RedisStore struct {
	client  *redis.Client
	Codecs  []securecookie.Codec
	options *sessions.Options
}

// NewRedisStore creates a new Redis store.
func NewRedisStore(client *redis.Client, keyPairs ...[]byte) *RedisStore {
	rs := &RedisStore{
		client: client,
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		options: &sessions.Options{
			Path:   "/",
			MaxAge: defaultMaxAge,
		},
	}
	return rs
}

// Options sets the options for the store.
func (s *RedisStore) Options(opts sessions.Options) {
	s.options = &opts
}

// Get retrieves a session from Redis.
func (s *RedisStore) Get(r *http.Request, name string) (*gorillasessions.Session, error) {
	return gorillasessions.GetRegistry(r).Get(s, name)
}

// New creates a new session.
func (s *RedisStore) New(r *http.Request, name string) (*gorillasessions.Session, error) {
	session := gorillasessions.NewSession(s, name)
	session.Options = &gorillasessions.Options{
		Path:     s.options.Path,
		Domain:    s.options.Domain,
		MaxAge:    s.options.MaxAge,
		Secure:    s.options.Secure,
		HttpOnly:  s.options.HttpOnly,
		SameSite:  s.options.SameSite,
	}
	session.IsNew = true
	
	// Try to load existing session from cookie
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err := securecookie.DecodeMulti(name, c.Value, &session.ID, s.Codecs...)
		if err == nil {
			// Successfully decoded session ID, try to load from Redis
			err = s.load(session)
			if err == nil {
				session.IsNew = false
			}
			// If load fails, continue with new session (session.IsNew = true)
		}
		// If decode fails (e.g., old cookie format), ignore and create new session
	}
	
	return session, nil
}

// Save saves a session to Redis.
func (s *RedisStore) Save(r *http.Request, w http.ResponseWriter, session *gorillasessions.Session) error {
	// Delete if max age is < 0
	if session.Options.MaxAge < 0 {
		if err := s.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, s.newCookie(session, ""))
		return nil
	}

	if session.ID == "" {
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32),
			), "=")
	}
	
	if err := s.save(session); err != nil {
		return err
	}
	
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.Codecs...)
	if err != nil {
		return err
	}
	
	http.SetCookie(w, s.newCookie(session, encoded))
	return nil
}

// newCookie creates a new HTTP cookie for the session.
func (s *RedisStore) newCookie(session *gorillasessions.Session, value string) *http.Cookie {
	cookie := &http.Cookie{
		Name:     session.Name(),
		Value:    value,
		Path:     session.Options.Path,
		Domain:   session.Options.Domain,
		MaxAge:   session.Options.MaxAge,
		Secure:   session.Options.Secure,
		HttpOnly: session.Options.HttpOnly,
		SameSite: session.Options.SameSite,
	}
	if session.Options.MaxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(session.Options.MaxAge) * time.Second)
	}
	return cookie
}

// save stores session data in Redis.
func (s *RedisStore) save(session *gorillasessions.Session) error {
	// Use gob encoding to preserve types (especially for model.User)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(session.Values); err != nil {
		return fmt.Errorf("failed to encode session values: %w", err)
	}
	
	maxAge := session.Options.MaxAge
	if maxAge == 0 {
		maxAge = s.options.MaxAge
	}
	
	key := fmt.Sprintf("session:%s", session.ID)
	return s.client.Set(context.Background(), key, buf.Bytes(), time.Duration(maxAge)*time.Second).Err()
}

// load retrieves session data from Redis.
func (s *RedisStore) load(session *gorillasessions.Session) error {
	key := fmt.Sprintf("session:%s", session.ID)
	data, err := s.client.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return fmt.Errorf("session not found")
	}
	if err != nil {
		return err
	}
	
	// Use gob decoding to preserve types (especially for model.User)
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&session.Values); err != nil {
		return fmt.Errorf("failed to decode session data: %w", err)
	}
	
	return nil
}

// delete removes session from Redis.
func (s *RedisStore) delete(session *gorillasessions.Session) error {
	key := fmt.Sprintf("session:%s", session.ID)
	return s.client.Del(context.Background(), key).Err()
}
