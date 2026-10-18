package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-sql/civil"
)

type fakeSales struct {
	got SalesFilter
	err error
}

func (f *fakeSales) Dashboard(_ context.Context, filter SalesFilter) (SalesDashboard, error) {
	f.got = filter
	if f.err != nil {
		return SalesDashboard{}, f.err
	}
	return SalesDashboard{Source: "mssql", AsOf: "2026-10-17T07:30:00.000Z", From: filter.From.String(), To: filter.To.String(), KPI: SalesKPI{Spk: 3, Delivered: 2, Revenue: 570000000, Target: 4}, Trend: []SalesTrend{{Month: "2026-10", Spk: 3, Delivered: 2, Target: 4}}, Branches: []SalesBranch{}, Products: []SalesProduct{}, Salespeople: []SalesPerson{}, Channels: []SalesChannel{{Channel: "WhatsApp", Leads: 40, Spk: 3, Delivered: 2}}}, nil
}

func TestSalesFilterValidation(t *testing.T) {
	cases := []struct {
		query string
		ok    bool
	}{
		{"from=2026-10-01&to=2026-10-17", true},
		{"from=2026-10-17&to=2026-10-17&branch=Palu", true},
		{"from=2025-10-16&to=2026-10-17", true}, // exactly 366 days
		{"from=2025-10-15&to=2026-10-17", false},
		{"from=2026-10-18&to=2026-10-17", false},
		{"from=2026-02-30&to=2026-03-01", false},
		{"from=2026-1-05&to=2026-10-17", false},
		{"to=2026-10-17", false},
		{"from=2026-10-01&to=2026-10-17&branch=palu", false},
		{"from=2026-10-01&to=2026-10-17&branch=Jakarta", false},
	}
	for _, tc := range cases {
		q, _ := url.ParseQuery(tc.query)
		if _, err := parseSalesFilter(q); (err == nil) != tc.ok {
			t.Errorf("%s: err=%v, want ok=%t", tc.query, err, tc.ok)
		}
	}
}
func TestSalesEndpointBoundaries(t *testing.T) {
	a := testApp(t)
	path := "/api/sales/dashboard?from=2026-10-01&to=2026-10-17"
	if w := request(a, "GET", path, "", ""); w.Code != 401 {
		t.Fatal("sales data must require the API token", w.Code)
	}
	if w := request(a, "GET", path, "", a.token); w.Code != 503 || !strings.Contains(w.Body.String(), "MS SQL belum dikonfigurasi") {
		t.Fatal("unconfigured MS SQL must answer 503", w.Code, w.Body)
	}
	fake := &fakeSales{}
	a.sales = fake
	if w := request(a, "GET", "/api/sales/dashboard?from=2026-10-17&to=2026-10-01", "", a.token); w.Code != 400 {
		t.Fatal("invalid filter must be rejected before SQL", w.Code)
	}
	fake.err = errors.New("mssql: login failed for user 'svc_onecrm_dashboard'")
	if w := request(a, "GET", path, "", a.token); w.Code != 503 || strings.Contains(w.Body.String(), "svc_onecrm") {
		t.Fatal("database errors must be generic 503", w.Code, w.Body)
	}
}
func TestSalesDashboardJSONContract(t *testing.T) {
	a := testApp(t)
	fake := &fakeSales{}
	a.sales = fake
	w := request(a, "GET", "/api/sales/dashboard?from=2026-10-01&to=2026-10-17&branch=Palu", "", a.token)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if fake.got != (SalesFilter{From: civil.Date{Year: 2026, Month: time.October, Day: 1}, To: civil.Date{Year: 2026, Month: time.October, Day: 17}, Branch: "Palu"}) {
		t.Fatalf("filter not passed through: %+v", fake.got)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	keys := func(m map[string]json.RawMessage) []string {
		out := []string{}
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	if got := keys(body); !slices.Equal(got, []string{"asOf", "branch", "branches", "channels", "from", "kpi", "products", "salespeople", "source", "to", "trend"}) {
		t.Fatal("top-level keys drifted from lib/sales.ts:", got)
	}
	var kpi map[string]json.RawMessage
	_ = json.Unmarshal(body["kpi"], &kpi)
	if got := keys(kpi); !slices.Equal(got, []string{"cancelled", "credit", "delivered", "leads", "outstanding", "revenue", "spk", "target"}) {
		t.Fatal("kpi keys drifted from lib/sales.ts:", got)
	}
	if string(body["branches"]) != "[]" {
		t.Fatal("empty lists must encode as [], not null")
	}
	w = request(a, "GET", "/api/integrations", "", a.token)
	if !strings.Contains(w.Body.String(), `"mssql":{"configured":true}`) {
		t.Fatal("integrations must report MS SQL status", w.Body)
	}
}

// TestMSSQLMatchesGolden runs only against a real SQL Server loaded with backend/sql/mssql/001-004:
//
//	MSSQL_TEST_DSN='sqlserver://svc_onecrm_dashboard:<password>@127.0.0.1:14333?database=onecrm_reporting&encrypt=true&TrustServerCertificate=true' go test -run MSSQL ./...
//
// Set MSSQL_TEST_EXPECT_NO_SELECT=1 when the DSN uses the least-privilege API login to also prove table access is denied.
func TestMSSQLMatchesGolden(t *testing.T) {
	dsn := os.Getenv("MSSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MSSQL_TEST_DSN not set; skipping SQL Server integration test")
	}
	raw, err := os.ReadFile("testdata/sales-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []struct {
			Filter   struct{ From, To, Branch string } `json:"filter"`
			Expected SalesDashboard                    `json:"expected"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(raw, &golden); err != nil || len(golden.Cases) == 0 {
		t.Fatal("invalid golden file", err)
	}
	db, src := openSales(dsn)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, c := range golden.Cases {
		f, err := parseSalesFilter(url.Values{"from": {c.Filter.From}, "to": {c.Filter.To}, "branch": {c.Filter.Branch}})
		if err != nil {
			t.Fatal(err)
		}
		got, err := src.Dashboard(ctx, f)
		if err != nil {
			t.Fatalf("%+v: %v", c.Filter, err)
		}
		want := c.Expected
		want.Source = "mssql"
		if !reflect.DeepEqual(got, want) {
			g, _ := json.Marshal(got)
			e, _ := json.Marshal(want)
			t.Errorf("%+v: SQL Server differs from lib/sales.ts\n got: %s\nwant: %s", c.Filter, g, e)
		}
	}
	t.Logf("%d golden cases match lib/sales.ts", len(golden.Cases))
	if os.Getenv("MSSQL_TEST_EXPECT_NO_SELECT") == "1" {
		var n int
		if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM rpt.FactSpk").Scan(&n); err == nil {
			t.Fatal("API login can read rpt tables directly; expected EXECUTE-only access")
		}
	}
	if _, err = src.Dashboard(ctx, SalesFilter{From: civil.Date{Year: 2026, Month: 10, Day: 17}, To: civil.Date{Year: 2026, Month: 10, Day: 1}}); err == nil {
		t.Fatal("procedure must THROW on an inverted period")
	}
}
