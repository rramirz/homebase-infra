ALTER TABLE people ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
ALTER TABLE liabilities ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
ALTER TABLE incomes ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
ALTER TABLE expenses ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
ALTER TABLE retirement_plans ADD COLUMN IF NOT EXISTS confidence text NOT NULL DEFAULT 'unknown' CHECK(confidence IN ('verified','estimated','unknown'));
