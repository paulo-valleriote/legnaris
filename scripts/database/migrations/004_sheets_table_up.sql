CREATE TABLE IF NOT EXISTS sheets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_id UUID NOT NULL,
    template_id UUID NOT NULL,
    template_version_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    data JSONB NOT NULL,
    lifecycle_status VARCHAR(50) NOT NULL,
    deletion_scheduled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sheets_owner
        FOREIGN KEY (owner_id) REFERENCES users(id),
    CONSTRAINT fk_sheets_template
        FOREIGN KEY (template_id) REFERENCES sheet_templates(id),
    CONSTRAINT fk_sheets_template_version
        FOREIGN KEY (template_version_id) REFERENCES sheet_template_versions(id)
);
