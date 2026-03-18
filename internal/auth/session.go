package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	sessionName   = "hakken_session"
	keyUserID     = "user_id"
	keyFlash      = "flash"
	keyDateFormat = "date_format"
)

var store *sessions.CookieStore

// InitStore initialises the session cookie store. Call once at startup.
func InitStore(secret string, secure bool) {
	store = sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func getSession(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, sessionName)
}

// SetUserID stores the user ID in the session and saves it.
func SetUserID(w http.ResponseWriter, r *http.Request, userID string) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values[keyUserID] = userID
	return sess.Save(r, w)
}

// GetUserID retrieves the user ID from the session. Returns "" if not present.
func GetUserID(r *http.Request) string {
	sess, err := getSession(r)
	if err != nil {
		return ""
	}
	v, _ := sess.Values[keyUserID].(string)
	return v
}

// ClearSession removes all session values and saves.
func ClearSession(w http.ResponseWriter, r *http.Request) {
	sess, err := getSession(r)
	if err != nil {
		return
	}
	sess.Values = map[interface{}]interface{}{}
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
}

// SetFlash saves a one-shot flash message to the session.
func SetFlash(w http.ResponseWriter, r *http.Request, msg string) {
	sess, err := getSession(r)
	if err != nil {
		return
	}
	sess.AddFlash(msg, keyFlash)
	_ = sess.Save(r, w)
}

// SetDateFormat stores the user's date format preference in the session.
func SetDateFormat(w http.ResponseWriter, r *http.Request, pref string) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values[keyDateFormat] = pref
	return sess.Save(r, w)
}

// GetDateFormat retrieves the date format preference from the session. Returns "dmy" if absent.
func GetDateFormat(r *http.Request) string {
	sess, err := getSession(r)
	if err != nil {
		return "dmy"
	}
	v, _ := sess.Values[keyDateFormat].(string)
	if v == "" {
		return "dmy"
	}
	return v
}

// GetFlash retrieves and clears the flash message from the session.
func GetFlash(w http.ResponseWriter, r *http.Request) string {
	sess, err := getSession(r)
	if err != nil {
		return ""
	}
	flashes := sess.Flashes(keyFlash)
	if len(flashes) == 0 {
		return ""
	}
	_ = sess.Save(r, w)
	if msg, ok := flashes[0].(string); ok {
		return msg
	}
	return ""
}
