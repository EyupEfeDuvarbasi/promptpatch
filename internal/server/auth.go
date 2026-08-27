package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type authProvider struct{ id, secret, authorize, token, userinfo, scope string }
type authUser struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar,omitempty"`
	Expires  int64  `json:"expires"`
}

func (s *Server) authRoutes() {
	s.mux.HandleFunc("GET /auth/{provider}", s.authStart)
	s.mux.HandleFunc("GET /auth/{provider}/callback", s.authCallback)
	s.mux.HandleFunc("GET /v1/me", s.me)
	s.mux.HandleFunc("POST /auth/logout", s.sameOrigin(s.logout))
	s.passwordAuthRoutes()
}

func (s *Server) sameOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && strings.TrimRight(origin, "/") != strings.TrimRight(s.config.PublicURL, "/") {
			writeError(w, http.StatusForbidden, "geçersiz origin")
			return
		}
		next(w, r)
	}
}

func (s *Server) authEnabled() bool {
	_, google := s.provider("google")
	_, github := s.provider("github")
	users, _ := readUsers()
	return google || github || len(users) > 0
}

func (s *Server) sessionUser(r *http.Request) (authUser, bool) {
	value, err := s.readSignedCookie(r, "prompter_session")
	var user authUser
	if err != nil || json.Unmarshal([]byte(value), &user) != nil || user.Expires < time.Now().Unix() {
		return authUser{}, false
	}
	if user.Provider == "password" {
		users, err := readUsers()
		if err != nil {
			return authUser{}, false
		}
		found := false
		for _, stored := range users {
			if stored.ID == user.ID {
				found = true
				break
			}
		}
		if !found {
			return authUser{}, false
		}
	}
	return user, true
}

func (s *Server) requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.authEnabled() {
			if _, ok := s.sessionUser(r); !ok {
				writeError(w, http.StatusUnauthorized, "giriş gerekli")
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) provider(name string) (authProvider, bool) {
	providers := map[string]authProvider{
		"github": {s.config.GitHubID, s.config.GitHubSecret, "https://github.com/login/oauth/authorize", "https://github.com/login/oauth/access_token", "https://api.github.com/user", "read:user user:email"},
		"google": {s.config.GoogleID, s.config.GoogleSecret, "https://accounts.google.com/o/oauth2/v2/auth", "https://oauth2.googleapis.com/token", "https://openidconnect.googleapis.com/v1/userinfo", "openid email profile"},
	}
	p, exists := providers[name]
	return p, exists && p.id != "" && (name == "google" || p.secret != "")
}

func randomToken() string {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(value)
}

func (s *Server) authStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")
	p, ok := s.provider(name)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "giriş sağlayıcısı yapılandırılmamış")
		return
	}
	state, verifier := randomToken(), randomToken()
	s.setSignedCookie(w, "prompter_oauth", name+"|"+state+"|"+verifier, 10*time.Minute)
	challenge := sha256.Sum256([]byte(verifier))
	query := url.Values{"client_id": {p.id}, "redirect_uri": {s.callbackURL(name)}, "response_type": {"code"}, "scope": {p.scope}, "state": {state}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"}}
	http.Redirect(w, r, p.authorize+"?"+query.Encode(), http.StatusFound)
}

func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")
	p, ok := s.provider(name)
	flow, err := s.readSignedCookie(r, "prompter_oauth")
	parts := strings.Split(flow, "|")
	if !ok || err != nil || len(parts) != 3 || parts[0] != name || !hmac.Equal([]byte(parts[1]), []byte(r.URL.Query().Get("state"))) {
		writeError(w, http.StatusBadRequest, "OAuth state doğrulanamadı")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "OAuth kodu eksik")
		return
	}
	token, err := s.exchange(p, name, code, parts[2])
	if err != nil {
		writeError(w, http.StatusBadGateway, "OAuth token alınamadı")
		return
	}
	user, err := s.fetchUser(p, name, token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "kullanıcı kimliği doğrulanamadı")
		return
	}
	user.Expires = time.Now().Add(7 * 24 * time.Hour).Unix()
	encoded, _ := json.Marshal(user)
	s.setSignedCookie(w, "prompter_session", string(encoded), 7*24*time.Hour)
	s.clearCookie(w, "prompter_oauth")
	http.Redirect(w, r, "/#overview", http.StatusFound)
}

func (s *Server) exchange(p authProvider, name, code, verifier string) (string, error) {
	form := url.Values{"client_id": {p.id}, "code": {code}, "redirect_uri": {s.callbackURL(name)}, "grant_type": {"authorization_code"}, "code_verifier": {verifier}}
	if p.secret != "" {
		form.Set("client_secret", p.secret)
	}
	req, _ := http.NewRequest(http.MethodPost, p.token, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	response, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return "", errors.New("token response")
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body) != nil || body.AccessToken == "" {
		return "", errors.New("token missing")
	}
	return body.AccessToken, nil
}

func (s *Server) fetchUser(p authProvider, name, token string) (authUser, error) {
	req, _ := http.NewRequest(http.MethodGet, p.userinfo, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Prompter")
	response, err := s.client.Do(req)
	if err != nil {
		return authUser{}, err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return authUser{}, errors.New("userinfo response")
	}
	var raw struct {
		ID      any    `json:"id"`
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Login   string `json:"login"`
		Email   string `json:"email"`
		Avatar  string `json:"avatar_url"`
		Picture string `json:"picture"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&raw) != nil {
		return authUser{}, errors.New("userinfo json")
	}
	id := raw.Sub
	if id == "" {
		id = fmt.Sprint(raw.ID)
	}
	display, avatar := raw.Name, raw.Picture
	if display == "" {
		display = raw.Login
	}
	if avatar == "" {
		avatar = raw.Avatar
	}
	if id == "" || id == "<nil>" {
		return authUser{}, errors.New("identity missing")
	}
	return authUser{ID: id, Provider: name, Name: display, Email: raw.Email, Avatar: avatar}, nil
}

func (s *Server) callbackURL(provider string) string {
	return strings.TrimRight(s.config.PublicURL, "/") + "/auth/" + provider + "/callback"
}
func (s *Server) sign(value string) string {
	mac := hmac.New(sha256.New, s.sessionKey)
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Server) setSignedCookie(w http.ResponseWriter, name, value string, age time.Duration) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(value))
	http.SetCookie(w, &http.Cookie{Name: name, Value: encoded + "." + s.sign(encoded), Path: "/", MaxAge: int(age.Seconds()), HttpOnly: true, Secure: strings.HasPrefix(s.config.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
}
func (s *Server) readSignedCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(s.sign(parts[0])), []byte(parts[1])) {
		return "", errors.New("invalid cookie")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	return string(decoded), err
}
func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1, HttpOnly: true, Secure: strings.HasPrefix(s.config.PublicURL, "https://"), SameSite: http.SameSiteLaxMode})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := s.sessionUser(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "providers": map[string]bool{"google": s.config.GoogleID != "", "github": s.config.GitHubID != "" && s.config.GitHubSecret != ""}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": user})
}
func (s *Server) logout(w http.ResponseWriter, _ *http.Request) {
	s.clearCookie(w, "prompter_session")
	w.WriteHeader(http.StatusNoContent)
}
