ALTER TABLE IF EXISTS sheet_templates
    DROP CONSTRAINT IF EXISTS fk_sheet_templates_current_version;

DROP TABLE IF EXISTS sheet_template_versions;
