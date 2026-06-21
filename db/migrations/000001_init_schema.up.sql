CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('buyer', 'supplier')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS supplier_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_name VARCHAR(255) NOT NULL,
    capabilities VARCHAR(255)[] NOT NULL,
    capacity_index INT NOT NULL CHECK (capacity_index >= 1 AND capacity_index <= 100),
    rating NUMERIC(3, 2) NOT NULL CHECK (rating >= 1.0 AND rating <= 5.0),
    base_lead_time INT NOT NULL CHECK (base_lead_time > 0),
    risk_score NUMERIC(3, 2) NOT NULL CHECK (risk_score >= 0.0 AND risk_score <= 1.0),
    auto_bid BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rfqs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    part_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    required_capability VARCHAR(255) NOT NULL,
    priority_cost NUMERIC(3, 2) NOT NULL DEFAULT 0.33 CHECK (priority_cost >= 0.0 AND priority_cost <= 1.0),
    priority_lead_time NUMERIC(3, 2) NOT NULL DEFAULT 0.33 CHECK (priority_lead_time >= 0.0 AND priority_lead_time <= 1.0),
    priority_risk NUMERIC(3, 2) NOT NULL DEFAULT 0.34 CHECK (priority_risk >= 0.0 AND priority_risk <= 1.0),
    status VARCHAR(50) NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'matching', 'matched', 'quotes_generated', 'ranked', 'awarded')),
    awarded_quote_id UUID,
    spec_size VARCHAR(100) DEFAULT '',
    spec_material VARCHAR(100) DEFAULT '',
    spec_notes TEXT DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rfq_matches (
    rfq_id UUID NOT NULL REFERENCES rfqs(id) ON DELETE CASCADE,
    supplier_id UUID NOT NULL REFERENCES supplier_profiles(id) ON DELETE CASCADE,
    matched_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (rfq_id, supplier_id)
);

CREATE TABLE IF NOT EXISTS quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rfq_id UUID NOT NULL REFERENCES rfqs(id) ON DELETE CASCADE,
    supplier_id UUID NOT NULL REFERENCES supplier_profiles(id) ON DELETE CASCADE,
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(12, 2) NOT NULL CHECK (total_price >= 0),
    lead_time_days INT NOT NULL CHECK (lead_time_days > 0),
    risk_score NUMERIC(3, 2) NOT NULL CHECK (risk_score >= 0.0 AND risk_score <= 1.0),
    status VARCHAR(50) NOT NULL DEFAULT 'suggested' CHECK (status IN ('suggested', 'submitted', 'rejected', 'awarded')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE rfqs ADD CONSTRAINT fk_awarded_quote FOREIGN KEY (awarded_quote_id) REFERENCES quotes(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rfq_id UUID NOT NULL REFERENCES rfqs(id) ON DELETE CASCADE,
    quote_id UUID UNIQUE NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    score NUMERIC(5, 4) NOT NULL CHECK (score >= 0.0 AND score <= 1.0),
    rank INT NOT NULL CHECK (rank > 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
