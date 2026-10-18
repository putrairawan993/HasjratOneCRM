package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/golang-sql/civil"
	_ "github.com/microsoft/go-mssqldb"
)

// SalesDashboard mirrors SalesDashboard in lib/sales.ts. It carries counts only; the UI derives every ratio.
type SalesKPI struct {
	Spk         int64 `json:"spk"`
	Cancelled   int64 `json:"cancelled"`
	Delivered   int64 `json:"delivered"`
	Revenue     int64 `json:"revenue"`
	Target      int64 `json:"target"`
	Outstanding int64 `json:"outstanding"`
	Credit      int64 `json:"credit"`
	Leads       int64 `json:"leads"`
}
type SalesTrend struct {
	Month     string `json:"month"`
	Spk       int64  `json:"spk"`
	Delivered int64  `json:"delivered"`
	Target    int64  `json:"target"`
}
type SalesBranch struct {
	Branch    string `json:"branch"`
	Spk       int64  `json:"spk"`
	Delivered int64  `json:"delivered"`
	Revenue   int64  `json:"revenue"`
	Target    int64  `json:"target"`
}
type SalesProduct struct {
	Product   string `json:"product"`
	Spk       int64  `json:"spk"`
	Delivered int64  `json:"delivered"`
	Revenue   int64  `json:"revenue"`
	Target    int64  `json:"target"`
}
type SalesPerson struct {
	Name      string `json:"name"`
	Branch    string `json:"branch"`
	Spk       int64  `json:"spk"`
	Delivered int64  `json:"delivered"`
	Revenue   int64  `json:"revenue"`
}
type SalesChannel struct {
	Channel   string `json:"channel"`
	Leads     int64  `json:"leads"`
	Spk       int64  `json:"spk"`
	Delivered int64  `json:"delivered"`
}
type SalesDashboard struct {
	Source      string         `json:"source"`
	AsOf        string         `json:"asOf"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Branch      *string        `json:"branch"`
	KPI         SalesKPI       `json:"kpi"`
	Trend       []SalesTrend   `json:"trend"`
	Branches    []SalesBranch  `json:"branches"`
	Products    []SalesProduct `json:"products"`
	Salespeople []SalesPerson  `json:"salespeople"`
	Channels    []SalesChannel `json:"channels"`
}
type SalesFilter struct {
	From, To civil.Date
	Branch   string
}
type salesSource interface {
	Dashboard(context.Context, SalesFilter) (SalesDashboard, error)
}

// parseSalesFilter applies the same rules as validSalesFilter in lib/sales.ts before anything reaches SQL Server.
func parseSalesFilter(q url.Values) (SalesFilter, error) {
	from, errFrom := civil.ParseDate(q.Get("from"))
	to, errTo := civil.ParseDate(q.Get("to"))
	if errFrom != nil || errTo != nil || len(q.Get("from")) != 10 || len(q.Get("to")) != 10 {
		return SalesFilter{}, errors.New("from dan to wajib berformat YYYY-MM-DD")
	}
	if from.After(to) {
		return SalesFilter{}, errors.New("from harus sebelum atau sama dengan to")
	}
	if to.DaysSince(from) > 366 {
		return SalesFilter{}, errors.New("rentang periode maksimal 366 hari")
	}
	branch := q.Get("branch")
	if branch != "" && !slices.Contains(branches, branch) {
		return SalesFilter{}, errors.New("cabang tidak valid")
	}
	return SalesFilter{From: from, To: to, Branch: branch}, nil
}

type mssqlSales struct{ db *sql.DB }

// Dashboard calls rpt.usp_SalesDashboard (EXECUTE is the API login's only permission) and reads its 6 result sets in order.
func (m *mssqlSales) Dashboard(ctx context.Context, f SalesFilter) (SalesDashboard, error) {
	rows, err := m.db.QueryContext(ctx, "rpt.usp_SalesDashboard", sql.Named("DateFrom", f.From), sql.Named("DateTo", f.To), sql.Named("Branch", f.Branch))
	if err != nil {
		return SalesDashboard{}, err
	}
	defer rows.Close()
	d := SalesDashboard{Source: "mssql", From: f.From.String(), To: f.To.String(), Trend: []SalesTrend{}, Branches: []SalesBranch{}, Products: []SalesProduct{}, Salespeople: []SalesPerson{}, Channels: []SalesChannel{}}
	if f.Branch != "" {
		d.Branch = &f.Branch
	}
	var asOf sql.NullTime
	kpiRows := 0
	sets := []func() error{
		func() error {
			kpiRows++
			k := &d.KPI
			return rows.Scan(&k.Spk, &k.Cancelled, &k.Delivered, &k.Revenue, &k.Target, &k.Outstanding, &k.Credit, &k.Leads, &asOf)
		},
		func() error {
			var t SalesTrend
			err := rows.Scan(&t.Month, &t.Spk, &t.Delivered, &t.Target)
			d.Trend = append(d.Trend, t)
			return err
		},
		func() error {
			var b SalesBranch
			err := rows.Scan(&b.Branch, &b.Spk, &b.Delivered, &b.Revenue, &b.Target)
			d.Branches = append(d.Branches, b)
			return err
		},
		func() error {
			var p SalesProduct
			err := rows.Scan(&p.Product, &p.Spk, &p.Delivered, &p.Revenue, &p.Target)
			d.Products = append(d.Products, p)
			return err
		},
		func() error {
			var p SalesPerson
			err := rows.Scan(&p.Name, &p.Branch, &p.Spk, &p.Delivered, &p.Revenue)
			d.Salespeople = append(d.Salespeople, p)
			return err
		},
		func() error {
			var c SalesChannel
			err := rows.Scan(&c.Channel, &c.Leads, &c.Spk, &c.Delivered)
			d.Channels = append(d.Channels, c)
			return err
		},
	}
	for i, scan := range sets {
		if i > 0 && !rows.NextResultSet() {
			return SalesDashboard{}, fmt.Errorf("result set %d tidak ditemukan", i+1)
		}
		for rows.Next() {
			if err = scan(); err != nil {
				return SalesDashboard{}, err
			}
		}
		if err = rows.Err(); err != nil {
			return SalesDashboard{}, err
		}
	}
	if kpiRows != 1 {
		return SalesDashboard{}, fmt.Errorf("kpi harus satu baris, diterima %d", kpiRows)
	}
	if asOf.Valid {
		// Same shape as Date.prototype.toISOString(), so MS SQL and the simulation produce identical stamps.
		d.AsOf = asOf.Time.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return d, nil
}

func (a *app) salesDashboard(w http.ResponseWriter, r *http.Request) {
	if a.sales == nil {
		fail(w, 503, "MS SQL belum dikonfigurasi")
		return
	}
	f, err := parseSalesFilter(r.URL.Query())
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	d, err := a.sales.Dashboard(ctx, f)
	if err != nil {
		log.Printf("sales dashboard query failed: %T", err)
		fail(w, 503, "database penjualan tidak tersedia")
		return
	}
	respond(w, 200, d)
}

// openSales connects the optional MS SQL reporting database. A failed ping only warns: the pool reconnects per request.
func openSales(dsn string) (*sql.DB, salesSource) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		log.Fatal("invalid MS SQL configuration")
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		log.Print("warning: MS SQL reporting database unreachable; /api/sales/dashboard returns 503 until it is available")
	}
	return db, &mssqlSales{db: db}
}
