package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func seeded(t *testing.T) State {
	t.Helper()
	b, err := os.ReadFile("seed.json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func testApp(t *testing.T) *app {
	return &app{repo: &memoryRepository{state: seeded(t)}, token: "test-token-only-32-characters-long", origins: map[string]bool{"http://localhost:4173": true}, client: &http.Client{Timeout: time.Second}, model: "deepseek-flash"}
}
func request(a *app, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestReplayDoesNotDuplicateLead(t *testing.T) {
	a := testApp(t)
	for range 2 {
		w := request(a, "POST", "/api/events/inquiry", `{"eventId":"event-test-1"}`, a.token)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	s, _ := a.repo.Read(context.Background())
	if len(s.Leads) != 37 || len(s.IngestedEvents) != 1 {
		t.Fatalf("duplicate event created extra records: %d", len(s.Leads))
	}
}
func TestCampaignRechecksPurchaseConsentAndERP(t *testing.T) {
	a := testApp(t)
	w := request(a, "POST", "/api/campaigns", `{"days":30}`, a.token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	s, _ := a.repo.Read(context.Background())
	before := len(s.Campaigns[0].IDs)
	if before < 2 {
		t.Fatal("expected eligible audience")
	}
	id := s.Campaigns[0].IDs[0]
	other := s.Campaigns[0].IDs[1]
	for _, r := range []struct{ path, body string }{{"/api/leads/" + id + "/purchase", `{}`}, {"/api/leads/" + other + "/consent", `{"optedOut":true}`}, {"/api/campaigns/CMP-1/simulate", `{}`}} {
		w = request(a, "POST", r.path, r.body, a.token)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	s, _ = a.repo.Read(context.Background())
	want := "Simulated: " + strconvTest(before-2) + " layak, 2 ditahan"
	if s.Campaigns[0].Status != want {
		t.Fatalf("send-time purchase/consent suppression: %s", s.Campaigns[0].Status)
	}
	w = request(a, "POST", "/api/erp/health", `{"healthy":false}`, a.token)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request(a, "POST", "/api/campaigns/CMP-1/simulate", `{}`, a.token)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	s, _ = a.repo.Read(context.Background())
	if !strings.HasPrefix(s.Campaigns[0].Status, "Simulated: 0 layak") {
		t.Fatal("stale ERP must fail closed")
	}
}
func strconvTest(n int) string { b, _ := json.Marshal(n); return string(b) }
func TestPurchaseAndConsentTransitions(t *testing.T) {
	a := testApp(t)
	if request(a, "POST", "/api/leads/L-1002", `{"stage":"Won"}`, a.token).Code != 400 {
		t.Fatal("Won must require verified purchase path")
	}
	if request(a, "POST", "/api/leads/L-1002/purchase", `{}`, a.token).Code != 200 {
		t.Fatal("purchase failed")
	}
	if request(a, "POST", "/api/leads/L-1002", `{"stage":"New"}`, a.token).Code != 400 {
		t.Fatal("Won reopened")
	}
	for _, body := range []string{`{"optedOut":true}`, `{"optedOut":false}`} {
		if request(a, "POST", "/api/leads/L-1003/consent", body, a.token).Code != 200 {
			t.Fatal("consent update failed")
		}
	}
	s, _ := a.repo.Read(context.Background())
	l, _ := find(&s, "L-1003")
	if l.MarketingConsent {
		t.Fatal("clearing opt-out must not silently grant marketing consent")
	}
}
func TestAPIBoundaries(t *testing.T) {
	a := testApp(t)
	cases := []struct {
		method, path, body, token string
		status                    int
	}{{"GET", "/api/state", "", "", 401}, {"GET", "/api/state", "", "wrong", 401}, {"POST", "/api/leads", `{"name":"","branch":"Manado","channel":"Website","product":"MPV Family"}`, a.token, 400}, {"POST", "/api/leads", `{"unexpected":"field"}`, a.token, 400}, {"POST", "/api/leads", `{} {}`, a.token, 400}, {"POST", "/api/leads/L-1001/messages", `{"text":""}`, a.token, 400}, {"POST", "/api/leads/L-1001", `{"owner":"unknown"}`, a.token, 400}, {"POST", "/api/rules", `{"key":"aftersales","enabled":false}`, a.token, 400}, {"GET", "/api/state", "", a.token, 200}}
	for _, tc := range cases {
		w := request(a, tc.method, tc.path, tc.body, tc.token)
		if w.Code != tc.status {
			t.Errorf("%s %s => %d, want %d: %s", tc.method, tc.path, w.Code, tc.status, w.Body)
		}
	}
	r := httptest.NewRequest("GET", "/api/state", nil)
	r.Header.Set("Origin", "https://untrusted.example")
	r.Header.Set("Authorization", "Bearer "+a.token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("untrusted origin allowed")
	}
}
func TestFailedMutationRollsBack(t *testing.T) {
	a := testApp(t)
	_, err := a.repo.Update(context.Background(), func(s *State) error { s.Leads = nil; return io.ErrUnexpectedEOF })
	if err == nil {
		t.Fatal("expected failure")
	}
	s, _ := a.repo.Read(context.Background())
	if len(s.Leads) != 36 {
		t.Fatal("mutation not rolled back")
	}
}
func TestMessageRecordsFirstHumanReply(t *testing.T) {
	a := testApp(t)
	w := request(a, "POST", "/api/leads/L-1001/messages", `{"text":"Follow-up simulasi"}`, a.token)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	s, _ := a.repo.Read(context.Background())
	l, _ := find(&s, "L-1001")
	if l.FirstReplyMins == nil || *l.FirstReplyMins != 46 || len(l.Messages) != 2 {
		t.Fatal("first human response not recorded correctly")
	}
}
func TestDeepSeekOnlyReceivesControlledContext(t *testing.T) {
	a := testApp(t)
	var captured string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer mock-key" {
			t.Error("invalid DeepSeek request")
		}
		b, _ := io.ReadAll(r.Body)
		captured = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Bapak/Ibu, kapan waktu yang nyaman untuk berdiskusi?"}}]}`))
	}))
	defer server.Close()
	a.deepseekURL = server.URL
	a.deepseekKey = "mock-key"
	w := request(a, "POST", "/api/ai/draft", `{"leadId":"L-1001"}`, a.token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"mode":"deepseek"`) {
		t.Fatal(w.Body)
	}
	if strings.Contains(captured, "Aditya") || strings.Contains(captured, "+62") || strings.Contains(captured, "customerId") {
		t.Fatal("PII leaked into AI prompt")
	}
	if !strings.Contains(captured, "MPV Family") || !strings.Contains(captured, "deepseek-flash") {
		t.Fatal("controlled context missing")
	}
}
func TestDeepSeekFallbackAndProviderFailure(t *testing.T) {
	a := testApp(t)
	w := request(a, "POST", "/api/ai/draft", `{"leadId":"L-1001"}`, a.token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "local-template") {
		t.Fatal("missing honest no-key fallback")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`secret-provider-error`))
	}))
	defer server.Close()
	a.deepseekKey = "mock"
	a.deepseekURL = server.URL
	w = request(a, "POST", "/api/ai/draft", `{"leadId":"L-1001"}`, a.token)
	if w.Code != 502 || strings.Contains(w.Body.String(), "secret-provider-error") {
		t.Fatal("provider error not safely handled")
	}
}

func TestIntegrationStatusRequiresAuthAndNeverReturnsAPIKey(t *testing.T) {
	a := testApp(t)
	a.deepseekKey = "private-test-key-never-return-this"
	if request(a, "GET", "/api/integrations", "", "").Code != 401 {
		t.Fatal("integration metadata must require authentication")
	}
	w := request(a, "GET", "/api/integrations", "", a.token)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"configured":true`) || !strings.Contains(w.Body.String(), `"model":"deepseek-flash"`) {
		t.Fatalf("unexpected metadata: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), a.deepseekKey) {
		t.Fatal("provider API key leaked")
	}
	a.deepseekKey = ""
	w = request(a, "GET", "/api/integrations", "", a.token)
	if !strings.Contains(w.Body.String(), `"configured":false`) {
		t.Fatal("missing honest unconfigured status")
	}
}

func TestProviderErrorsAreActionableAndSanitized(t *testing.T) {
	for _, tc := range []struct {
		status int
		text   string
	}{{401, "API key DeepSeek ditolak"}, {402, "Saldo DeepSeek tidak mencukupi"}, {429, "Batas request DeepSeek tercapai"}} {
		t.Run(strconvTest(tc.status), func(t *testing.T) {
			a := testApp(t)
			a.deepseekKey = "mock-key"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("secret-provider-debug-payload"))
			}))
			defer server.Close()
			a.deepseekURL = server.URL
			w := request(a, "POST", "/api/ai/draft", `{"leadId":"L-1001"}`, a.token)
			if w.Code != 502 || !strings.Contains(w.Body.String(), tc.text) || strings.Contains(w.Body.String(), "secret-provider-debug-payload") {
				t.Fatalf("unexpected sanitized error: %s", w.Body.String())
			}
		})
	}
}
