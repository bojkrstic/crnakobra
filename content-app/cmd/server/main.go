package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "ck_content_session"

type app struct {
	db             *pgxpool.Pool
	adminEmail     string
	adminPassword  string
	loginTemplate  *template.Template
	adminTemplate  *template.Template
	formTemplate   *template.Template
	publicTemplate *template.Template
}

type item struct {
	ID          int64
	Kind        string
	Title       string
	Slug        string
	Summary     string
	Body        string
	ImageURL    string
	HasImage    bool
	ShowOnCamps bool
	CampID      int64
	CampTitle   string
	EventDate   *time.Time
	Location    string
	Published   bool
	CreatedAt   time.Time
}

func main() {
	databaseURL := mustEnv("DATABASE_URL")
	a := &app{adminEmail: strings.ToLower(mustEnv("ADMIN_EMAIL")), adminPassword: mustEnv("ADMIN_PASSWORD")}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	a.db = pool
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}
	if err := a.ensureSchema(ctx); err != nil {
		log.Fatalf("prepare schema: %v", err)
	}
	if err := a.bootstrapAdmin(ctx); err != nil {
		log.Fatalf("prepare administrator: %v", err)
	}
	a.parseTemplates()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /", a.home)
	mux.HandleFunc("GET /vesti", a.publicList("news", "Vesti"))
	mux.HandleFunc("GET /kampovi", a.publicList("camp", "Kampovi i manifestacije"))
	mux.HandleFunc("GET /galerija", a.publicList("gallery", "Galerija"))
	mux.HandleFunc("GET /dynamic/camp-gallery", a.campGalleryFragment)
	mux.HandleFunc("GET /dynamic/camps", a.campsFragment)
	mux.HandleFunc("GET /media/{id}", a.media)
	mux.HandleFunc("GET /dynamic/media/{id}", a.media)
	mux.HandleFunc("GET /admin", a.admin)
	mux.HandleFunc("POST /admin/login", a.login)
	mux.HandleFunc("POST /admin/logout", a.logout)
	mux.HandleFunc("GET /admin/new", a.editForm)
	mux.HandleFunc("GET /admin/edit/{id}", a.editForm)
	mux.HandleFunc("POST /admin/items", a.saveItem)
	mux.HandleFunc("POST /admin/items/{id}/delete", a.deleteItem)

	addr := env("LISTEN_ADDR", ":8090")
	log.Printf("content app listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, securityHeaders(mux)))
}

func (a *app) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (email TEXT PRIMARY KEY, password_hash TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (token_hash TEXT PRIMARY KEY, email TEXT NOT NULL REFERENCES admin_users(email) ON DELETE CASCADE, expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`ALTER TABLE content_items ADD COLUMN IF NOT EXISTS image_data BYTEA`,
		`ALTER TABLE content_items ADD COLUMN IF NOT EXISTS image_mime TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE content_items ADD COLUMN IF NOT EXISTS show_on_camps BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE content_items ADD COLUMN IF NOT EXISTS camp_id BIGINT REFERENCES content_items(id) ON DELETE SET NULL`,
	}
	for _, statement := range statements {
		if _, err := a.db.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) bootstrapAdmin(ctx context.Context) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(a.adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(ctx, `INSERT INTO admin_users (email, password_hash) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`, a.adminEmail, string(hash))
	return err
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	if err := a.db.Ping(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "ok")
}

func (a *app) home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintln(w, `<!doctype html><title>Crna Kobra sadržaj</title><p>Dinamički sadržaj je aktivan.</p><ul><li><a href="/vesti">Vesti</a></li><li><a href="/kampovi">Kampovi</a></li><li><a href="/galerija">Galerija</a></li><li><a href="/admin">Administracija</a></li></ul>`)
}

func (a *app) admin(w http.ResponseWriter, r *http.Request) {
	if !a.authenticated(r) {
		a.render(w, a.loginTemplate, map[string]any{"Error": r.URL.Query().Get("error") == "1"})
		return
	}
	items, err := a.items(r.Context(), "")
	if err != nil {
		http.Error(w, "cannot load content", 500)
		return
	}
	a.render(w, a.adminTemplate, map[string]any{"Items": items})
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=1", 303)
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	var hash string
	err := a.db.QueryRow(r.Context(), `SELECT password_hash FROM admin_users WHERE email = $1`, email).Scan(&hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		http.Redirect(w, r, "/admin?error=1", 303)
		return
	}
	raw, err := randomToken()
	if err != nil {
		http.Error(w, "cannot create session", 500)
		return
	}
	if _, err := a.db.Exec(r.Context(), `DELETE FROM admin_sessions WHERE expires_at < NOW()`); err != nil {
		log.Printf("clean sessions: %v", err)
	}
	if _, err := a.db.Exec(r.Context(), `INSERT INTO admin_sessions (token_hash, email, expires_at) VALUES ($1, $2, $3)`, tokenHash(raw), email, time.Now().Add(12*time.Hour)); err != nil {
		http.Error(w, "cannot create session", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: raw, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 12 * 60 * 60})
	http.Redirect(w, r, "/admin", 303)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_, _ = a.db.Exec(r.Context(), `DELETE FROM admin_sessions WHERE token_hash = $1`, tokenHash(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	http.Redirect(w, r, "/admin", 303)
}

func (a *app) editForm(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	entry := item{Kind: "news"}
	if id := r.PathValue("id"); id != "" {
		var err error
		entry, err = a.itemByID(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	camps, err := a.campsForAdmin(r.Context())
	if err != nil {
		http.Error(w, "cannot load camps", 500)
		return
	}
	a.render(w, a.formTemplate, map[string]any{"Item": entry, "Date": dateValue(entry.EventDate), "Camps": camps})
}

func (a *app) saveItem(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	if err := r.ParseMultipartForm(9 << 20); err != nil {
		http.Error(w, "slika je prevelika", 400)
		return
	}
	entry, err := itemFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var image []byte
	var mime string
	file, header, uploadErr := r.FormFile("image")
	if uploadErr == nil {
		defer file.Close()
		if header.Size > 8<<20 {
			http.Error(w, "slika može imati najviše 8 MB", 400)
			return
		}
		image, err = io.ReadAll(io.LimitReader(file, 8<<20+1))
		if err != nil || len(image) > 8<<20 {
			http.Error(w, "slika nije validna", 400)
			return
		}
		mime = http.DetectContentType(image)
		if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
			http.Error(w, "dozvoljeni su JPEG, PNG i WebP", 400)
			return
		}
	}
	if entry.ID == 0 {
		var id int64
		err = a.db.QueryRow(r.Context(), `INSERT INTO content_items (kind,title,slug,summary,body,image_url,image_data,image_mime,event_date,location,show_on_camps,published,published_at) VALUES ($1,$2,$3,$4,$5,'',$6,$7,$8,$9,$10,$11,CASE WHEN $11 THEN NOW() END) RETURNING id`, entry.Kind, entry.Title, entry.Slug, entry.Summary, entry.Body, image, mime, entry.EventDate, entry.Location, entry.ShowOnCamps, entry.Published).Scan(&id)
		entry.ID = id
	} else if len(image) > 0 {
		_, err = a.db.Exec(r.Context(), `UPDATE content_items SET kind=$1,title=$2,slug=$3,summary=$4,body=$5,image_data=$6,image_mime=$7,event_date=$8,location=$9,show_on_camps=$10,published=$11,published_at=CASE WHEN $11 AND published_at IS NULL THEN NOW() WHEN NOT $11 THEN NULL ELSE published_at END,updated_at=NOW() WHERE id=$12`, entry.Kind, entry.Title, entry.Slug, entry.Summary, entry.Body, image, mime, entry.EventDate, entry.Location, entry.ShowOnCamps, entry.Published, entry.ID)
	} else {
		_, err = a.db.Exec(r.Context(), `UPDATE content_items SET kind=$1,title=$2,slug=$3,summary=$4,body=$5,event_date=$6,location=$7,show_on_camps=$8,published=$9,published_at=CASE WHEN $9 AND published_at IS NULL THEN NOW() WHEN NOT $9 THEN NULL ELSE published_at END,updated_at=NOW() WHERE id=$10`, entry.Kind, entry.Title, entry.Slug, entry.Summary, entry.Body, entry.EventDate, entry.Location, entry.ShowOnCamps, entry.Published, entry.ID)
	}
	if err != nil {
		http.Error(w, "ne mogu da sačuvam sadržaj (naslov i URL oznaka moraju biti jedinstveni)", 400)
		return
	}
	if entry.Kind == "gallery" {
		var campID any
		if entry.CampID > 0 {
			campID = entry.CampID
		}
		if _, err := a.db.Exec(r.Context(), `UPDATE content_items SET camp_id=$1 WHERE id=$2`, campID, entry.ID); err != nil {
			http.Error(w, "ne mogu da povežem sliku sa kampom", 500)
			return
		}
	}
	http.Redirect(w, r, "/admin", 303)
}

func (a *app) deleteItem(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	_, err := a.db.Exec(r.Context(), `DELETE FROM content_items WHERE id = $1`, r.PathValue("id"))
	if err != nil {
		http.Error(w, "ne mogu da obrišem sadržaj", 500)
		return
	}
	http.Redirect(w, r, "/admin", 303)
}

func (a *app) publicList(kind, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := a.items(r.Context(), kind)
		if err != nil {
			http.Error(w, "cannot load content", 500)
			return
		}
		a.render(w, a.publicTemplate, map[string]any{"Items": items, "Title": title})
	}
}

func (a *app) media(w http.ResponseWriter, r *http.Request) {
	var data []byte
	var mime string
	err := a.db.QueryRow(r.Context(), `SELECT image_data, image_mime FROM content_items WHERE id=$1 AND image_data IS NOT NULL`, r.PathValue("id")).Scan(&data, &mime)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func (a *app) campGalleryFragment(w http.ResponseWriter, r *http.Request) {
	items, err := a.campGalleryItems(r.Context())
	if err != nil {
		http.Error(w, "cannot load gallery", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(items) == 0 {
		fmt.Fprint(w, `<p class="dynamic-gallery-empty">Fotografije sa kampova stižu uskoro.</p>`)
		return
	}
	fmt.Fprint(w, `<div class="dynamic-gallery-grid">`)
	for _, entry := range items {
		fmt.Fprintf(w, `<figure class="dynamic-gallery-item"><img data-media-id="%d" alt="%s" loading="lazy">`, entry.ID, template.HTMLEscapeString(entry.Title))
		if entry.Title != "" || entry.CampTitle != "" {
			caption := entry.Title
			if entry.CampTitle != "" {
				caption += ` · ` + entry.CampTitle
			}
			fmt.Fprintf(w, `<figcaption>%s</figcaption>`, template.HTMLEscapeString(caption))
		}
		fmt.Fprint(w, `</figure>`)
	}
	fmt.Fprint(w, `</div>`)
}

func (a *app) campsFragment(w http.ResponseWriter, r *http.Request) {
	items, err := a.items(r.Context(), "camp")
	if err != nil {
		http.Error(w, "cannot load camps", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if len(items) == 0 {
		fmt.Fprint(w, `<p class="dynamic-camps-empty">Novi termini kampova biće objavljeni uskoro.</p>`)
		return
	}
	fmt.Fprint(w, `<div class="dynamic-camps-grid">`)
	for _, entry := range items {
		fmt.Fprint(w, `<article class="dynamic-camp-card">`)
		if entry.HasImage {
			fmt.Fprintf(w, `<img data-media-id="%d" alt="%s" loading="lazy">`, entry.ID, template.HTMLEscapeString(entry.Title))
		}
		fmt.Fprint(w, `<div class="dynamic-camp-copy">`)
		if entry.EventDate != nil || entry.Location != "" {
			fmt.Fprint(w, `<p class="dynamic-camp-meta">`)
			if entry.EventDate != nil {
				fmt.Fprint(w, entry.EventDate.Format("02.01.2006."))
			}
			if entry.EventDate != nil && entry.Location != "" {
				fmt.Fprint(w, ` · `)
			}
			if entry.Location != "" {
				fmt.Fprint(w, template.HTMLEscapeString(entry.Location))
			}
			fmt.Fprint(w, `</p>`)
		}
		fmt.Fprintf(w, `<h3>%s</h3>`, template.HTMLEscapeString(entry.Title))
		if entry.Summary != "" {
			fmt.Fprintf(w, `<p>%s</p>`, template.HTMLEscapeString(entry.Summary))
		}
		if entry.Body != "" {
			fmt.Fprintf(w, `<p class="dynamic-camp-body">%s</p>`, template.HTMLEscapeString(entry.Body))
		}
		fmt.Fprint(w, `</div></article>`)
	}
	fmt.Fprint(w, `</div>`)
}

func (a *app) items(ctx context.Context, kind string) ([]item, error) {
	query := `SELECT id,kind,title,slug,summary,body,image_url,image_data IS NOT NULL,show_on_camps,event_date,location,published,created_at FROM content_items WHERE published=TRUE`
	args := []any{}
	if kind != "" {
		query += ` AND kind=$1`
		args = append(args, kind)
	}
	query += ` ORDER BY COALESCE(published_at, created_at) DESC`
	if kind == "" {
		query = `SELECT id,kind,title,slug,summary,body,image_url,image_data IS NOT NULL,show_on_camps,event_date,location,published,created_at FROM content_items ORDER BY created_at DESC`
	}
	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []item
	for rows.Next() {
		var entry item
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.Title, &entry.Slug, &entry.Summary, &entry.Body, &entry.ImageURL, &entry.HasImage, &entry.ShowOnCamps, &entry.EventDate, &entry.Location, &entry.Published, &entry.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (a *app) campGalleryItems(ctx context.Context) ([]item, error) {
	rows, err := a.db.Query(ctx, `SELECT i.id,i.kind,i.title,i.slug,i.summary,i.body,i.image_url,i.image_data IS NOT NULL,i.show_on_camps,i.event_date,i.location,i.published,i.created_at,c.title FROM content_items i JOIN content_items c ON c.id=i.camp_id WHERE i.kind='gallery' AND i.published=TRUE AND i.image_data IS NOT NULL ORDER BY COALESCE(i.published_at, i.created_at) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []item
	for rows.Next() {
		var entry item
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.Title, &entry.Slug, &entry.Summary, &entry.Body, &entry.ImageURL, &entry.HasImage, &entry.ShowOnCamps, &entry.EventDate, &entry.Location, &entry.Published, &entry.CreatedAt, &entry.CampTitle); err != nil {
			return nil, err
		}
		results = append(results, entry)
	}
	return results, rows.Err()
}

func (a *app) campsForAdmin(ctx context.Context) ([]item, error) {
	rows, err := a.db.Query(ctx, `SELECT id, title FROM content_items WHERE kind='camp' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var camps []item
	for rows.Next() {
		var camp item
		if err := rows.Scan(&camp.ID, &camp.Title); err != nil {
			return nil, err
		}
		camps = append(camps, camp)
	}
	return camps, rows.Err()
}

func (a *app) itemByID(ctx context.Context, rawID string) (item, error) {
	var entry item
	err := a.db.QueryRow(ctx, `SELECT id,kind,title,slug,summary,body,image_url,image_data IS NOT NULL,show_on_camps,COALESCE(camp_id,0),event_date,location,published,created_at FROM content_items WHERE id=$1`, rawID).Scan(&entry.ID, &entry.Kind, &entry.Title, &entry.Slug, &entry.Summary, &entry.Body, &entry.ImageURL, &entry.HasImage, &entry.ShowOnCamps, &entry.CampID, &entry.EventDate, &entry.Location, &entry.Published, &entry.CreatedAt)
	return entry, err
}

func itemFromRequest(r *http.Request) (item, error) {
	entry := item{ID: parseID(r.FormValue("id")), Kind: r.FormValue("kind"), Title: strings.TrimSpace(r.FormValue("title")), Slug: slugify(r.FormValue("slug")), Summary: strings.TrimSpace(r.FormValue("summary")), Body: strings.TrimSpace(r.FormValue("body")), Location: strings.TrimSpace(r.FormValue("location")), ShowOnCamps: r.FormValue("show_on_camps") == "on", CampID: parseID(r.FormValue("camp_id")), Published: r.FormValue("published") == "on"}
	if entry.Kind != "news" && entry.Kind != "camp" && entry.Kind != "gallery" {
		return entry, fmt.Errorf("neispravna vrsta sadržaja")
	}
	if entry.Title == "" {
		return entry, fmt.Errorf("naslov je obavezan")
	}
	if entry.Slug == "" {
		entry.Slug = slugify(entry.Title)
	}
	if rawDate := r.FormValue("event_date"); rawDate != "" {
		date, err := time.Parse("2006-01-02", rawDate)
		if err != nil {
			return entry, fmt.Errorf("datum nije ispravan")
		}
		entry.EventDate = &date
	}
	return entry, nil
}

func (a *app) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	var exists bool
	err = a.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=$1 AND expires_at > NOW())`, tokenHash(cookie.Value)).Scan(&exists)
	return err == nil && exists
}
func (a *app) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if a.authenticated(r) {
		return true
	}
	http.Redirect(w, r, "/admin", 303)
	return false
}
func (a *app) render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("render: %v", err)
	}
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}
func tokenHash(raw string) string {
	value := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(value[:])
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func parseID(value string) int64 { id, _ := strconv.ParseInt(value, 10, 64); return id }
func dateValue(date *time.Time) string {
	if date == nil {
		return ""
	}
	return date.Format("2006-01-02")
}
func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacements := strings.NewReplacer("š", "s", "đ", "dj", "č", "c", "ć", "c", "ž", "z", " ", "-", "_", "-", "/", "-")
	value = replacements.Replace(value)
	return strings.Trim(value, "-")
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/dynamic/") || strings.HasPrefix(r.URL.Path, "/media/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *app) parseTemplates() {
	base := `<style>body{margin:0;background:#f0ede5;color:#151512;font:16px/1.55 Arial,sans-serif}main{max-width:980px;margin:auto;padding:48px 22px}h1,h2{font-family:Georgia,serif}a{color:#a2221b}.top{background:#151512;color:#fff;padding:15px 22px}.top a{color:#fff;margin-right:16px}.box,.card{background:#fff;padding:28px;margin:20px 0;box-shadow:0 8px 22px #0001}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:18px}.card img{width:100%;max-height:230px;object-fit:cover}.button,button{background:#cf2f24;color:#fff;border:0;padding:12px 16px;font-weight:bold;cursor:pointer;text-decoration:none;display:inline-block}input,select,textarea{width:100%;box-sizing:border-box;padding:10px;margin:5px 0 14px}textarea{min-height:140px}.muted{color:#666}.error{color:#a2221b;font-weight:bold}.actions{display:flex;gap:8px;align-items:center}.danger{background:#222}</style><header class="top"><a href="/">Crna Kobra sadržaj</a><a href="/vesti">Vesti</a><a href="/kampovi">Kampovi</a><a href="/galerija">Galerija</a><a href="/admin">Admin</a></header>`
	a.loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>` + base + `<main><div class="box"><h1>Administracija</h1><p>Prijavite se da uređujete sadržaj.</p>{{if .Error}}<p class="error">E-mail ili lozinka nisu ispravni.</p>{{end}}<form method="post" action="/admin/login"><label>E-mail<input type="email" name="email" required></label><label>Lozinka<input type="password" name="password" required></label><button>Prijavi se</button></form></div></main>`))
	a.adminTemplate = template.Must(template.New("admin").Parse(`<!doctype html>` + base + `<main><div class="actions"><h1>Upravljanje sadržajem</h1><a class="button" href="/admin/new">Dodaj sadržaj</a><form method="post" action="/admin/logout"><button class="danger">Odjava</button></form></div><p class="muted">Objavljene stavke vide se na javnim razvojnim rutama.</p>{{range .Items}}<article class="box"><div class="actions"><div><strong>{{.Title}}</strong> · {{.Kind}} {{if .Published}}· objavljeno{{else}}· nacrt{{end}}<br><span class="muted">{{.CreatedAt.Format "02.01.2006."}}</span></div><a class="button" href="/admin/edit/{{.ID}}">Izmeni</a><form method="post" action="/admin/items/{{.ID}}/delete" onsubmit="return confirm('Obrisati stavku?')"><button class="danger">Obriši</button></form></div></article>{{else}}<div class="box">Još nema sadržaja.</div>{{end}}</main>`))
	a.formTemplate = template.Must(template.New("form").Parse(`<!doctype html>` + base + `<main><div class="box"><h1>{{if .Item.ID}}Izmeni sadržaj{{else}}Dodaj sadržaj{{end}}</h1><form method="post" action="/admin/items" enctype="multipart/form-data"><input type="hidden" name="id" value="{{.Item.ID}}"><label>Vrsta<select name="kind"><option value="news" {{if eq .Item.Kind "news"}}selected{{end}}>Vest</option><option value="camp" {{if eq .Item.Kind "camp"}}selected{{end}}>Kamp / manifestacija</option><option value="gallery" {{if eq .Item.Kind "gallery"}}selected{{end}}>Galerija</option></select></label><label>Naslov<input name="title" value="{{.Item.Title}}" required></label><label>URL oznaka (opciono)<input name="slug" value="{{.Item.Slug}}"><span class="muted">Ako ostavite prazno, napraviće se iz naslova.</span></label><label>Kratak opis<textarea name="summary">{{.Item.Summary}}</textarea></label><label>Tekst<textarea name="body">{{.Item.Body}}</textarea></label><label>Datum događaja (za kamp)<input type="date" name="event_date" value="{{.Date}}"></label><label>Mesto<input name="location" value="{{.Item.Location}}"></label><label>Slika (JPEG, PNG ili WebP; najviše 8 MB)<input type="file" name="image" accept="image/jpeg,image/png,image/webp"></label>{{if .Item.ID}}<p class="muted">Ostavite sliku praznu ako je ne menjate.</p>{{end}}<label><input style="width:auto" type="checkbox" name="show_on_camps" {{if .Item.ShowOnCamps}}checked{{end}}> Prikaži sliku u sekciji Kampovi i manifestacije</label><label><input style="width:auto" type="checkbox" name="published" {{if .Item.Published}}checked{{end}}> Objavi odmah</label><p><button>Sačuvaj</button> <a href="/admin">Otkaži</a></p></form></div></main>`))
	// Override forme: galerijska slika se vezuje za konkretan kamp.
	a.formTemplate = template.Must(template.New("form").Parse(`<!doctype html>` + base + `<main><div class="box"><h1>{{if .Item.ID}}Izmeni sadržaj{{else}}Dodaj sadržaj{{end}}</h1><form method="post" action="/admin/items" enctype="multipart/form-data"><input type="hidden" name="id" value="{{.Item.ID}}"><label>Vrsta<select name="kind"><option value="news" {{if eq .Item.Kind "news"}}selected{{end}}>Vest</option><option value="camp" {{if eq .Item.Kind "camp"}}selected{{end}}>Kamp / manifestacija</option><option value="gallery" {{if eq .Item.Kind "gallery"}}selected{{end}}>Fotografija kampa</option></select></label><label>Naslov<input name="title" value="{{.Item.Title}}" required></label><label>Kratak opis<textarea name="summary">{{.Item.Summary}}</textarea></label><label>Tekst<textarea name="body">{{.Item.Body}}</textarea></label><label>Datum događaja (za kamp)<input type="date" name="event_date" value="{{.Date}}"></label><label>Mesto<input name="location" value="{{.Item.Location}}"></label><label>Kamp kome pripada fotografija<select name="camp_id"><option value="">— izaberite kamp —</option>{{range .Camps}}<option value="{{.ID}}" {{if eq .ID $.Item.CampID}}selected{{end}}>{{.Title}}</option>{{end}}</select></label><label>Slika (JPEG, PNG ili WebP; najviše 8 MB)<input type="file" name="image" accept="image/jpeg,image/png,image/webp"></label><label><input style="width:auto" type="checkbox" name="published" {{if .Item.Published}}checked{{end}}> Objavi odmah</label><p><button>Sačuvaj</button> <a href="/admin">Otkaži</a></p></form></div></main>`))
	a.publicTemplate = template.Must(template.New("public").Parse(`<!doctype html>` + base + `<main><h1>{{.Title}}</h1><div class="grid">{{range .Items}}<article class="card">{{if .HasImage}}<img src="/media/{{.ID}}" alt="{{.Title}}">{{end}}<p class="muted">{{.Kind}}{{if .EventDate}} · {{.EventDate.Format "02.01.2006."}}{{end}}{{if .Location}} · {{.Location}}{{end}}</p><h2>{{.Title}}</h2><p>{{.Summary}}</p><p style="white-space:pre-line">{{.Body}}</p></article>{{else}}<p>Trenutno nema objavljenog sadržaja.</p>{{end}}</div></main>`))
}
