import { describe, expect, it } from 'vitest';
import type { TemplateDetail } from '../api/models/template';
import { presetFromConfig } from '../sheet/preset/compose/adapter';
import { presetFor } from '../sheet/preset/registry';
import { PRESET_TEMPLATE_ID, presetConfig, presetTemplateDetail } from './preset_fixtures';

function templateWithId(id: string): TemplateDetail {
  return { ...presetTemplateDetail(), id };
}

describe('the preset registry', () => {
  it('Edge: a Template without a registered config keeps the generic form', () => {
    expect(presetFor(templateWithId('fantasy_v1'))).toBeUndefined();
  });

  it('Happy path: a registered config resolves by its Template id', () => {
    const config = presetConfig();
    const preset = presetFor(templateWithId(PRESET_TEMPLATE_ID), [config]);

    expect(preset).toBeDefined();
    expect(preset?.templateId).toBe(PRESET_TEMPLATE_ID);
    expect(preset?.sections.map((section) => section.key)).toEqual(
      config.pieces.map((piece) => piece.key),
    );
  });

  it('Edge: a config registered for another Template id does not leak', () => {
    const other = presetConfig();

    expect(presetFor(templateWithId('fantasy_v1'), [other])).toBeUndefined();
  });

  it('Edge: an unknown Template id returns undefined when no config is registered', () => {
    expect(presetFor(templateWithId('ghost_v1'))).toBeUndefined();
  });

  it('Happy path: the default registry resolves both fifth-edition configs', () => {
    expect(presetFor(templateWithId('fifth_character_v1'))?.templateId).toBe('fifth_character_v1');
    expect(presetFor(templateWithId('fifth_stat_block_v1'))?.templateId).toBe(
      'fifth_stat_block_v1',
    );
  });
});

describe('presetFromConfig', () => {
  it('Happy path: the adapter keeps the Template id, the section order and the layout blocks', () => {
    const layout = [{ sections: ['vitals'] }] as const;
    const narrowLayout = [{ sections: ['charges'] }] as const;
    const config = presetConfig({
      layout: [...layout],
      narrowLayout: [...narrowLayout],
      sectionNav: { after: 'vitals' },
    });

    const preset = presetFromConfig(config);

    expect(preset.templateId).toBe(PRESET_TEMPLATE_ID);
    expect(preset.Header).toBeTypeOf('function');
    expect(preset.sections.map((section) => section.key)).toEqual(['vitals', 'charges']);
    expect(preset.layout).toEqual(layout);
    expect(preset.narrowLayout).toEqual(narrowLayout);
    expect(preset.sectionNav).toEqual({ after: 'vitals' });
  });

  it('Edge: optional layout blocks are absent when the config omits them', () => {
    const preset = presetFromConfig(presetConfig());

    expect(preset.layout).toBeUndefined();
    expect(preset.narrowLayout).toBeUndefined();
    expect(preset.sectionNav).toBeUndefined();
  });
});
