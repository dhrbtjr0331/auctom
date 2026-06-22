ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS target_margin;
ALTER TABLE supplier_profiles DROP COLUMN IF EXISTS utilization_rate;

ALTER TABLE rfqs DROP COLUMN IF EXISTS auto_award_mode;
ALTER TABLE rfqs DROP COLUMN IF EXISTS target_budget;
ALTER TABLE rfqs DROP COLUMN IF EXISTS min_score_threshold;
