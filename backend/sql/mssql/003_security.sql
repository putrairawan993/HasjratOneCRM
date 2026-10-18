-- Least-privilege access for the OneCRM API, a Row-Level Security example and a Change Tracking sync template.
-- Run with sqlcmd and pass the API password as a variable (never commit it):
--   sqlcmd -C -b -I -S <server> -U <admin> -v ApiPassword="<strong password without single quotes>" -i 003_security.sql
SET NOCOUNT ON;
SET QUOTED_IDENTIFIER ON;
SET ANSI_NULLS ON;
GO
USE master;
GO
IF SUSER_ID(N'svc_onecrm_dashboard') IS NULL
  CREATE LOGIN svc_onecrm_dashboard WITH PASSWORD = N'$(ApiPassword)', CHECK_POLICY = ON, DEFAULT_DATABASE = onecrm_reporting;
ELSE
  ALTER LOGIN svc_onecrm_dashboard WITH PASSWORD = N'$(ApiPassword)';
GO
USE onecrm_reporting;
GO
IF USER_ID(N'svc_onecrm_dashboard') IS NULL CREATE USER svc_onecrm_dashboard FOR LOGIN svc_onecrm_dashboard;
IF DATABASE_PRINCIPAL_ID(N'rpt_dashboard_reader') IS NULL CREATE ROLE rpt_dashboard_reader;
IF IS_ROLEMEMBER(N'rpt_dashboard_reader', N'svc_onecrm_dashboard') = 0 ALTER ROLE rpt_dashboard_reader ADD MEMBER svc_onecrm_dashboard;
-- The API may only execute the dashboard procedure. dbo owns both the procedure and the rpt tables, so ownership chaining
-- lets the procedure read them while any ad-hoc SELECT from this login is refused.
GRANT EXECUTE ON OBJECT::rpt.usp_SalesDashboard TO rpt_dashboard_reader;
DENY SELECT, INSERT, UPDATE, DELETE ON SCHEMA::rpt TO rpt_dashboard_reader;
GO

-- Row-Level Security example for branch managers (created with STATE = OFF; enable after the API sets session context).
-- Per request the API would run, on the same connection, before calling the procedure:
--   EXEC sys.sp_set_session_context @key = N'rpt_scope', @value = N'all', @read_only = 1;         -- management
--   EXEC sys.sp_set_session_context @key = N'rpt_branch_key', @value = 2, @read_only = 1;        -- Palu branch manager
-- With the policy ON, a session that sets neither key sees no rows (fail closed).
-- Created only when missing: a schema-bound predicate cannot be altered while the policy references it.
IF OBJECT_ID(N'rpt.fn_BranchAccess', N'IF') IS NULL
  EXEC (N'CREATE FUNCTION rpt.fn_BranchAccess (@BranchKey tinyint)
  RETURNS TABLE
  WITH SCHEMABINDING
  AS
  RETURN
    SELECT allowed = 1
    WHERE CAST(SESSION_CONTEXT(N''rpt_scope'') AS nvarchar(10)) = N''all''
       OR CAST(SESSION_CONTEXT(N''rpt_branch_key'') AS tinyint) = @BranchKey;');
GO
IF NOT EXISTS (SELECT 1 FROM sys.security_policies WHERE name = N'BranchAccessPolicy' AND schema_id = SCHEMA_ID(N'rpt'))
  EXEC (N'CREATE SECURITY POLICY rpt.BranchAccessPolicy
    ADD FILTER PREDICATE rpt.fn_BranchAccess(BranchKey) ON rpt.FactSpk,
    ADD FILTER PREDICATE rpt.fn_BranchAccess(BranchKey) ON rpt.FactLeadDaily,
    ADD FILTER PREDICATE rpt.fn_BranchAccess(BranchKey) ON rpt.FactTarget
    WITH (STATE = OFF)');
GO

/* ---------------------------------------------------------------------------------------------------------------------
   Change Tracking sync template (ILLUSTRATIVE ONLY: dms_prod, dbo.SalesOrderSpk and its columns are placeholder names for
   the real DMS/ERP database; agree the actual tables with the DBA). Not executed by this script.

   -- 1. Once, on the DMS/ERP source (DBA approval required):
   ALTER DATABASE dms_prod SET CHANGE_TRACKING = ON (CHANGE_RETENTION = 3 DAYS, AUTO_CLEANUP = ON);
   ALTER TABLE dms_prod.dbo.SalesOrderSpk ENABLE CHANGE_TRACKING WITH (TRACK_COLUMNS_UPDATED = OFF);

   -- 2. Every few minutes the Go sync worker, using a read-only login on the source (ideally a readable AG secondary):
   DECLARE @last bigint = (SELECT LastVersion FROM onecrm_reporting.rpt.EtlWatermark WHERE SourceTable = N'dms.SalesOrderSpk');
   DECLARE @now bigint = CHANGE_TRACKING_CURRENT_VERSION();
   IF @last < CHANGE_TRACKING_MIN_VALID_VERSION(OBJECT_ID(N'dbo.SalesOrderSpk'))
     THROW 50010, N'Watermark expired: run a full reload of FactSpk, then resume incremental sync.', 1;
   SELECT ct.SYS_CHANGE_OPERATION, ct.SpkNo, src.SpkDate, src.DoDate, src.Status, src.BranchCode, src.ModelCode,
          src.SalesCode, src.LeadSource, src.PaymentType, src.NetAmount, src.CrmLeadId
   FROM CHANGETABLE(CHANGES dbo.SalesOrderSpk, @last) AS ct
   LEFT JOIN dbo.SalesOrderSpk AS src ON src.SpkNo = ct.SpkNo;      -- deletes come back with src.* = NULL

   -- 3. The worker MERGEs the batch into rpt.FactSpk (mapping codes to dimension keys) and, in the same transaction:
   UPDATE onecrm_reporting.rpt.EtlWatermark SET LastVersion = @now, LoadedAt = SYSUTCDATETIME()
   WHERE SourceTable = N'dms.SalesOrderSpk';
   -- A nightly job reconciles row counts and amounts per day between source and FactSpk and alerts on any difference.
--------------------------------------------------------------------------------------------------------------------- */
