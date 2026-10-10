CREATE TABLE IF NOT EXISTS user_types (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    code VARCHAR(80) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    code VARCHAR(80) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id BIGSERIAL PRIMARY KEY,
    module VARCHAR(80) NOT NULL,
    name VARCHAR(120) NOT NULL,
    code VARCHAR(150) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS user_type_id BIGINT NULL REFERENCES user_types(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS role_id BIGINT NULL REFERENCES roles(id) ON DELETE SET NULL;

ALTER TABLE admin_users
    ADD COLUMN IF NOT EXISTS role_id BIGINT NULL REFERENCES roles(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_users_user_type_id ON users(user_type_id);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_admin_users_role_id ON admin_users(role_id);
CREATE INDEX IF NOT EXISTS idx_permissions_module ON permissions(module, name);

-- Preserve legacy administrator roles while allowing them to be managed in the RBAC UI.
INSERT INTO roles(name, code, description, is_system)
SELECT INITCAP(REPLACE(role, '_', ' ')), role, 'Migrated from the existing administrator role.', TRUE
FROM admin_users
WHERE BTRIM(role) <> ''
ON CONFLICT (code) DO NOTHING;

INSERT INTO roles(name, code, description, is_system) VALUES
    ('Super Admin','super_admin','Full platform access.',TRUE),
    ('Admin','admin','Platform administration.',TRUE),
    ('Support','support','Customer support access.',TRUE),
    ('Billing','billing','Plans and billing access.',TRUE),
    ('Content Manager','content_manager','Website and template content access.',TRUE),
    ('Read Only','read_only','Read-only platform access.',TRUE)
ON CONFLICT (code) DO NOTHING;

UPDATE admin_users a SET role_id = r.id
FROM roles r WHERE r.code = a.role AND a.role_id IS NULL;

INSERT INTO permissions(module,name,code,description) VALUES
('dashboard','View','dashboard.view','View the admin dashboard.'),
('notifications','View','notifications.view','View notification history.'),('notifications','Create','notifications.create','Create in-app notifications.'),
('users','View','users.view','View users.'),('users','Create','users.create','Create users.'),('users','Update','users.update','Update users.'),('users','Delete','users.delete','Delete users.'),
('organizations','View','organizations.view','View organizations.'),('organizations','Create','organizations.create','Create organizations.'),('organizations','Update','organizations.update','Update organizations.'),('organizations','Delete','organizations.delete','Delete organizations.'),
('cards','View','cards.view','View digital cards.'),('cards','Create','cards.create','Create digital cards.'),('cards','Update','cards.update','Update digital cards.'),('cards','Delete','cards.delete','Delete digital cards.'),
('contacts','View','contacts.view','View contacts.'),('contacts','Create','contacts.create','Create contacts.'),('contacts','Update','contacts.update','Update contacts.'),('contacts','Delete','contacts.delete','Delete contacts.'),
('analytics','View','analytics.view','View analytics.'),
('templates','View','templates.view','View templates.'),('templates','Create','templates.create','Create templates.'),('templates','Update','templates.update','Update templates.'),('templates','Delete','templates.delete','Delete templates.'),
('qr','View','qr.view','View QR codes.'),('qr','Create','qr.create','Create QR codes.'),('qr','Update','qr.update','Update QR codes.'),('qr','Delete','qr.delete','Delete QR codes.'),
('nfc','View','nfc.view','View NFC tags.'),('nfc','Create','nfc.create','Create NFC tags.'),('nfc','Update','nfc.update','Update NFC tags.'),('nfc','Delete','nfc.delete','Delete NFC tags.'),
('billing','View','billing.view','View plans and billing.'),('billing','Manage','billing.manage','Manage plans and subscriptions.'),
('website','View','website.view','View website configuration.'),('website','Manage','website.manage','Manage website configuration.'),
('security','View','security.view','View security settings and audits.'),('security','Manage','security.manage','Manage security settings and administrator accounts.'),
('settings','View','settings.view','View platform settings.'),('settings','Manage','settings.manage','Manage platform settings.'),
('administration','View','administration.view','View Administration records.'),('administration','Manage','administration.manage','Manage Administration records and role permissions.')
ON CONFLICT (code) DO NOTHING;

-- Keep the legacy admin role's previous full-access behavior. Other legacy roles
-- receive explicit, least-privilege module grants.
INSERT INTO role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p WHERE r.code IN ('super_admin','admin')
ON CONFLICT DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON
    (r.code='support' AND p.code IN ('dashboard.view','users.view','organizations.view','cards.view','contacts.view','contacts.update','analytics.view','qr.view','nfc.view')) OR
    (r.code='billing' AND p.code IN ('dashboard.view','billing.view','billing.manage')) OR
    (r.code='content_manager' AND p.module IN ('dashboard','website','templates')) OR
    (r.code='read_only' AND p.name='View')
ON CONFLICT DO NOTHING;
