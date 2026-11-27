import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { InlineEdit } from '../sheet/inline_edit';
import goldenCharacter from '../../../testdata/templates/fifth_character_v1.json';
import { setViewportWidth } from './match_media';

const TEXT_SCHEMA = {
  type: 'text',
  label: 'Name',
  isEditable: true,
  defaultValue: 0,
  maxLength: 60,
};

afterEach(() => {
  setViewportWidth(1280);
});

function renderInHeading() {
  const onCommit = vi.fn();
  render(
    <h1>
      <InlineEdit label="Name" value="Brenna" schema={TEXT_SCHEMA} onCommit={onCommit} />
    </h1>,
  );
  return onCommit;
}

describe('the inline editor', () => {
  it('Edge: the phone dialog is drawn on the document body, never inside its heading', async () => {
    setViewportWidth(390);
    renderInHeading();

    await userEvent.click(screen.getByRole('button', { name: 'Name, Brenna' }));

    const dialog = screen.getByRole('dialog');
    expect(dialog.closest('h1')).toBeNull();
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveAttribute('maxlength', '60');
  });

  it('Edge: the wide editor is capped at the served maximum length', async () => {
    setViewportWidth(1280);
    renderInHeading();

    await userEvent.click(screen.getByRole('button', { name: 'Name, Brenna' }));

    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveAttribute('maxlength', '60');
  });

  it('Happy path: the phone dialog offers Save and commits without a Return key', async () => {
    setViewportWidth(390);
    const onCommit = renderInHeading();

    await userEvent.click(screen.getByRole('button', { name: 'Name, Brenna' }));
    const input = screen.getByRole('textbox', { name: 'Name' });
    await userEvent.clear(input);
    await userEvent.type(input, 'Ysolde');
    await userEvent.click(screen.getByRole('button', { name: message('common.save') }));

    expect(onCommit).toHaveBeenCalledWith('Ysolde');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Edge: Cancel closes the phone dialog and discards the draft', async () => {
    setViewportWidth(390);
    const onCommit = renderInHeading();

    await userEvent.click(screen.getByRole('button', { name: 'Name, Brenna' }));
    await userEvent.type(screen.getByRole('textbox', { name: 'Name' }), 'x');
    await userEvent.click(screen.getByRole('button', { name: message('common.cancel') }));

    expect(onCommit).not.toHaveBeenCalled();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('Happy path: the portalled dialog still commits what was typed', async () => {
    setViewportWidth(390);
    const onCommit = renderInHeading();

    await userEvent.click(screen.getByRole('button', { name: 'Name, Brenna' }));
    const input = screen.getByRole('textbox', { name: 'Name' });
    await userEvent.clear(input);
    await userEvent.type(input, 'Ysolde{Enter}');

    expect(onCommit).toHaveBeenCalledWith('Ysolde');
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});

describe('the served maximum length', () => {
  it('Happy path: a template text leaf keeps its max_length', () => {
    const template = parseTemplateDetail(goldenCharacter.data);
    const identity = template.fields['identity'];
    const subclass = identity?.kind === 'group' ? identity.children['subclass'] : undefined;

    expect(subclass?.kind === 'leaf' ? subclass.schema.maxLength : undefined).toBe(60);
  });

  it('Edge: the refused-edit copy says the value was not saved', () => {
    expect(message('api.error.invalidEdit')).toContain('was not saved');
  });
});
