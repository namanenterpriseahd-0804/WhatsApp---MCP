package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
	"github.com/rs/cors"
)

var (
	db       *sql.DB
	jwtKey   = []byte("super-secret-key-replace-in-production")
	jobs     = make(map[string]*BulkJob)
	jobsMu   sync.RWMutex
)

// ── Models ────────────────────────────────────────────────────────────────────

type User struct {
	ID            string
	Email         string
	PasswordHash  string
	AccessToken   string
	PhoneNumberID string
}

type BulkJob struct {
	ID          string    `json:"id"`
	UserID      string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Sent        int       `json:"sent"`
	Failed      int       `json:"failed"`
	Log         []string  `json:"log"`
	mu          sync.Mutex
}

func (j *BulkJob) addLog(msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Log = append(j.Log, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
}

func (j *BulkJob) setStatus(s string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = s
}

type ContactRow struct {
	Phone     string            `json:"phone"`
	Variables map[string]string `json:"variables"`
}

type BulkSendRequest struct {
	Contacts    []ContactRow `json:"contacts"`
	Template    string       `json:"template"`
	MediaBase64 string       `json:"media_base64,omitempty"`
	MediaType   string       `json:"media_type,omitempty"`
	MediaMime   string       `json:"media_mime,omitempty"`
	MediaName   string       `json:"media_name,omitempty"`
	ScheduleAt  string       `json:"schedule_at,omitempty"`
	DelayMin    float64      `json:"delay_min"`
	DelayMax    float64      `json:"delay_max"`
}

// ── Database ──────────────────────────────────────────────────────────────────

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "saas.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE,
			password_hash TEXT,
			access_token TEXT,
			phone_number_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}
}

func getUserByID(id string) (*User, error) {
	var u User
	err := db.QueryRow("SELECT id, email, password_hash, access_token, phone_number_id FROM users WHERE id = ?", id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.AccessToken, &u.PhoneNumberID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ── Auth Middleware ───────────────────────────────────────────────────────────

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing or invalid token"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid claims"})
			return
		}

		userID, _ := claims["user_id"].(string)
		r.Header.Set("X-User-ID", userID)
		next.ServeHTTP(w, r)
	}
}

// ── Auth Handlers ─────────────────────────────────────────────────────────────

func registerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = db.Exec("INSERT INTO users (id, email, password_hash, access_token, phone_number_id) VALUES (?, ?, ?, '', '')", userID, req.Email, string(hash))
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "Email already exists"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Registered successfully"})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var u User
	err := db.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", req.Email).Scan(&u.ID, &u.PasswordHash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": u.ID,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString(jwtKey)

	json.NewEncoder(w).Encode(map[string]interface{}{"token": tokenStr})
}

// ── Settings Handlers ─────────────────────────────────────────────────────────

func getSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	u, err := getUserByID(userID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"email":           u.Email,
		"has_credentials": u.AccessToken != "" && u.PhoneNumberID != "",
		"phone_number_id": u.PhoneNumberID,
	})
}

func saveSettingsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	var req struct {
		AccessToken   string `json:"access_token"`
		PhoneNumberID string `json:"phone_number_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, err := db.Exec("UPDATE users SET access_token = ?, phone_number_id = ? WHERE id = ?", req.AccessToken, req.PhoneNumberID, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// ── Meta API Helpers ──────────────────────────────────────────────────────────

func substituteTemplate(tmpl string, vars map[string]string) string {
	for k, v := range vars {
		tmpl = strings.ReplaceAll(tmpl, "{"+k+"}", v)
	}
	return tmpl
}

func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	return phone
}

func uploadMediaToGraphAPI(token, phoneID string, data []byte, filename, mimeType string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/media", phoneID)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("messaging_product", "whatsapp")
	part, _ := writer.CreateFormFile("file", filename)
	part.Write(data)
	writer.WriteField("type", mimeType)
	writer.Close()

	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("media upload failed: %s", string(respBody))
	}
	var res struct{ ID string `json:"id"` }
	json.Unmarshal(respBody, &res)
	return res.ID, nil
}

func sendCloudAPIMessage(token, phoneID, phone, text, mediaID, mediaType string) error {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", phoneID)
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                phone,
	}

	if mediaID != "" {
		payload["type"] = mediaType
		mediaObj := map[string]string{"id": mediaID}
		if text != "" && mediaType != "audio" {
			mediaObj["caption"] = text
		}
		payload[mediaType] = mediaObj
	} else {
		payload["type"] = "text"
		payload["text"] = map[string]interface{}{"preview_url": false, "body": text}
	}

	jsonData, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API Error: %s", string(respBody))
	}
	return nil
}

// ── Bulk Send Handlers ────────────────────────────────────────────────────────

func bulkSendHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	u, err := getUserByID(userID)
	if err != nil || u.AccessToken == "" || u.PhoneNumberID == "" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Meta Cloud API credentials not configured in settings."})
		return
	}

	var req BulkSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid payload"})
		return
	}
	if len(req.Contacts) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "No contacts provided"})
		return
	}

	scheduleAt := time.Now()
	if req.ScheduleAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ScheduleAt); err == nil {
			scheduleAt = t
		}
	}

	var mediaData []byte
	if req.MediaBase64 != "" {
		mediaData, _ = base64.StdEncoding.DecodeString(req.MediaBase64)
	}

	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	job := &BulkJob{
		ID:          jobID,
		UserID:      userID,
		CreatedAt:   time.Now(),
		ScheduledAt: scheduleAt,
		Status:      "pending",
		Total:       len(req.Contacts),
	}
	jobsMu.Lock()
	jobs[jobID] = job
	jobsMu.Unlock()

	go runBulkJob(job, req, mediaData, u.AccessToken, u.PhoneNumberID)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"job_id":       jobID,
		"total":        len(req.Contacts),
		"scheduled_at": scheduleAt.Format(time.RFC3339),
	})
}

func jobsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")

	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		jobsMu.Lock()
		if job, ok := jobs[id]; ok && job.UserID == userID {
			delete(jobs, id)
		}
		jobsMu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return
	}

	if id := r.URL.Query().Get("id"); id != "" {
		jobsMu.RLock()
		job, ok := jobs[id]
		jobsMu.RUnlock()
		if !ok || job.UserID != userID {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(job)
		return
	}

	jobsMu.RLock()
	defer jobsMu.RUnlock()
	out := make([]*BulkJob, 0)
	for _, j := range jobs {
		if j.UserID == userID {
			out = append(out, j)
		}
	}
	json.NewEncoder(w).Encode(out)
}

func runBulkJob(job *BulkJob, req BulkSendRequest, mediaData []byte, token, phoneID string) {
	if wait := time.Until(job.ScheduledAt); wait > 0 {
		job.addLog(fmt.Sprintf("Waiting until %s", job.ScheduledAt.Format("2006-01-02 15:04:05")))
		time.Sleep(wait)
	}

	job.setStatus("running")
	job.addLog(fmt.Sprintf("Starting — %d contacts", job.Total))

	var mediaID string
	if len(mediaData) > 0 {
		mime := req.MediaMime
		if mime == "" {
			mime = "image/jpeg"
		}
		filename := req.MediaName
		if filename == "" {
			filename = "upload.jpg"
		}
		id, err := uploadMediaToGraphAPI(token, phoneID, mediaData, filename, mime)
		if err != nil {
			job.addLog("WARN: media upload failed: " + err.Error())
		} else {
			mediaID = id
			job.addLog("Media uploaded OK. ID: " + mediaID)
		}
	}

	delayMin, delayMax := req.DelayMin, req.DelayMax
	if delayMin < 1 {
		delayMin = 5
	}
	if delayMax <= delayMin {
		delayMax = delayMin + 10
	}

	for i, contact := range req.Contacts {
		phone := normalizePhone(contact.Phone)
		text := substituteTemplate(req.Template, contact.Variables)

		sendErr := sendCloudAPIMessage(token, phoneID, phone, text, mediaID, req.MediaType)

		job.mu.Lock()
		name := contact.Variables["name"]
		if name == "" {
			name = phone
		}
		if sendErr != nil {
			job.Failed++
			job.Log = append(job.Log, fmt.Sprintf("[%s] [%d/%d] FAILED %s: %s",
				time.Now().Format("15:04:05"), i+1, job.Total, name, sendErr.Error()))
		} else {
			job.Sent++
			job.Log = append(job.Log, fmt.Sprintf("[%s] [%d/%d] Sent to %s",
				time.Now().Format("15:04:05"), i+1, job.Total, name))
		}
		job.mu.Unlock()

		if i < len(req.Contacts)-1 {
			d := delayMin + mrand.Float64()*(delayMax-delayMin)
			time.Sleep(time.Duration(d * float64(time.Second)))
		}
	}

	job.setStatus("done")
	job.addLog(fmt.Sprintf("Complete — Sent: %d, Failed: %d", job.Sent, job.Failed))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	initDB()
	fmt.Println("SQLite database initialized successfully.")

	mux := http.NewServeMux()

	// Public Routes
	mux.HandleFunc("/api/auth/register", registerHandler)
	mux.HandleFunc("/api/auth/login", loginHandler)

	// Protected Routes
	// Serve static files from Vite build
	fs := http.FileServer(http.Dir("../frontend/dist"))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat("../frontend/dist" + r.URL.Path); os.IsNotExist(err) && !strings.HasPrefix(r.URL.Path, "/api/") {
			http.ServeFile(w, r, "../frontend/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/api/settings", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			getSettingsHandler(w, r)
		} else if r.Method == http.MethodPost {
			saveSettingsHandler(w, r)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/bulk-send", authMiddleware(bulkSendHandler))
	mux.HandleFunc("/api/jobs", authMiddleware(jobsHandler))

	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler(mux)

	fmt.Println("SaaS Backend listening on http://localhost:8080")
	http.ListenAndServe(":8080", handler)
}
