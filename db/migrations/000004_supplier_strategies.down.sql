ALTER TABLE supplier_profiles 
DROP COLUMN IF EXISTS target_margin,
DROP COLUMN IF EXISTS utilization_rate,
DROP COLUMN IF EXISTS risk_tolerance,
DROP COLUMN IF EXISTS agent_tier;
