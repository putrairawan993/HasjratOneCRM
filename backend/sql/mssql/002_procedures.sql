-- rpt.usp_SalesDashboard: the only entry point the OneCRM API may call (see 003_security.sql).
-- Returns 6 result sets that map 1:1 to SalesDashboard in lib/sales.ts:
--   1 kpi (one row, incl. as_of)  2 trend  3 branches  4 products  5 salespeople  6 channels
-- Metric definitions (keep identical to lib/sales.ts):
--   spk          SPK created in the period, all statuses
--   cancelled    of those, Status = 'Cancelled'
--   delivered    DO dated in the period; revenue = SUM(Amount) of those DO
--   target       monthly targets of every month the period touches
--   outstanding  SPK backlog at period end: SpkDate <= @DateTo, not cancelled, not delivered by @DateTo
--   credit       delivered units paid through leasing (PaymentType = 'Kredit')
--   leads        unique CRM leads created in the period
--   trend        6 calendar months ending month(@DateTo), capped at @DateTo, independent of @DateFrom
-- Every ratio (achievement, conversion, credit share) is derived by the UI from these counts.
SET NOCOUNT ON;
SET QUOTED_IDENTIFIER ON;
SET ANSI_NULLS ON;
GO
USE onecrm_reporting;
GO
CREATE OR ALTER PROCEDURE rpt.usp_SalesDashboard
  @DateFrom date,
  @DateTo date,
  @Branch nvarchar(40) = NULL
AS
BEGIN
  SET NOCOUNT ON;
  SET XACT_ABORT ON;

  IF @DateFrom IS NULL OR @DateTo IS NULL THROW 50001, N'DateFrom dan DateTo wajib diisi.', 1;
  IF @DateFrom > @DateTo THROW 50002, N'DateFrom harus sebelum atau sama dengan DateTo.', 1;
  IF DATEDIFF(day, @DateFrom, @DateTo) > 366 THROW 50003, N'Rentang periode maksimal 366 hari.', 1;

  DECLARE @BranchKey tinyint = NULL;
  IF NULLIF(@Branch, N'') IS NOT NULL
  BEGIN
    SELECT @BranchKey = BranchKey FROM rpt.DimBranch WHERE BranchName = @Branch COLLATE Latin1_General_BIN2;
    IF @BranchKey IS NULL THROW 50004, N'Cabang tidak dikenal.', 1;
  END;

  DECLARE @MonthFrom date = DATEFROMPARTS(YEAR(@DateFrom), MONTH(@DateFrom), 1);
  DECLARE @MonthTo date = DATEFROMPARTS(YEAR(@DateTo), MONTH(@DateTo), 1);
  DECLARE @TrendStart date = DATEADD(month, -5, @MonthTo);

  -- Branch-scoped working set: only rows that can affect a period metric or the backlog at @DateTo.
  SELECT s.SpkDate, s.DoDate, s.Status, s.BranchKey, s.ProductKey, s.SalespersonKey, s.LeadSourceKey, s.PaymentType, s.Amount,
    InSpk = CASE WHEN s.SpkDate BETWEEN @DateFrom AND @DateTo THEN 1 ELSE 0 END,
    InDo = CASE WHEN s.DoDate BETWEEN @DateFrom AND @DateTo THEN 1 ELSE 0 END
  INTO #spk
  FROM rpt.FactSpk AS s
  WHERE (@BranchKey IS NULL OR s.BranchKey = @BranchKey) AND s.SpkDate <= @DateTo
  OPTION (RECOMPILE);

  -- 1 kpi
  SELECT
    spk = COUNT(CASE WHEN InSpk = 1 THEN 1 END),
    cancelled = COUNT(CASE WHEN InSpk = 1 AND Status = 'Cancelled' THEN 1 END),
    delivered = COUNT(CASE WHEN InDo = 1 THEN 1 END),
    revenue = CAST(COALESCE(SUM(CASE WHEN InDo = 1 THEN Amount END), 0) AS bigint),
    target = (SELECT CAST(COALESCE(SUM(t.TargetUnits), 0) AS bigint) FROM rpt.FactTarget AS t WHERE t.TargetMonth BETWEEN @MonthFrom AND @MonthTo AND (@BranchKey IS NULL OR t.BranchKey = @BranchKey)),
    outstanding = COUNT(CASE WHEN Status <> 'Cancelled' AND (DoDate IS NULL OR DoDate > @DateTo) THEN 1 END),
    credit = COUNT(CASE WHEN InDo = 1 AND PaymentType = 'Kredit' THEN 1 END),
    leads = (SELECT CAST(COALESCE(SUM(l.Leads), 0) AS bigint) FROM rpt.FactLeadDaily AS l WHERE l.LeadDate BETWEEN @DateFrom AND @DateTo AND (@BranchKey IS NULL OR l.BranchKey = @BranchKey)),
    as_of = (SELECT MAX(w.LoadedAt) FROM rpt.EtlWatermark AS w)
  FROM #spk
  OPTION (RECOMPILE);

  -- 2 trend (reads the fact table directly: trend months may start before @DateFrom)
  SELECT
    [month] = CONVERT(char(7), m.MonthStart, 126),
    spk = (SELECT COUNT(*) FROM rpt.FactSpk AS s WHERE s.SpkDate >= m.MonthStart AND s.SpkDate < m.NextMonth AND s.SpkDate <= @DateTo AND (@BranchKey IS NULL OR s.BranchKey = @BranchKey)),
    delivered = (SELECT COUNT(*) FROM rpt.FactSpk AS s WHERE s.DoDate >= m.MonthStart AND s.DoDate < m.NextMonth AND s.DoDate <= @DateTo AND (@BranchKey IS NULL OR s.BranchKey = @BranchKey)),
    target = (SELECT CAST(COALESCE(SUM(t.TargetUnits), 0) AS bigint) FROM rpt.FactTarget AS t WHERE t.TargetMonth = m.MonthStart AND (@BranchKey IS NULL OR t.BranchKey = @BranchKey))
  FROM (VALUES (0), (1), (2), (3), (4), (5)) AS n (i)
  CROSS APPLY (SELECT MonthStart = DATEADD(month, n.i, @TrendStart), NextMonth = DATEADD(month, n.i + 1, @TrendStart)) AS m
  ORDER BY m.MonthStart
  OPTION (RECOMPILE);

  -- 3 branches (all branches in key order, or only the filtered one)
  SELECT
    branch = b.BranchName,
    spk = COUNT(CASE WHEN s.InSpk = 1 THEN 1 END),
    delivered = COUNT(CASE WHEN s.InDo = 1 THEN 1 END),
    revenue = CAST(COALESCE(SUM(CASE WHEN s.InDo = 1 THEN s.Amount END), 0) AS bigint),
    target = (SELECT CAST(COALESCE(SUM(t.TargetUnits), 0) AS bigint) FROM rpt.FactTarget AS t WHERE t.BranchKey = b.BranchKey AND t.TargetMonth BETWEEN @MonthFrom AND @MonthTo)
  FROM rpt.DimBranch AS b
  LEFT JOIN #spk AS s ON s.BranchKey = b.BranchKey
  WHERE @BranchKey IS NULL OR b.BranchKey = @BranchKey
  GROUP BY b.BranchKey, b.BranchName
  ORDER BY b.BranchKey;

  -- 4 products (every product, facts scoped to the branch filter)
  SELECT
    product = p.ProductName,
    spk = COUNT(CASE WHEN s.InSpk = 1 THEN 1 END),
    delivered = COUNT(CASE WHEN s.InDo = 1 THEN 1 END),
    revenue = CAST(COALESCE(SUM(CASE WHEN s.InDo = 1 THEN s.Amount END), 0) AS bigint),
    target = (SELECT CAST(COALESCE(SUM(t.TargetUnits), 0) AS bigint) FROM rpt.FactTarget AS t WHERE t.ProductKey = p.ProductKey AND t.TargetMonth BETWEEN @MonthFrom AND @MonthTo AND (@BranchKey IS NULL OR t.BranchKey = @BranchKey))
  FROM rpt.DimProduct AS p
  LEFT JOIN #spk AS s ON s.ProductKey = p.ProductKey
  GROUP BY p.ProductKey, p.ProductName
  ORDER BY p.ProductKey;

  -- 5 salespeople (leaderboard)
  SELECT
    name = sp.SalespersonName,
    branch = b.BranchName,
    spk = COUNT(CASE WHEN s.InSpk = 1 THEN 1 END),
    delivered = COUNT(CASE WHEN s.InDo = 1 THEN 1 END),
    revenue = CAST(COALESCE(SUM(CASE WHEN s.InDo = 1 THEN s.Amount END), 0) AS bigint)
  FROM rpt.DimSalesperson AS sp
  JOIN rpt.DimBranch AS b ON b.BranchKey = sp.BranchKey
  LEFT JOIN #spk AS s ON s.SalespersonKey = sp.SalespersonKey
  WHERE @BranchKey IS NULL OR sp.BranchKey = @BranchKey
  GROUP BY sp.SalespersonKey, sp.SalespersonName, b.BranchName
  ORDER BY delivered DESC, revenue DESC, name ASC;

  -- 6 channels (lead source attribution: CRM leads -> SPK -> DO)
  SELECT
    channel = ls.LeadSourceName,
    leads = (SELECT CAST(COALESCE(SUM(l.Leads), 0) AS bigint) FROM rpt.FactLeadDaily AS l WHERE l.LeadSourceKey = ls.LeadSourceKey AND l.LeadDate BETWEEN @DateFrom AND @DateTo AND (@BranchKey IS NULL OR l.BranchKey = @BranchKey)),
    spk = COUNT(CASE WHEN s.InSpk = 1 THEN 1 END),
    delivered = COUNT(CASE WHEN s.InDo = 1 THEN 1 END)
  FROM rpt.DimLeadSource AS ls
  LEFT JOIN #spk AS s ON s.LeadSourceKey = ls.LeadSourceKey
  GROUP BY ls.LeadSourceKey, ls.LeadSourceName, ls.SortOrder
  ORDER BY ls.SortOrder;
END;
GO
