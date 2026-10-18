package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const snapshot = "2026-10-17T07:30:00.000Z"

var channels = []string{"WhatsApp", "Instagram", "Facebook", "TikTok", "Website"}
var branches = []string{"Manado", "Palu", "Kendari", "Gorontalo"}
var reps = []string{"Nadia", "Raka", "Maya", "Fajar"}
var products = []string{"MPV Family", "SUV Urban", "Scooter 155"}
var stages = []string{"New", "Qualified", "Test drive", "Proposal", "Order", "Won", "Nurture", "Lost"}

type Message struct {
	ID        string `json:"id"`
	Direction string `json:"direction"`
	Text      string `json:"text"`
	At        string `json:"at"`
}
type Activity struct {
	Text string `json:"text"`
	At   string `json:"at"`
}
type Lead struct {
	ID               string     `json:"id"`
	CustomerID       string     `json:"customerId"`
	Name             string     `json:"name"`
	Channel          string     `json:"channel"`
	Branch           string     `json:"branch"`
	Owner            *string    `json:"owner"`
	Product          string     `json:"product"`
	Stage            string     `json:"stage"`
	Value            int64      `json:"value"`
	CreatedAt        string     `json:"createdAt"`
	FirstReplyMins   *int       `json:"firstReplyMins"`
	MarketingConsent bool       `json:"marketingConsent"`
	OptedOut         bool       `json:"optedOut"`
	Intent           string     `json:"intent"`
	Score            int        `json:"score"`
	Inquiries        int        `json:"inquiries"`
	Contact          string     `json:"contact"`
	ClosedAt         *string    `json:"closedAt"`
	NextAction       string     `json:"nextAction"`
	Messages         []Message  `json:"messages"`
	History          []Activity `json:"history"`
}
type Campaign struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}
type Health struct {
	ERP bool `json:"erp"`
}
type Rules struct {
	Routing    bool `json:"routing"`
	SLA        bool `json:"sla"`
	Aftersales bool `json:"aftersales"`
}
type State struct {
	Version        int        `json:"version"`
	Leads          []Lead     `json:"leads"`
	Activity       []Activity `json:"activity"`
	IngestedEvents []string   `json:"ingestedEvents"`
	Health         Health     `json:"health"`
	Rules          Rules      `json:"rules"`
	Campaigns      []Campaign `json:"campaigns"`
	LastSyncAt     string     `json:"lastSyncAt"`
}
type input struct {
	Name             string  `json:"name"`
	Branch           string  `json:"branch"`
	Channel          string  `json:"channel"`
	Product          string  `json:"product"`
	MarketingConsent bool    `json:"marketingConsent"`
	Owner            *string `json:"owner"`
	Stage            string  `json:"stage"`
	Text             string  `json:"text"`
	OptedOut         *bool   `json:"optedOut"`
	Healthy          *bool   `json:"healthy"`
	Key              string  `json:"key"`
	Enabled          *bool   `json:"enabled"`
	EventID          string  `json:"eventId"`
	LeadID           string  `json:"leadId"`
	Days             int     `json:"days"`
	Search           string  `json:"search"`
}

type repository interface {
	Read(context.Context) (State, error)
	Update(context.Context, func(*State) error) (State, error)
}
type mysqlRepository struct{ db *sql.DB }

func (r *mysqlRepository) Read(ctx context.Context) (State, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, "SELECT payload FROM demo_workspace WHERE id=1").Scan(&raw)
	var state State
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(raw, &state)
	return state, err
}
func (r *mysqlRepository) Update(ctx context.Context, fn func(*State) error) (State, error) {
	// shortcut: one assessment workspace uses one transactional snapshot. Normalize aggregates before multi-tenant production.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM demo_workspace WHERE id=1 FOR UPDATE").Scan(&raw); err != nil {
		return State{}, err
	}
	var state State
	if err = json.Unmarshal(raw, &state); err != nil {
		return State{}, err
	}
	if err = fn(&state); err != nil {
		return State{}, err
	}
	raw, err = json.Marshal(state)
	if err != nil {
		return State{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE demo_workspace SET payload=?, revision=revision+1, updated_at=CURRENT_TIMESTAMP(3) WHERE id=1", raw); err != nil {
		return State{}, err
	}
	return state, tx.Commit()
}

type memoryRepository struct {
	mu    sync.Mutex
	state State
}

func (r *memoryRepository) Read(context.Context) (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return clone(r.state), nil
}
func (r *memoryRepository) Update(_ context.Context, fn func(*State) error) (State, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := clone(r.state)
	if err := fn(&s); err != nil {
		return State{}, err
	}
	r.state = s
	return clone(s), nil
}
func clone(s State) State { b, _ := json.Marshal(s); var c State; _ = json.Unmarshal(b, &c); return c }
func audit(s *State, text string) {
	s.Activity = append([]Activity{{Text: text, At: snapshot}}, s.Activity...)
	if len(s.Activity) > 100 {
		s.Activity = s.Activity[:100]
	}
}
func find(s *State, id string) (*Lead, error) {
	for i := range s.Leads {
		if s.Leads[i].ID == id {
			return &s.Leads[i], nil
		}
	}
	return nil, errors.New("lead tidak ditemukan")
}
func eligibility(l Lead, healthy bool) bool {
	return healthy && !l.OptedOut && l.MarketingConsent && l.Stage != "Won" && l.Stage != "Order" && l.Stage != "Lost"
}
func newLead(s *State, in input) error {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 80 || !slices.Contains(channels, in.Channel) || !slices.Contains(branches, in.Branch) || !slices.Contains(products, in.Product) {
		return errors.New("nama, channel, cabang, atau produk tidak valid")
	}
	maxID := 1000
	for _, l := range s.Leads {
		n, _ := strconv.Atoi(strings.TrimPrefix(l.ID, "L-"))
		if n > maxID {
			maxID = n
		}
	}
	var owner *string
	if s.Rules.Routing {
		v := reps[slices.Index(branches, in.Branch)]
		owner = &v
	}
	id := fmt.Sprintf("L-%d", maxID+1)
	s.Leads = append(s.Leads, Lead{ID: id, CustomerID: fmt.Sprintf("C-demo-%d", maxID+1), Name: name, Channel: in.Channel, Branch: in.Branch, Owner: owner, Product: in.Product, Stage: "New", Value: 285000000, CreatedAt: snapshot, MarketingConsent: in.MarketingConsent, Intent: "Inquiry baru", Score: 60, Inquiries: 1, Contact: "Identitas kontak simulasi", NextAction: "Konfirmasi kebutuhan dan waktu dihubungi", Messages: []Message{{ID: id + "-in", Direction: "in", Text: "Saya tertarik " + in.Product + ". Mohon informasinya.", At: snapshot}}, History: []Activity{{Text: "Lead simulasi dibuat. SLA mulai dihitung.", At: snapshot}}})
	audit(s, id+": inquiry baru dari "+in.Channel+".")
	return nil
}
func change(s *State, path string, in input) error {
	if path == "/api/leads" {
		return newLead(s, in)
	}
	if path == "/api/events/inquiry" {
		if in.EventID == "" || len(in.EventID) > 100 {
			return errors.New("eventId wajib, maksimal 100 karakter")
		}
		if slices.Contains(s.IngestedEvents, in.EventID) {
			audit(s, in.EventID+": duplikat dilewati, lead tidak bertambah.")
			return nil
		}
		if err := newLead(s, input{Name: "Customer Event Demo", Branch: "Manado", Channel: "Website", Product: "MPV Family"}); err != nil {
			return err
		}
		s.IngestedEvents = append(s.IngestedEvents, in.EventID)
		audit(s, in.EventID+": event diterima sekali.")
		return nil
	}
	if path == "/api/erp/health" {
		if in.Healthy == nil {
			return errors.New("healthy wajib")
		}
		s.Health.ERP = *in.Healthy
		audit(s, fmt.Sprintf("ERP freshness simulasi: %t", *in.Healthy))
		return nil
	}
	if path == "/api/rules" {
		if in.Enabled == nil {
			return errors.New("enabled wajib")
		}
		switch in.Key {
		case "routing":
			s.Rules.Routing = *in.Enabled
		case "sla":
			s.Rules.SLA = *in.Enabled
		default:
			return errors.New("aturan tidak boleh diubah")
		}
		audit(s, "Rule "+in.Key+" diperbarui.")
		return nil
	}
	if path == "/api/campaigns" {
		ids := []string{}
		now, _ := time.Parse(time.RFC3339, snapshot)
		days := in.Days
		if days != 1 && days != 7 && days != 30 {
			days = 30
		}
		cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
		for _, l := range s.Leads {
			created, _ := time.Parse(time.RFC3339, l.CreatedAt)
			match := !created.Before(cutoff) && (in.Branch == "" || l.Branch == in.Branch) && (in.Channel == "" || l.Channel == in.Channel) && (in.Search == "" || strings.Contains(strings.ToLower(l.Name+" "+l.ID+" "+l.Product+" "+pointer(l.Owner)), strings.ToLower(in.Search)))
			if match && eligibility(l, s.Health.ERP) {
				ids = append(ids, l.ID)
			}
		}
		if len(ids) == 0 {
			return errors.New("tidak ada audience yang memenuhi syarat")
		}
		s.Campaigns = append([]Campaign{{ID: fmt.Sprintf("CMP-%d", len(s.Campaigns)+1), Name: "Follow-up minat produk · Oktober", IDs: ids, Status: "Draft"}}, s.Campaigns...)
		audit(s, fmt.Sprintf("Draft audience: %d lead. Tidak ada pengiriman nyata.", len(ids)))
		return nil
	}
	if strings.HasPrefix(path, "/api/campaigns/") && strings.HasSuffix(path, "/simulate") {
		parts := strings.Split(path, "/")
		if len(parts) != 5 {
			return errors.New("route tidak valid")
		}
		for i := range s.Campaigns {
			c := &s.Campaigns[i]
			if c.ID != parts[3] {
				continue
			}
			count := 0
			for _, id := range c.IDs {
				l, e := find(s, id)
				if e == nil && eligibility(*l, s.Health.ERP) {
					count++
				}
			}
			c.Status = fmt.Sprintf("Simulated: %d layak, %d ditahan", count, len(c.IDs)-count)
			audit(s, c.ID+": send-time check. Tidak ada pengiriman nyata.")
			return nil
		}
		return errors.New("campaign tidak ditemukan")
	}
	if !strings.HasPrefix(path, "/api/leads/") {
		return errors.New("route tidak ditemukan")
	}
	parts := strings.Split(path, "/")
	if len(parts) < 4 || len(parts) > 5 {
		return errors.New("route tidak valid")
	}
	l, err := find(s, parts[3])
	if err != nil {
		return err
	}
	action := "update"
	if len(parts) == 5 {
		action = parts[4]
	}
	switch action {
	case "update":
		if in.Owner != nil {
			if *in.Owner != "" && !slices.Contains(reps, *in.Owner) {
				return errors.New("owner tidak valid")
			}
			if *in.Owner == "" {
				l.Owner = nil
			} else {
				l.Owner = in.Owner
			}
		}
		if in.Stage != "" {
			if !slices.Contains(stages, in.Stage) || in.Stage == "Won" {
				return errors.New("status tidak valid; Won memerlukan event purchase")
			}
			if l.Stage == "Won" {
				return errors.New("opportunity Won tidak dapat dibuka ulang melalui demo")
			}
			l.Stage = in.Stage
		}
		audit(s, l.ID+": owner/status diperbarui.")
	case "consent":
		if in.OptedOut == nil {
			return errors.New("optedOut wajib")
		}
		l.OptedOut = *in.OptedOut
		if l.OptedOut {
			l.MarketingConsent = false
		}
		l.History = append(l.History, Activity{Text: "Consent/opt-out diperbarui (simulasi).", At: snapshot})
		audit(s, l.ID+": consent diperbarui.")
	case "purchase":
		if !s.Health.ERP {
			return errors.New("ERP tidak segar; purchase belum dapat diverifikasi")
		}
		if l.Stage == "Won" {
			return nil
		}
		l.Stage = "Won"
		v := snapshot
		l.ClosedAt = &v
		l.NextAction = "Konfirmasi jadwal serah terima"
		l.History = append(l.History, Activity{Text: "Simulasi order ERP terverifikasi. Prospecting dihentikan.", At: snapshot})
		audit(s, l.ID+": purchase confirmed. Prospecting suppressed.")
	case "messages":
		text := strings.TrimSpace(in.Text)
		if text == "" || len([]rune(text)) > 2000 {
			return errors.New("pesan harus 1–2000 karakter")
		}
		l.Messages = append(l.Messages, Message{ID: fmt.Sprintf("%s-msg-%d", l.ID, len(l.Messages)+1), Direction: "out", Text: text, At: snapshot})
		if l.FirstReplyMins == nil {
			created, _ := time.Parse(time.RFC3339, l.CreatedAt)
			now, _ := time.Parse(time.RFC3339, snapshot)
			mins := max(0, int(now.Sub(created).Minutes()))
			l.FirstReplyMins = &mins
		}
		audit(s, l.ID+": balasan simulasi dicatat, tidak dikirim ke channel.")
	default:
		return errors.New("route tidak ditemukan")
	}
	return nil
}
func pointer(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type app struct {
	repo        repository
	token       string
	origins     map[string]bool
	deepseekKey string
	deepseekURL string
	model       string
	client      *http.Client
	sales       salesSource
}

func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, text string) {
	respond(w, status, map[string]string{"error": text})
}
func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	origin := r.Header.Get("Origin")
	if origin != "" {
		if !a.origins[origin] {
			fail(w, 403, "origin tidak diizinkan")
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if _, err := a.repo.Read(ctx); err != nil {
			fail(w, 503, "MySQL unavailable")
			return
		}
		respond(w, 200, map[string]string{"status": "ok", "database": "mysql", "mode": "assessment-demo"})
		return
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if a.token == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(a.token)) != 1 {
		fail(w, 401, "demo access token diperlukan")
		return
	}
	if r.URL.Path == "/api/state" && r.Method == http.MethodGet {
		state, err := a.repo.Read(r.Context())
		if err != nil {
			fail(w, 503, "database tidak tersedia")
			return
		}
		respond(w, 200, state)
		return
	}
	if r.URL.Path == "/api/integrations" && r.Method == http.MethodGet {
		respond(w, 200, map[string]any{"ai": map[string]any{"provider": "deepseek", "configured": a.deepseekKey != "", "model": a.model}, "mssql": map[string]any{"configured": a.sales != nil}})
		return
	}
	if r.URL.Path == "/api/sales/dashboard" && r.Method == http.MethodGet {
		a.salesDashboard(w, r)
		return
	}
	if r.Method != http.MethodPost {
		fail(w, 405, "method tidak didukung")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var in input
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		fail(w, 400, "JSON request tidak valid")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		fail(w, 400, "request harus berisi satu objek JSON")
		return
	}
	if r.URL.Path == "/api/ai/draft" {
		a.draft(w, r, in)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	s, err := a.repo.Update(ctx, func(s *State) error { return change(s, r.URL.Path, in) })
	if err != nil {
		if strings.Contains(err.Error(), "tidak") || strings.Contains(err.Error(), "wajib") || strings.Contains(err.Error(), "harus") || strings.Contains(err.Error(), "memerlukan") || strings.Contains(err.Error(), "belum") {
			fail(w, 400, err.Error())
		} else {
			log.Printf("state update failed: %T", err)
			fail(w, 503, "database tidak tersedia")
		}
		return
	}
	respond(w, 200, s)
}
func (a *app) draft(w http.ResponseWriter, r *http.Request, in input) {
	s, err := a.repo.Read(r.Context())
	if err != nil {
		fail(w, 503, "database tidak tersedia")
		return
	}
	l, err := find(&s, in.LeadID)
	if err != nil {
		fail(w, 404, err.Error())
		return
	}
	if a.deepseekKey == "" {
		respond(w, 200, map[string]string{"mode": "local-template", "draft": "Selamat siang Bapak/Ibu, terima kasih atas minat Anda pada " + l.Product + ". Kapan waktu yang nyaman untuk berdiskusi dengan tim sales? Informasi harga dan ketersediaan akan kami konfirmasi."})
		return
	}
	// Only controlled product/intent labels leave the server. Names, contacts, raw chat and private memory never enter the prompt.
	system := "Anda adalah asisten draft sales untuk demo otomotif. Tulis satu balasan Bahasa Indonesia maksimal 100 kata yang siap ditinjau sales. Gunakan sapaan Bapak/Ibu. Jangan mengarang harga, diskon, stok, kredit, atau janji. Minta waktu yang nyaman untuk diskusi. Keluarkan hanya pesan untuk customer, tanpa catatan internal, penjelasan bahwa ini draf, atau instruksi review."
	payload := map[string]any{"model": a.model, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": "Produk sintetis: " + l.Product + ". Intent terkontrol: " + l.Intent}}, "max_tokens": 512, "stream": false, "thinking": map[string]string{"type": "disabled"}}
	b, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.deepseekURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		fail(w, 502, "konfigurasi DeepSeek tidak valid")
		return
	}
	req.Header.Set("Authorization", "Bearer "+a.deepseekKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		fail(w, 502, "DeepSeek belum merespons; coba kembali atau tulis manual")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		message := "DeepSeek menolak request; periksa konfigurasi server"
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			message = "API key DeepSeek ditolak; periksa key di backend"
		case http.StatusPaymentRequired:
			message = "Saldo DeepSeek tidak mencukupi; isi saldo akun provider"
		case http.StatusTooManyRequests:
			message = "Batas request DeepSeek tercapai; coba kembali nanti"
		}
		fail(w, 502, message)
		return
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result); err != nil || len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		fail(w, 502, "respons DeepSeek tidak valid")
		return
	}
	text := result.Choices[0].Message.Content
	if len([]rune(text)) > 2000 {
		fail(w, 502, "draft DeepSeek melebihi batas; tulis manual")
		return
	}
	respond(w, 200, map[string]string{"mode": "deepseek", "model": a.model, "draft": text})
}
func env(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
func main() {
	token := os.Getenv("DEMO_API_TOKEN")
	if len(token) < 24 {
		log.Fatal("DEMO_API_TOKEN must be at least 24 characters")
	}
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required; no in-memory production fallback")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("invalid MySQL configuration")
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		log.Fatal("MySQL connection failed; check local settings")
	}
	if _, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS demo_workspace (id TINYINT PRIMARY KEY, payload JSON NOT NULL, revision BIGINT UNSIGNED NOT NULL DEFAULT 1, updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3))"); err != nil {
		log.Fatal("MySQL demo schema setup failed")
	}
	seed, err := os.ReadFile(env("SEED_PATH", "seed.json"))
	if err != nil {
		log.Fatal("seed.json is required")
	}
	var initial State
	if json.Unmarshal(seed, &initial) != nil || initial.Version != 1 {
		log.Fatal("invalid seed data")
	}
	if _, err = db.ExecContext(ctx, "INSERT IGNORE INTO demo_workspace (id,payload) VALUES (1,?)", seed); err != nil {
		log.Fatal("MySQL seed failed")
	}
	origins := map[string]bool{}
	for _, v := range strings.Split(env("CORS_ORIGINS", "http://localhost:4173,http://127.0.0.1:4173"), ",") {
		origins[strings.TrimSpace(v)] = true
	}
	a := &app{repo: &mysqlRepository{db: db}, token: token, origins: origins, deepseekKey: os.Getenv("DEEPSEEK_API_KEY"), deepseekURL: "https://api.deepseek.com", model: env("DEEPSEEK_MODEL", "deepseek-flash"), client: &http.Client{Timeout: 25 * time.Second}}
	if dsn := os.Getenv("MSSQL_DSN"); dsn != "" {
		var salesDB *sql.DB
		salesDB, a.sales = openSales(dsn)
		defer salesDB.Close()
	}
	server := http.Server{Addr: env("LISTEN_ADDR", "127.0.0.1:8080"), Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	log.Printf("OneCRM assessment Go API listening on %s; persistent MySQL; MS SQL sales reporting configured: %t; no live channel/ERP integration", server.Addr, a.sales != nil)
	log.Fatal(server.ListenAndServe())
}
