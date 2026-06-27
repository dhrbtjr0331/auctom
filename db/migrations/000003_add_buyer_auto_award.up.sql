-- Update supplier_profiles to support bidding strategies
ALTER TABLE supplier_profiles 
ADD COLUMN IF NOT EXISTS target_margin NUMERIC(4, 2) DEFAULT 0.20 CHECK (target_margin >= 0.05 AND target_margin <= 1.00),
ADD COLUMN IF NOT EXISTS utilization_rate NUMERIC(3, 2) DEFAULT 0.50 CHECK (utilization_rate >= 0.00 AND utilization_rate <= 1.00);

-- Update rfqs to support auto-award rules
ALTER TABLE rfqs
ADD COLUMN IF NOT EXISTS auto_award_mode VARCHAR(20) DEFAULT 'disabled' CHECK (auto_award_mode IN ('disabled', 'instant', 'one-click')),
ADD COLUMN IF NOT EXISTS target_budget NUMERIC(12, 2),
ADD COLUMN IF NOT EXISTS min_score_threshold NUMERIC(5, 4) DEFAULT 0.8000 CHECK (min_score_threshold >= 0.0000 AND min_score_threshold <= 1.0000);
