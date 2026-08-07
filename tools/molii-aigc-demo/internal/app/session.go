package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"molii-aigc-demo/internal/store"
)

const sessionCookieName = "molii_demo_session"

func (s *Server) ensureSession(w http.ResponseWriter, r *http.Request) (store.UISession, string, error) {
	now := time.Now().UTC()
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		raw := strings.TrimSpace(cookie.Value)
		if validSessionToken(raw) {
			sessionID := sessionID(raw)
			session, getErr := s.store.GetUISession(r.Context(), sessionID)
			if getErr == nil && session.ExpiresAt.After(now) {
				csrf := s.csrfToken(raw)
				csrfHash := sha256.Sum256([]byte(csrf))
				if subtle.ConstantTimeCompare(session.CSRFTokenHash, csrfHash[:]) == 1 {
					expires := now.Add(s.sessionTTL)
					if touchErr := s.store.TouchUISession(r.Context(), session.ID, now, expires); touchErr != nil {
						return store.UISession{}, "", touchErr
					}
					session.ExpiresAt = expires
					session.LastSeenAt = &now
					s.setSessionCookie(w, raw, expires)
					return session, csrf, nil
				}
			}
			if getErr == nil || errors.Is(getErr, store.ErrNotFound) {
				_ = s.store.DeleteUISession(r.Context(), sessionID)
			}
		}
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return store.UISession{}, "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(rawBytes)
	csrf := s.csrfToken(raw)
	csrfHash := sha256.Sum256([]byte(csrf))
	expires := now.Add(s.sessionTTL)
	session, err := s.store.CreateUISession(r.Context(), store.UISession{
		ID:            sessionID(raw),
		CSRFTokenHash: csrfHash[:],
		ExpiresAt:     expires,
		LastSeenAt:    &now,
	})
	if err != nil {
		return store.UISession{}, "", err
	}
	s.setSessionCookie(w, raw, expires)
	return session, csrf, nil
}

func validSessionToken(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == 32
}

func sessionID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Server) csrfToken(raw string) string {
	mac := hmac.New(sha256.New, s.sessionSecret)
	_, _ = mac.Write([]byte("molii-demo-csrf\x00"))
	_, _ = mac.Write([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: value, Path: "/", Expires: expires,
		MaxAge: int(s.sessionTTL.Seconds()), HttpOnly: true, Secure: s.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) withWriteSession(next func(http.ResponseWriter, *http.Request, store.UISession)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _, err := s.ensureSession(w, r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_session", "UI session is unavailable")
			return
		}
		if !sameRequestOrigin(r) {
			writeError(w, http.StatusForbidden, "invalid_origin", "Request origin does not match the Demo")
			return
		}
		provided := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
		providedHash := sha256.Sum256([]byte(provided))
		if provided == "" || subtle.ConstantTimeCompare(session.CSRFTokenHash, providedHash[:]) != 1 {
			writeError(w, http.StatusForbidden, "csrf_failed", "CSRF token is invalid")
			return
		}
		next(w, r, session)
	}
}

func sameRequestOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser clients still need the unguessable CSRF value.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	requestScheme := "http"
	if r.TLS != nil {
		requestScheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		requestScheme = forwarded
	}
	return strings.EqualFold(parsed.Scheme, requestScheme) && strings.EqualFold(parsed.Host, r.Host)
}

func (s *Server) hostAllowed(rawHost string) bool {
	host := strings.TrimSpace(strings.ToLower(rawHost))
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	_, ok := s.allowedHosts[host]
	return ok
}
