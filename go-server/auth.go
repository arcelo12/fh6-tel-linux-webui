package main

import (
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type UserRegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserVerifyRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SessionInfo struct {
	UserID    int64
	Username  string
	Role      string
	ExpiresAt time.Time
}

const sessionDuration = 30 * 24 * time.Hour

// hashPassword hashes a password using a random salt and multiple rounds of SHA-512.
// TODO(security): Replace this standard-library fallback with Argon2id or bcrypt once external package dependencies can be imported.
func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	if err != nil {
		return "", err
	}

	hash := sha512.New()
	hash.Write(salt)
	hash.Write([]byte(password))
	hashed := hash.Sum(nil)

	// PBKDF2-like loop to stretch the password hash and increase entropy
	for i := 0; i < 10000; i++ {
		hash.Reset()
		hash.Write(hashed)
		hashed = hash.Sum(nil)
	}

	saltHex := hex.EncodeToString(salt)
	hashHex := hex.EncodeToString(hashed)
	return fmt.Sprintf("%s:%s", saltHex, hashHex), nil
}

// verifyPassword verifies a password against a stored hash using constant-time comparison.
func verifyPassword(password, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	storedHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	hash := sha512.New()
	hash.Write(salt)
	hash.Write([]byte(password))
	hashed := hash.Sum(nil)

	for i := 0; i < 10000; i++ {
		hash.Reset()
		hash.Write(hashed)
		hashed = hash.Sum(nil)
	}

	return subtle.ConstantTimeCompare(hashed, storedHash) == 1
}

// createSessionToken generates a secure random session ID
func createSessionToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type RateLimiter struct {
	mu          sync.Mutex
	loginIPs    map[string][]time.Time
	registerIPs map[string][]time.Time
}

var authRateLimiter = &RateLimiter{
	loginIPs:    make(map[string][]time.Time),
	registerIPs: make(map[string][]time.Time),
}

func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// allowRequest checks if an IP has exceeded the limit within the time window.
// Uses globalConfig.Auth.MaxLoginAttempts and MaxRegisterAttempts.
func (rl *RateLimiter) allowRequest(ip string, isLogin bool) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	var window time.Duration
	var limit int
	var record map[string][]time.Time

	if isLogin {
		window = 15 * time.Minute // 15 mins for login
		limit = globalConfig.Auth.MaxLoginAttempts
		record = rl.loginIPs
	} else {
		window = 1 * time.Hour // 1 hour for register
		limit = globalConfig.Auth.MaxRegisterAttempts
		record = rl.registerIPs
	}

	times := record[ip]
	var validTimes []time.Time
	for _, t := range times {
		if now.Sub(t) <= window {
			validTimes = append(validTimes, t)
		}
	}

	if len(validTimes) >= limit {
		record[ip] = validTimes // Cleanup old ones while keeping the valid ones
		return false
	}

	validTimes = append(validTimes, now)
	record[ip] = validTimes
	return true
}

// Register Handler
func handleRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
		return
	}

	ip := getIP(r)
	if !authRateLimiter.allowRequest(ip, false) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "Terlalu banyak percobaan pendaftaran. Silakan coba lagi nanti."})
		return
	}

	// Validate inputs
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := req.Password

	if len(username) < 3 || len(username) > 30 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username must be between 3 and 30 characters"})
		return
	}
	if !strings.Contains(email, "@") || len(email) < 5 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email address"})
		return
	}
	if len(password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Password must be at least 8 characters long"})
		return
	}

	hashedPassword, err := hashPassword(password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	// If this is the very first user, make them an admin
	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	role := "user"
	if userCount == 0 {
		role = "admin"
	}

	// Generate Verification Token
	b := make([]byte, 3)
	rand.Read(b)
	verificationToken := hex.EncodeToString(b)
	// For admin, they could be auto-verified, but let's stick to standard flow unless we want admin auto-verification
	isVerified := 0
	if role == "admin" {
		isVerified = 1
	}

	// Insert into DB
	createdAt := time.Now().UnixMilli()
	_, err = db.Exec(
		"INSERT INTO users (username, email, password_hash, created_at, role, is_verified, verification_token) VALUES (?, ?, ?, ?, ?, ?, ?)",
		username, email, hashedPassword, createdAt, role, isVerified, verificationToken,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Username or Email already registered"})
			return
		}
		log.Printf("Error registering user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create user"})
		return
	}

	if isVerified == 0 {
		// Send Email
		go sendVerificationEmail(email, username, verificationToken)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful", "needs_verification": fmt.Sprintf("%t", isVerified == 0)})
}

// Verify Handler
func handleVerify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req UserVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	res, err := db.Exec("UPDATE users SET is_verified = 1, verification_token = NULL WHERE email = ? AND verification_token = ?", req.Email, req.Token)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid verification code or email"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Account verified successfully"})
}

// Login Handler
func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
		return
	}

	ip := getIP(r)
	if !authRateLimiter.allowRequest(ip, true) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "Terlalu banyak percobaan login gagal. Tunggu 15 menit."})
		return
	}

	email := strings.TrimSpace(req.Email)
	password := req.Password

	var userID int64
	var username, storedHash string
	var isVerified int
	err := db.QueryRow(
		"SELECT id, username, password_hash, is_verified FROM users WHERE email = ?",
		email,
	).Scan(&userID, &username, &storedHash, &isVerified)

	// Validate credentials
	if err == sql.ErrNoRows || !verifyPassword(password, storedHash) {
		w.WriteHeader(http.StatusUnauthorized)
		// Generic error message to prevent account harvesting
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
		return
	} else if err != nil {
		log.Printf("Login database error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		return
	}

	if isVerified == 0 {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account not verified", "needs_verification": "true"})
		return
	}

	// Create session token
	token, err := createSessionToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
		return
	}

	// Save session in DB
	expiresAt := time.Now().Add(sessionDuration)
	_, err = db.Exec(
		"INSERT INTO auth_sessions (session_id, user_id, username, expires_at) VALUES (?, ?, ?, ?)",
		token, userID, username, expiresAt.UnixMilli(),
	)
	if err != nil {
		log.Printf("Failed to save auth session: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to store session"})
		return
	}

	// Provision Cookie: HttpOnly, Secure, SameSite=Lax
	secureFlag := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secureFlag,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Login successful",
		"username": username,
	})
}

// Logout Handler
func handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	cookie, err := r.Cookie("session_id")
	if err == nil {
		_, _ = db.Exec("DELETE FROM auth_sessions WHERE session_id = ?", cookie.Value)
	}

	// Expire the cookie
	secureFlag := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	expiredCookie := &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secureFlag,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, expiredCookie)

	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

// checkSession is a helper middleware to validate user session and return SessionInfo
func checkSession(r *http.Request) (*SessionInfo, bool) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil, false
	}

	var userID int64
	var username string
	var expiresAtMs int64
	err = db.QueryRow(
		"SELECT user_id, username, expires_at FROM auth_sessions WHERE session_id = ?",
		cookie.Value,
	).Scan(&userID, &username, &expiresAtMs)
	if err != nil {
		return nil, false
	}

	expiresAt := time.UnixMilli(expiresAtMs)
	if time.Now().After(expiresAt) {
		// Session expired, clean up
		_, _ = db.Exec("DELETE FROM auth_sessions WHERE session_id = ?", cookie.Value)
		return nil, false
	}

	// Fetch role from users table
	var role string
	err = db.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		role = "user" // Default fallback
	}

	return &SessionInfo{
		UserID:    userID,
		Username:  username,
		Role:      role,
		ExpiresAt: expiresAt,
	}, true
}

// Me Handler
func handleMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	session, ok := checkSession(r)
	if !ok {
		log.Printf("[Auth Debug] handleMe: session invalid/unauthorized")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	log.Printf("[Auth Debug] handleMe: authenticated successfully as %s (ID %d) role=%s", session.Username, session.UserID, session.Role)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       session.UserID,
		"username": session.Username,
		"role":     session.Role,
	})
}

// Get Dashboard Layout Handler
func handleGetLayout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := checkSession(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var layout sql.NullString
	err := db.QueryRow("SELECT dashboard_layout FROM users WHERE id = ?", session.UserID).Scan(&layout)
	if err != nil || !layout.Valid {
		json.NewEncoder(w).Encode(map[string]interface{}{"layout": nil})
		return
	}
	
	// Pass the raw JSON string back (parsed as JSON so it doesn't double-escape)
	w.Write([]byte(fmt.Sprintf(`{"layout": %s}`, layout.String)))
}

// Save Dashboard Layout Handler
func handleSaveLayout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, ok := checkSession(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var req struct {
		Layout interface{} `json:"layout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	layoutBytes, _ := json.Marshal(req.Layout)
	_, err := db.Exec("UPDATE users SET dashboard_layout = ? WHERE id = ?", string(layoutBytes), session.UserID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
