package server

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const passwordIterations = 600_000

var userStoreMu sync.Mutex

type passwordUser struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Salt    string `json:"salt"`
	Hash    string `json:"hash"`
	Created int64  `json:"created"`
}

func (s *Server) passwordAuthRoutes() {
	s.mux.HandleFunc("POST /auth/register", s.sameOrigin(s.register))
	s.mux.HandleFunc("POST /auth/login", s.sameOrigin(s.passwordLogin))
	s.mux.HandleFunc("POST /auth/password", s.sameOrigin(s.requireLogin(s.changePassword)))
	s.mux.HandleFunc("POST /auth/logout-all", s.sameOrigin(s.requireLogin(s.logoutAll)))
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := s.sessionUser(r)
	if !ok || session.Provider != "password" {
		writeError(w, http.StatusForbidden, "parola hesabı gerekli")
		return
	}
	var request struct{ Current, Password string }
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || len(request.Password) < 12 || len(request.Password) > 1024 {
		writeError(w, 400, "geçerli mevcut parola ve en az 12 karakter yeni parola gerekli")
		return
	}
	userStoreMu.Lock()
	defer userStoreMu.Unlock()
	users, err := readUsers()
	if err != nil {
		writeError(w, 500, "parola değiştirilemedi")
		return
	}
	for i := range users {
		if users[i].ID != session.ID {
			continue
		}
		salt, _ := base64.RawStdEncoding.DecodeString(users[i].Salt)
		expected, _ := base64.RawStdEncoding.DecodeString(users[i].Hash)
		actual, _ := passwordHash(request.Current, salt)
		if !hmac.Equal(actual, expected) {
			writeError(w, 401, "mevcut parola hatalı")
			return
		}
		newSalt := make([]byte, 16)
		_, _ = rand.Read(newSalt)
		hash, _ := passwordHash(request.Password, newSalt)
		users[i].Salt = base64.RawStdEncoding.EncodeToString(newSalt)
		users[i].Hash = base64.RawStdEncoding.EncodeToString(hash)
		users[i].ID = randomToken()
		if writeUsers(users) != nil {
			writeError(w, 500, "parola kaydedilemedi")
			return
		}
		s.issuePasswordSession(w, users[i])
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeError(w, 404, "hesap bulunamadı")
}

func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) {
	session, ok := s.sessionUser(r)
	if !ok || session.Provider != "password" {
		writeError(w, 403, "parola hesabı gerekli")
		return
	}
	userStoreMu.Lock()
	defer userStoreMu.Unlock()
	users, err := readUsers()
	if err != nil {
		writeError(w, 500, "oturumlar kapatılamadı")
		return
	}
	for i := range users {
		if users[i].ID == session.ID {
			users[i].ID = randomToken()
			if writeUsers(users) != nil {
				writeError(w, 500, "oturumlar kapatılamadı")
				return
			}
			s.clearCookie(w, "prompter_session")
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	writeError(w, 404, "hesap bulunamadı")
}

func usersPath() (string, error) {
	path, err := projectStorePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "users.json"), nil
}

func readUsers() ([]passwordUser, error) {
	path, err := usersPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []passwordUser{}, nil
	}
	if err != nil {
		return nil, err
	}
	var users []passwordUser
	if json.Unmarshal(data, &users) != nil {
		return nil, errors.New("kullanıcı deposu bozuk")
	}
	return users, nil
}

func writeUsers(users []passwordUser) error {
	path, err := usersPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "users-*.json")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(users); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func normalizeEmail(value string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(email)
	return email, err == nil && address.Address == email && len(email) <= 254
}

func passwordHash(password string, salt []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, password, salt, passwordIterations, 32)
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
	var request struct{ Email, Password, Name string }
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "geçerli giriş bilgileri gerekli")
		return "", "", "", false
	}
	email, ok := normalizeEmail(request.Email)
	if !ok || len(request.Password) < 12 || len(request.Password) > 1024 {
		writeError(w, http.StatusBadRequest, "geçerli e-posta ve en az 12 karakter parola gerekli")
		return "", "", "", false
	}
	return email, request.Password, strings.TrimSpace(request.Name), true
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	email, password, name, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	if name == "" || len([]rune(name)) > 100 {
		writeError(w, http.StatusBadRequest, "ad gerekli")
		return
	}
	userStoreMu.Lock()
	defer userStoreMu.Unlock()
	users, err := readUsers()
	if err != nil {
		writeError(w, 500, "kullanıcı deposu okunamadı")
		return
	}
	for _, user := range users {
		if user.Email == email {
			writeError(w, http.StatusConflict, "bu e-posta zaten kayıtlı")
			return
		}
	}
	salt := make([]byte, 16)
	id := randomToken()
	if _, err := rand.Read(salt); err != nil || id == "" {
		writeError(w, 500, "güvenli kayıt oluşturulamadı")
		return
	}
	hash, err := passwordHash(password, salt)
	if err != nil {
		writeError(w, 500, "parola işlenemedi")
		return
	}
	user := passwordUser{ID: id, Email: email, Name: name, Salt: base64.RawStdEncoding.EncodeToString(salt), Hash: base64.RawStdEncoding.EncodeToString(hash), Created: time.Now().Unix()}
	if writeUsers(append(users, user)) != nil {
		writeError(w, 500, "kullanıcı kaydedilemedi")
		return
	}
	s.issuePasswordSession(w, user)
	writeJSON(w, http.StatusCreated, map[string]bool{"authenticated": true})
}

func (s *Server) passwordLogin(w http.ResponseWriter, r *http.Request) {
	email, password, _, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	if allowed, _ := s.rate.allow("login|"+r.RemoteAddr, time.Now()); !allowed {
		writeError(w, http.StatusTooManyRequests, "çok fazla giriş denemesi")
		return
	}
	userStoreMu.Lock()
	users, err := readUsers()
	userStoreMu.Unlock()
	if err != nil {
		writeError(w, 500, "giriş yapılamadı")
		return
	}
	var found *passwordUser
	for i := range users {
		if users[i].Email == email {
			found = &users[i]
			break
		}
	}
	salt := make([]byte, 16)
	expected := make([]byte, 32)
	if found != nil {
		salt, _ = base64.RawStdEncoding.DecodeString(found.Salt)
		expected, _ = base64.RawStdEncoding.DecodeString(found.Hash)
	}
	actual, _ := passwordHash(password, salt)
	if found == nil || !hmac.Equal(actual, expected) {
		writeError(w, http.StatusUnauthorized, "e-posta veya parola hatalı")
		return
	}
	s.issuePasswordSession(w, *found)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

func (s *Server) issuePasswordSession(w http.ResponseWriter, user passwordUser) {
	session := authUser{ID: user.ID, Provider: "password", Name: user.Name, Email: user.Email, Expires: time.Now().Add(7 * 24 * time.Hour).Unix()}
	data, _ := json.Marshal(session)
	s.setSignedCookie(w, "prompter_session", string(data), 7*24*time.Hour)
}
