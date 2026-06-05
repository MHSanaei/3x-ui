package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v3/database"
	"github.com/mhsanaei/3x-ui/v3/database/model"
)

// PeriodStat is a money total plus the number of records that produced it.
type PeriodStat struct {
	Amount int64 `json:"amount"`
	Count  int64 `json:"count"`
}

// DailyPoint is one calendar day (server-local) of revenue and spend, used to
// drive the report chart.
type DailyPoint struct {
	Date    string `json:"date"` // YYYY-MM-DD in the server's local timezone
	Revenue int64  `json:"revenue"`
	Spend   int64  `json:"spend"`
}

// ResellerStat ranks a reseller by how much balance they have consumed.
type ResellerStat struct {
	UserId   int    `json:"userId"`
	Username string `json:"username"`
	Spend    int64  `json:"spend"`
	Clients  int64  `json:"clients"`
}

// IncomeReport is the admin income/analytics snapshot. Revenue is real money in
// (paid gateway payments); spend is wallet balance consumed by resellers
// (debit transactions, dominated by client-creation charges). Both are keyed by
// the same period slugs so the frontend can render them side by side.
type IncomeReport struct {
	Revenue      map[string]PeriodStat `json:"revenue"`
	Spend        map[string]PeriodStat `json:"spend"`
	NewClients   map[string]int64      `json:"newClients"`
	Daily        []DailyPoint          `json:"daily"`
	TopResellers []ResellerStat        `json:"topResellers"`
	PendingCount int64                 `json:"pendingCount"`
	TotalUsers   int64                 `json:"totalUsers"`
	TotalClients int64                 `json:"totalClients"`
	Outstanding  int64                 `json:"outstanding"` // sum of every user's current balance
}

// ReportService computes admin-facing income and activity aggregates. All time
// windows are anchored to the server's local midnight so "today" matches what
// the operator sees on the host clock.
type ReportService struct{}

type periodDef struct {
	key  string
	from int64 // inclusive, ms; 0 == since the beginning of time
	to   int64 // exclusive, ms; 0 == open-ended (up to now)
}

// IncomeReport assembles the full snapshot in a handful of aggregate queries.
func (s *ReportService) IncomeReport() (*IncomeReport, error) {
	db := database.GetDB()
	now := time.Now()
	loc := now.Location()
	ms := func(t time.Time) int64 { return t.UnixMilli() }

	startToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	startYesterday := startToday.AddDate(0, 0, -1)
	startWeek := startToday.AddDate(0, 0, -6) // last 7 days, today inclusive
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	startLastMonth := startMonth.AddDate(0, -1, 0)
	startYear := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)

	periods := []periodDef{
		{"today", ms(startToday), 0},
		{"yesterday", ms(startYesterday), ms(startToday)},
		{"last7", ms(startWeek), 0},
		{"thisMonth", ms(startMonth), 0},
		{"lastMonth", ms(startLastMonth), ms(startMonth)},
		{"thisYear", ms(startYear), 0},
		{"allTime", 0, 0},
	}

	report := &IncomeReport{
		Revenue:    make(map[string]PeriodStat, len(periods)),
		Spend:      make(map[string]PeriodStat, len(periods)),
		NewClients: make(map[string]int64, len(periods)),
	}

	// Revenue: paid gateway payments.
	revStat := func(p periodDef) (PeriodStat, error) {
		var r PeriodStat
		q := db.Model(&model.Payment{}).
			Select("COALESCE(SUM(amount),0) as amount, COUNT(*) as count").
			Where("status = ? AND created_at >= ?", model.PaymentPaid, p.from)
		if p.to > 0 {
			q = q.Where("created_at < ?", p.to)
		}
		err := q.Scan(&r).Error
		return r, err
	}

	// Spend: wallet debits (client-creation charges + admin deductions).
	spendStat := func(p periodDef) (PeriodStat, error) {
		var r PeriodStat
		q := db.Model(&model.Transaction{}).
			Select("COALESCE(SUM(amount),0) as amount, COUNT(*) as count").
			Where("type = ? AND created_at >= ?", model.TxDebit, p.from)
		if p.to > 0 {
			q = q.Where("created_at < ?", p.to)
		}
		err := q.Scan(&r).Error
		return r, err
	}

	// New clients created in the window.
	clientStat := func(p periodDef) (int64, error) {
		var n int64
		q := db.Model(&model.ClientRecord{}).Where("created_at >= ?", p.from)
		if p.to > 0 {
			q = q.Where("created_at < ?", p.to)
		}
		err := q.Count(&n).Error
		return n, err
	}

	for _, p := range periods {
		rev, err := revStat(p)
		if err != nil {
			return nil, err
		}
		report.Revenue[p.key] = rev

		sp, err := spendStat(p)
		if err != nil {
			return nil, err
		}
		report.Spend[p.key] = sp

		nc, err := clientStat(p)
		if err != nil {
			return nil, err
		}
		report.NewClients[p.key] = nc
	}

	// Daily series for the last 30 days (today inclusive).
	const dailyDays = 30
	start30 := startToday.AddDate(0, 0, -(dailyDays - 1))
	byDate := make(map[string]*DailyPoint, dailyDays)
	order := make([]string, 0, dailyDays)
	for i := 0; i < dailyDays; i++ {
		key := start30.AddDate(0, 0, i).Format("2006-01-02")
		dp := &DailyPoint{Date: key}
		byDate[key] = dp
		order = append(order, key)
	}
	dayKey := func(createdAt int64) string {
		return time.UnixMilli(createdAt).In(loc).Format("2006-01-02")
	}

	type tsAmount struct {
		CreatedAt int64
		Amount    int64
	}
	var payRows []tsAmount
	if err := db.Model(&model.Payment{}).
		Select("created_at, amount").
		Where("status = ? AND created_at >= ?", model.PaymentPaid, ms(start30)).
		Scan(&payRows).Error; err != nil {
		return nil, err
	}
	for _, row := range payRows {
		if dp := byDate[dayKey(row.CreatedAt)]; dp != nil {
			dp.Revenue += row.Amount
		}
	}

	var debRows []tsAmount
	if err := db.Model(&model.Transaction{}).
		Select("created_at, amount").
		Where("type = ? AND created_at >= ?", model.TxDebit, ms(start30)).
		Scan(&debRows).Error; err != nil {
		return nil, err
	}
	for _, row := range debRows {
		if dp := byDate[dayKey(row.CreatedAt)]; dp != nil {
			dp.Spend += row.Amount
		}
	}

	report.Daily = make([]DailyPoint, 0, dailyDays)
	for _, key := range order {
		report.Daily = append(report.Daily, *byDate[key])
	}

	// Top resellers by all-time spend.
	type spendRow struct {
		UserId int
		Spend  int64
	}
	var spendRows []spendRow
	if err := db.Model(&model.Transaction{}).
		Select("user_id, COALESCE(SUM(amount),0) as spend").
		Where("type = ?", model.TxDebit).
		Group("user_id").
		Order("spend desc").
		Limit(5).
		Scan(&spendRows).Error; err != nil {
		return nil, err
	}
	for _, sr := range spendRows {
		stat := ResellerStat{UserId: sr.UserId, Spend: sr.Spend}
		var u model.User
		if err := db.Select("username").Where("id = ?", sr.UserId).First(&u).Error; err == nil {
			stat.Username = u.Username
		}
		var clients int64
		db.Model(&model.ClientRecord{}).Where("owner_id = ?", sr.UserId).Count(&clients)
		stat.Clients = clients
		report.TopResellers = append(report.TopResellers, stat)
	}

	// Headline totals.
	if err := db.Model(&model.Payment{}).
		Where("status = ?", model.PaymentPending).
		Count(&report.PendingCount).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.User{}).Count(&report.TotalUsers).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&model.ClientRecord{}).Count(&report.TotalClients).Error; err != nil {
		return nil, err
	}
	var outstanding struct{ Total int64 }
	if err := db.Model(&model.User{}).
		Select("COALESCE(SUM(balance),0) as total").
		Scan(&outstanding).Error; err != nil {
		return nil, err
	}
	report.Outstanding = outstanding.Total

	return report, nil
}
