ALTER TABLE supplier_profiles 
ADD COLUMN target_margin NUMERIC(4, 2) DEFAULT 0.20 CHECK (target_margin >= 0.05 AND target_margin <= 1.00),
ADD COLUMN utilization_rate NUMERIC(3, 2) DEFAULT 0.50 CHECK (utilization_rate >= 0.00 AND utilization_rate <= 1.00),
ADD COLUMN risk_tolerance NUMERIC(3, 2) DEFAULT 0.50 CHECK (risk_tolerance >= 0.00 AND risk_tolerance <= 1.00),
ADD COLUMN agent_tier VARCHAR(20) DEFAULT 'freemium' CHECK (agent_tier IN ('freemium', 'premium'));
