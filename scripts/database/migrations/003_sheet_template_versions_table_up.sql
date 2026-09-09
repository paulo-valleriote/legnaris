CREATE TABLE IF NOT EXISTS sheet_template_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL,
    version_number INT NOT NULL,
    schema_version INT NOT NULL,
    schema JSONB NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sheet_template_versions_template
        FOREIGN KEY (template_id) REFERENCES sheet_templates(id),
    CONSTRAINT uq_sheet_template_versions_number
        UNIQUE (template_id, version_number)
);

ALTER TABLE sheet_templates
    ADD CONSTRAINT fk_sheet_templates_current_version
    FOREIGN KEY (current_version_id) REFERENCES sheet_template_versions(id);
