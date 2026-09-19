package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "sv_admin"
	sessionTTL    = 72 * time.Hour
)

type Auth struct {
	password string
	secret   []byte
	secure   bool
}

func New(password string, secret []byte, secure bool) *Auth {
	return &Auth{password: password, secret: secret, secure: secure}
}

func (a *Auth) sign(payload string) string {
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *Auth) token(exp int64) string {
	p := strconv.FormatInt(exp, 10)
	return p + "." + a.sign(p)
}

func (a *Auth) validToken(tok string) bool {
	parts := strings.SplitN(tok, ".", 2)
	if len(parts) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(a.sign(parts[0])), []byte(parts[1]))
}

func (a *Auth) CheckPassword(pw string) bool {
	return subtle.ConstantTimeCompare([]byte(pw), []byte(a.password)) == 1
}

func (a *Auth) SetCookie(w http.ResponseWriter) {
	exp := time.Now().Add(sessionTTL).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    a.token(exp),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.secure,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

func (a *Auth) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1,
	})
}

func (a *Auth) IsAdmin(r *http.Request) bool {
	if tok := strings.TrimSpace(r.Header.Get("X-Admin-Token")); tok != "" {
		if a.CheckPassword(tok) {
			return true
		}
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return a.validToken(c.Value)
}
