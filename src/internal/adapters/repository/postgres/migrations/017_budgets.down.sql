-- @sk-task 301-budget-enforcement#T1.2: Drop budget tables on rollback (AC-002)
DROP TABLE IF EXISTS budget_spend_daily;
DROP TABLE IF EXISTS budget_spend;
DROP TABLE IF EXISTS budgets;
