-- GoBot initial schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Organizations
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    domain TEXT UNIQUE,
    workspace_type TEXT CHECK (workspace_type IN ('google', 'microsoft', 'none')),
    workspace_admin_email TEXT,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Human users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'manager', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Departments
CREATE TABLE departments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, name)
);

-- Agents (digital employees)
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    department_id UUID REFERENCES departments(id) ON DELETE SET NULL,

    -- Identity
    name TEXT NOT NULL,
    title TEXT,
    email TEXT,
    phone_number TEXT,
    whatsapp_number TEXT,

    -- Status
    status TEXT NOT NULL DEFAULT 'inactive' CHECK (status IN ('inactive', 'active', 'paused', 'error')),

    -- LLM configuration
    llm_provider TEXT NOT NULL DEFAULT 'claude',
    llm_model TEXT NOT NULL DEFAULT 'claude-sonnet-4-5-20250929',
    system_prompt TEXT,
    temperature FLOAT DEFAULT 0.7,

    -- Scope & permissions
    permissions JSONB DEFAULT '{}',
    scope_description TEXT,
    max_autonomy_level TEXT NOT NULL DEFAULT 'suggest' CHECK (max_autonomy_level IN ('suggest', 'act_with_approval', 'fully_autonomous')),

    -- Reporting
    reports_to_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reports_to_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hierarchy relationships (flexible many-to-many)
CREATE TABLE agent_hierarchy (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_type TEXT NOT NULL CHECK (parent_type IN ('user', 'agent')),
    parent_id UUID NOT NULL,
    child_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    relationship TEXT NOT NULL DEFAULT 'reports_to' CHECK (relationship IN ('reports_to', 'delegates_to', 'escalates_to')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(parent_type, parent_id, child_agent_id, relationship)
);

-- Indexes
CREATE INDEX idx_agents_org_id ON agents(org_id);
CREATE INDEX idx_agents_department_id ON agents(department_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_users_org_id ON users(org_id);
CREATE INDEX idx_departments_org_id ON departments(org_id);
CREATE INDEX idx_hierarchy_parent ON agent_hierarchy(parent_type, parent_id);
CREATE INDEX idx_hierarchy_child ON agent_hierarchy(child_agent_id);
