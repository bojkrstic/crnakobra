package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type user struct {
	Email          string          `json:"email"`
	Entitlements   map[string]bool `json:"entitlements"`
	PersonalStatus string          `json:"personalStatus"`
}
type order struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Product   string    `json:"product"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}
type store struct {
	mu     sync.Mutex
	Users  map[string]*user  `json:"users"`
	Orders map[string]*order `json:"orders"`
}

var data = store{Users: map[string]*user{}, Orders: map[string]*order{}}
var secret = env("DEMO_SECRET", "local-demo-secret-change-before-production")
var frontend = env("FRONTEND_URL", "http://localhost:8080")
var listenAddr = env("LISTEN_ADDR", "127.0.0.1:8080")
var demoMode = os.Getenv("DEMO_MODE") == "true"

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func randomID(prefix string) string {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}
func sign(value string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
func token(email string) string        { return tokenFor(email, 15*time.Minute) }
func sessionToken(email string) string { return tokenFor(email, 7*24*time.Hour) }
func tokenFor(email string, duration time.Duration) string {
	expiry := time.Now().Add(duration).Unix()
	value := fmt.Sprintf("%s|%d", email, expiry)
	return base64.RawURLEncoding.EncodeToString([]byte(value + "|" + sign(value)))
}
func verifyToken(raw string) (string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", false
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 || !hmac.Equal([]byte(parts[2]), []byte(sign(parts[0]+"|"+parts[1]))) {
		return "", false
	}
	var expiry int64
	if _, err = fmt.Sscan(parts[1], &expiry); err != nil || time.Now().Unix() > expiry {
		return "", false
	}
	return strings.ToLower(parts[0]), true
}
func currentUser(r *http.Request) (*user, bool) {
	email, ok := verifyTokenCookie(r)
	if !ok {
		return nil, false
	}
	data.mu.Lock()
	defer data.mu.Unlock()
	return data.Users[email], data.Users[email] != nil
}
func verifyTokenCookie(r *http.Request) (string, bool) {
	c, err := r.Cookie("ck_session")
	if err != nil {
		return "", false
	}
	return verifyToken(c.Value)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/request-link", requestLink)
	mux.HandleFunc("/auth/magic", magicLink)
	mux.HandleFunc("/api/me", me)
	mux.HandleFunc("/api/orders", createOrder)
	mux.HandleFunc("/api/demo/checkout", demoCheckout)
	mux.HandleFunc("/webhooks/demo-payment", demoWebhook)
	mux.HandleFunc("/api/plans/monthly.pdf", monthlyPDF)
	mux.Handle("/", http.FileServer(http.Dir("..")))
	log.Printf("Crna Kobra payment API listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, logging(mux)))
}

func requestLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !strings.Contains(body.Email, "@") {
		http.Error(w, "valid email is required", 400)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	data.mu.Lock()
	if data.Users[email] == nil {
		data.Users[email] = &user{Email: email, Entitlements: map[string]bool{}}
	}
	data.mu.Unlock()
	jsonResponse(w, 200, map[string]string{"magicLink": frontend + "/auth/magic?token=" + token(email), "message": "Demo mode: open the link below instead of receiving an email."})
}
func magicLink(w http.ResponseWriter, r *http.Request) {
	email, ok := verifyToken(r.URL.Query().Get("token"))
	if !ok {
		http.Error(w, "invalid or expired magic link", 401)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ck_session", Value: sessionToken(email), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 60 * 60 * 24 * 7})
	http.Redirect(w, r, "/demo-portal.html", http.StatusFound)
}
func me(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	u, ok := currentUser(r)
	if !ok {
		http.Error(w, "sign in required", 401)
		return
	}
	jsonResponse(w, 200, u)
}
func createOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	u, ok := currentUser(r)
	if !ok {
		http.Error(w, "sign in required", 401)
		return
	}
	var body struct {
		Product string `json:"product"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Product != "monthly" && body.Product != "personalized") {
		http.Error(w, "invalid product", 400)
		return
	}
	o := &order{ID: randomID("CK-"), Email: u.Email, Product: body.Product, Status: "pending", CreatedAt: time.Now()}
	data.mu.Lock()
	data.Orders[o.ID] = o
	data.mu.Unlock()
	jsonResponse(w, 201, o)
}
func demoCheckout(w http.ResponseWriter, r *http.Request) {
	if !demoMode {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	u, ok := currentUser(r)
	if !ok {
		http.Error(w, "sign in required", 401)
		return
	}
	var body struct {
		OrderID string `json:"orderId"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	data.mu.Lock()
	o := data.Orders[body.OrderID]
	data.mu.Unlock()
	if o == nil || o.Email != u.Email {
		http.Error(w, "order not found", 404)
		return
	}
	payload := o.ID + "|paid"
	r2, _ := http.NewRequest("POST", "/webhooks/demo-payment", strings.NewReader(`{"orderId":"`+o.ID+`","status":"paid"}`))
	r2.Header.Set("X-Demo-Signature", sign(payload))
	rr := &responseRecorder{header: http.Header{}}
	demoWebhook(rr, r2)
	if rr.status >= 400 {
		http.Error(w, "demo payment failed", 500)
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "paid", "orderId": o.ID})
}
func demoWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		OrderID string `json:"orderId"`
		Status  string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.Status != "paid" || !hmac.Equal([]byte(r.Header.Get("X-Demo-Signature")), []byte(sign(body.OrderID+"|"+body.Status))) {
		http.Error(w, "invalid webhook", 401)
		return
	}
	data.mu.Lock()
	defer data.mu.Unlock()
	o := data.Orders[body.OrderID]
	if o == nil {
		http.Error(w, "order not found", 404)
		return
	}
	o.Status = "paid"
	u := data.Users[o.Email]
	if o.Product == "monthly" {
		u.Entitlements["monthly"] = true
	} else {
		u.PersonalStatus = "Uplata potvrđena — popunite upitnik."
	}
	jsonResponse(w, 200, map[string]string{"status": "accepted"})
}
func monthlyPDF(w http.ResponseWriter, r *http.Request) {
	u, ok := currentUser(r)
	if !ok || !u.Entitlements["monthly"] {
		http.Error(w, "payment required", 403)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=crna-kobra-mesecni-plan.pdf")
	w.Write(buildPDF())
}
func buildPDF() []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>", "<< /Length 250 >>\nstream\nBT /F1 20 Tf 55 780 Td (Crna Kobra - Mesecni plan) Tj /F1 12 Tf 0 -42 Td (Primer gotovog plana za vise korisnika.) Tj 0 -28 Td (Nedelja 1: redovni obroci, voda i setnja.) Tj 0 -22 Td (Nedelja 2: povrce uz svaki glavni obrok.) Tj 0 -22 Td (Nedelja 3: planiranje namirnica.) Tj 0 -22 Td (Nedelja 4: pracenje navika i prilagodjavanje.) Tj 0 -45 Td (Ovo je demo PDF za testiranje pristupa.) Tj ET\nendstream", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	start := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer << /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, start)
	return []byte(b.String())
}

type responseRecorder struct {
	header http.Header
	status int
}

func (r *responseRecorder) Header() http.Header { return r.header }
func (r *responseRecorder) WriteHeader(s int)   { r.status = s }
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = 200
	}
	return len(b), nil
}
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}
