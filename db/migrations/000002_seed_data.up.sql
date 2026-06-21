-- Seed Buyer
INSERT INTO users (id, email, password_hash, role) VALUES 
('b1111111-1111-1111-1111-111111111111', 'buyer@auctom.com', '$2a$10$i4MXBhV8LzlKfUWTTKUpteaouMR9UVTUInHmEAGljQaemREAUF4l6', 'buyer')
ON CONFLICT (email) DO NOTHING;

-- Seed Supplier 1
INSERT INTO users (id, email, password_hash, role) VALUES 
('f1111111-1111-1111-1111-111111111111', 'supplier1@auctom.com', '$2a$10$i4MXBhV8LzlKfUWTTKUpteaouMR9UVTUInHmEAGljQaemREAUF4l6', 'supplier')
ON CONFLICT (email) DO NOTHING;

INSERT INTO supplier_profiles (id, user_id, company_name, capabilities, capacity_index, rating, base_lead_time, risk_score, auto_bid) VALUES
('51111111-1111-1111-1111-111111111111', 'f1111111-1111-1111-1111-111111111111', 'Apex CNC Machining', ARRAY['CNC Machining', 'Sheet Metal'], 80, 4.80, 5, 0.15, TRUE)
ON CONFLICT (user_id) DO NOTHING;

-- Seed Supplier 2
INSERT INTO users (id, email, password_hash, role) VALUES 
('f2222222-2222-2222-2222-222222222222', 'supplier2@auctom.com', '$2a$10$i4MXBhV8LzlKfUWTTKUpteaouMR9UVTUInHmEAGljQaemREAUF4l6', 'supplier')
ON CONFLICT (email) DO NOTHING;

INSERT INTO supplier_profiles (id, user_id, company_name, capabilities, capacity_index, rating, base_lead_time, risk_score, auto_bid) VALUES
('52222222-2222-2222-2222-222222222222', 'f2222222-2222-2222-2222-222222222222', 'Vertex 3D & Molding', ARRAY['3D Printing', 'Injection Molding'], 70, 4.50, 3, 0.10, FALSE)
ON CONFLICT (user_id) DO NOTHING;

-- Seed Supplier 3
INSERT INTO users (id, email, password_hash, role) VALUES 
('f3333333-3333-3333-3333-333333333333', 'supplier3@auctom.com', '$2a$10$i4MXBhV8LzlKfUWTTKUpteaouMR9UVTUInHmEAGljQaemREAUF4l6', 'supplier')
ON CONFLICT (email) DO NOTHING;

INSERT INTO supplier_profiles (id, user_id, company_name, capabilities, capacity_index, rating, base_lead_time, risk_score, auto_bid) VALUES
('53333333-3333-3333-3333-333333333333', 'f3333333-3333-3333-3333-333333333333', 'Global Castings', ARRAY['CNC Machining', 'Injection Molding'], 90, 4.20, 10, 0.30, TRUE)
ON CONFLICT (user_id) DO NOTHING;

-- Seed Supplier 4
INSERT INTO users (id, email, password_hash, role) VALUES 
('f4444444-4444-4444-4444-444444444444', 'supplier4@auctom.com', '$2a$10$i4MXBhV8LzlKfUWTTKUpteaouMR9UVTUInHmEAGljQaemREAUF4l6', 'supplier')
ON CONFLICT (email) DO NOTHING;

INSERT INTO supplier_profiles (id, user_id, company_name, capabilities, capacity_index, rating, base_lead_time, risk_score, auto_bid) VALUES
('54444444-4444-4444-4444-444444444444', 'f4444444-4444-4444-4444-444444444444', 'Rapid Sheet Metal', ARRAY['Sheet Metal'], 85, 4.90, 2, 0.05, TRUE)
ON CONFLICT (user_id) DO NOTHING;
