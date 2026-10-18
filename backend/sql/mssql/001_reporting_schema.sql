-- OneCRM sales reporting mart (schema rpt) for Microsoft SQL Server 2019+/2022.
-- Star schema fed by the DMS/ERP (SPK, DO, targets) and by OneCRM (lead source attribution).
-- Idempotent: every object is created only when missing, so the script can be re-run safely.
SET NOCOUNT ON;
SET QUOTED_IDENTIFIER ON;
SET ANSI_NULLS ON;
GO
IF DB_ID(N'onecrm_reporting') IS NULL CREATE DATABASE onecrm_reporting;
GO
USE onecrm_reporting;
GO
IF SCHEMA_ID(N'rpt') IS NULL EXEC (N'CREATE SCHEMA rpt AUTHORIZATION dbo');
GO

-- Dimensions. Keys are stable surrogate keys; names come from the DMS master data.
IF OBJECT_ID(N'rpt.DimBranch', N'U') IS NULL
CREATE TABLE rpt.DimBranch (
  BranchKey tinyint NOT NULL CONSTRAINT PK_DimBranch PRIMARY KEY,
  BranchName nvarchar(40) NOT NULL CONSTRAINT UQ_DimBranch_Name UNIQUE
);
IF OBJECT_ID(N'rpt.DimProduct', N'U') IS NULL
CREATE TABLE rpt.DimProduct (
  ProductKey tinyint NOT NULL CONSTRAINT PK_DimProduct PRIMARY KEY,
  ProductName nvarchar(60) NOT NULL CONSTRAINT UQ_DimProduct_Name UNIQUE,
  Segment nvarchar(30) NOT NULL
);
IF OBJECT_ID(N'rpt.DimSalesperson', N'U') IS NULL
CREATE TABLE rpt.DimSalesperson (
  SalespersonKey smallint NOT NULL CONSTRAINT PK_DimSalesperson PRIMARY KEY,
  SalespersonName nvarchar(60) NOT NULL CONSTRAINT UQ_DimSalesperson_Name UNIQUE,
  BranchKey tinyint NOT NULL CONSTRAINT FK_DimSalesperson_Branch REFERENCES rpt.DimBranch (BranchKey)
);
-- SortOrder follows SALES_SOURCES in lib/sales.ts: WhatsApp, Instagram, Facebook, TikTok, Website, Walk-in.
IF OBJECT_ID(N'rpt.DimLeadSource', N'U') IS NULL
CREATE TABLE rpt.DimLeadSource (
  LeadSourceKey tinyint NOT NULL CONSTRAINT PK_DimLeadSource PRIMARY KEY,
  LeadSourceName nvarchar(20) NOT NULL CONSTRAINT UQ_DimLeadSource_Name UNIQUE,
  SortOrder tinyint NOT NULL CONSTRAINT UQ_DimLeadSource_Sort UNIQUE
);
GO

-- Facts.
-- One row per SPK (vehicle order). DoDate is set only once the unit is delivered (DO); cancelled SPK never get a DoDate.
IF OBJECT_ID(N'rpt.FactSpk', N'U') IS NULL
CREATE TABLE rpt.FactSpk (
  SpkNo varchar(20) NOT NULL CONSTRAINT PK_FactSpk PRIMARY KEY,
  SpkDate date NOT NULL,
  DoDate date NULL,
  Status varchar(10) NOT NULL CONSTRAINT CK_FactSpk_Status CHECK (Status IN ('Open', 'Delivered', 'Cancelled')),
  BranchKey tinyint NOT NULL CONSTRAINT FK_FactSpk_Branch REFERENCES rpt.DimBranch (BranchKey),
  ProductKey tinyint NOT NULL CONSTRAINT FK_FactSpk_Product REFERENCES rpt.DimProduct (ProductKey),
  SalespersonKey smallint NOT NULL CONSTRAINT FK_FactSpk_Salesperson REFERENCES rpt.DimSalesperson (SalespersonKey),
  LeadSourceKey tinyint NOT NULL CONSTRAINT FK_FactSpk_LeadSource REFERENCES rpt.DimLeadSource (LeadSourceKey),
  PaymentType varchar(6) NOT NULL CONSTRAINT CK_FactSpk_Payment CHECK (PaymentType IN ('Cash', 'Kredit')),
  Amount decimal(18, 0) NOT NULL CONSTRAINT CK_FactSpk_Amount CHECK (Amount >= 0),
  CrmLeadId varchar(20) NULL,
  LoadedAt datetime2(3) NOT NULL CONSTRAINT DF_FactSpk_LoadedAt DEFAULT SYSUTCDATETIME(),
  CONSTRAINT CK_FactSpk_DoDate CHECK ((Status = 'Delivered' AND DoDate IS NOT NULL AND DoDate >= SpkDate) OR (Status <> 'Delivered' AND DoDate IS NULL))
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_FactSpk_SpkDate' AND object_id = OBJECT_ID(N'rpt.FactSpk'))
  CREATE INDEX IX_FactSpk_SpkDate ON rpt.FactSpk (SpkDate) INCLUDE (Status, BranchKey, ProductKey, SalespersonKey, LeadSourceKey, DoDate);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'IX_FactSpk_DoDate' AND object_id = OBJECT_ID(N'rpt.FactSpk'))
  CREATE INDEX IX_FactSpk_DoDate ON rpt.FactSpk (DoDate) INCLUDE (BranchKey, ProductKey, SalespersonKey, LeadSourceKey, PaymentType, Amount);
-- Production scale (millions of rows): add a nonclustered columnstore index on FactSpk for the aggregations below.

-- Unique leads per day, branch and source, pushed from OneCRM for lead-to-SPK attribution.
IF OBJECT_ID(N'rpt.FactLeadDaily', N'U') IS NULL
CREATE TABLE rpt.FactLeadDaily (
  LeadDate date NOT NULL,
  BranchKey tinyint NOT NULL CONSTRAINT FK_FactLeadDaily_Branch REFERENCES rpt.DimBranch (BranchKey),
  LeadSourceKey tinyint NOT NULL CONSTRAINT FK_FactLeadDaily_LeadSource REFERENCES rpt.DimLeadSource (LeadSourceKey),
  Leads int NOT NULL CONSTRAINT CK_FactLeadDaily_Leads CHECK (Leads >= 0),
  CONSTRAINT PK_FactLeadDaily PRIMARY KEY (LeadDate, BranchKey, LeadSourceKey)
);

-- Monthly unit targets per branch and product. TargetMonth is always the first day of the month.
IF OBJECT_ID(N'rpt.FactTarget', N'U') IS NULL
CREATE TABLE rpt.FactTarget (
  TargetMonth date NOT NULL CONSTRAINT CK_FactTarget_Month CHECK (DAY(TargetMonth) = 1),
  BranchKey tinyint NOT NULL CONSTRAINT FK_FactTarget_Branch REFERENCES rpt.DimBranch (BranchKey),
  ProductKey tinyint NOT NULL CONSTRAINT FK_FactTarget_Product REFERENCES rpt.DimProduct (ProductKey),
  TargetUnits int NOT NULL CONSTRAINT CK_FactTarget_Units CHECK (TargetUnits >= 0),
  CONSTRAINT PK_FactTarget PRIMARY KEY (TargetMonth, BranchKey, ProductKey)
);

-- Incremental sync state: last Change Tracking version and load time per source table (drives the dashboard "data per" stamp).
IF OBJECT_ID(N'rpt.EtlWatermark', N'U') IS NULL
CREATE TABLE rpt.EtlWatermark (
  SourceTable sysname NOT NULL CONSTRAINT PK_EtlWatermark PRIMARY KEY,
  LastVersion bigint NOT NULL,
  LoadedAt datetime2(3) NOT NULL
);
GO
