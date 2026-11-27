import {
  drift,
  isPlainObject,
  optionalString,
  requireBoolean,
  requireNumber,
  requireString,
} from '../parse';
import type { Sheet } from './sheet';

export const CAMPAIGN_ROLES = ['gm', 'player'] as const;

export type CampaignRole = (typeof CAMPAIGN_ROLES)[number];

export const CAMPAIGN_STATUSES = ['active', 'deleting'] as const;

export type CampaignStatus = (typeof CAMPAIGN_STATUSES)[number];

export interface CampaignMember {
  readonly userId: string;
  readonly username: string;
  readonly role: CampaignRole;
  readonly owner: boolean;
  readonly joinedAt: string;
  readonly characterName: string;
}

export interface CampaignGameSystem {
  readonly id: string;
  readonly name: string;
}

export interface Campaign {
  readonly id: string;
  readonly name: string;
  readonly scopeName: string;
  readonly templateIds: readonly string[];
  readonly templateId: string | undefined;
  readonly gameSystem: CampaignGameSystem | undefined;
  readonly status: CampaignStatus;
  readonly version: number;
  readonly gmCount: number;
  readonly playerCount: number;
  readonly viewerUserId: string;
  readonly viewerRole: CampaignRole;
  readonly viewerOwner: boolean;
  readonly inviteCode: string | undefined;
  readonly passwordSet: boolean | undefined;
  readonly members: readonly CampaignMember[];
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface CampaignSummary {
  readonly id: string;
  readonly name: string;
  readonly scopeName: string;
  readonly templateIds: readonly string[];
  readonly templateId: string | undefined;
  readonly gameSystem: CampaignGameSystem | undefined;
  readonly version: number;
  readonly viewerRole: CampaignRole;
  readonly viewerOwner: boolean;
  readonly updatedAt: string;
}

export interface CampaignSheet {
  readonly id: string;
  readonly name: string;
  readonly templateId: string;
  readonly ownerId: string;
  readonly updatedAt: string;
}

export const LINKABLE_SHEET_KINDS = ['npc', 'enemy'] as const;

export type NpcKind = (typeof LINKABLE_SHEET_KINDS)[number];

export interface NpcLink {
  readonly id: string;
  readonly sheetId: string;
  readonly name: string;
  readonly kind: NpcKind;
  readonly ownerId: string;
  readonly ownerName: string;
  readonly revealed: boolean;
  readonly linkedAt: string;
  readonly isOwner: boolean;
}

export type CampaignSheetResult =
  | { readonly notModified: true }
  | { readonly notModified: false; readonly sheet: Sheet; readonly etag: string | undefined };

function requireObject(payload: unknown, what: string): Record<string, unknown> {
  if (!isPlainObject(payload)) {
    throw drift(`${what} is not an object`);
  }
  return payload;
}

function requireEnum<T extends string>(
  source: Record<string, unknown>,
  key: string,
  values: readonly T[],
): T {
  const value = requireString(source, key);
  const match = values.find((candidate) => candidate === value);
  if (match === undefined) {
    throw drift(`"${key}" is not a known value`);
  }
  return match;
}

function optionalText(source: Record<string, unknown>, key: string): string {
  const value = source[key];
  return typeof value === 'string' ? value : '';
}

function parseGameSystem(source: Record<string, unknown>): CampaignGameSystem | undefined {
  const value = source['game_system'];
  if (value === undefined || value === null) {
    return undefined;
  }

  const gameSystem = requireObject(value, 'campaign game system');
  return {
    id: requireString(gameSystem, 'id'),
    name: requireString(gameSystem, 'name'),
  };
}

function parseTemplateIds(source: Record<string, unknown>): readonly string[] | undefined {
  const value = source['template_ids'];
  if (value === undefined || value === null) {
    return undefined;
  }

  if (!Array.isArray(value)) {
    throw drift('campaign "template_ids" is not a list');
  }

  const ids: string[] = [];
  for (const entry of value) {
    if (typeof entry !== 'string') {
      throw drift('campaign "template_ids" holds a non-string entry');
    }
    ids.push(entry);
  }
  return ids;
}

interface CampaignScope {
  readonly scopeName: string;
  readonly templateIds: readonly string[];
  readonly templateId: string | undefined;
  readonly gameSystem: CampaignGameSystem | undefined;
}

function parseScope(source: Record<string, unknown>): CampaignScope {
  const templateId = optionalString(source, 'template_id');
  const gameSystem = parseGameSystem(source);

  if (templateId === undefined && gameSystem === undefined) {
    throw drift('campaign payload has neither a game system nor a template id');
  }

  const scopeName = optionalString(source, 'scope_name') ?? templateId ?? gameSystem?.name ?? '';
  const templateIds = parseTemplateIds(source) ?? (templateId === undefined ? [] : [templateId]);

  return { scopeName, templateIds, templateId, gameSystem };
}

function parseMember(payload: unknown): CampaignMember {
  const source = requireObject(payload, 'campaign member');
  return {
    userId: requireString(source, 'user_id'),
    username: optionalText(source, 'username'),
    role: requireEnum(source, 'role', CAMPAIGN_ROLES),
    owner: requireBoolean(source, 'owner'),
    joinedAt: requireString(source, 'joined_at'),
    characterName: optionalText(source, 'character_name'),
  };
}

export function parseCampaign(payload: unknown): Campaign {
  const source = requireObject(payload, 'campaign payload');
  const rawMembers = source['members'];
  if (!Array.isArray(rawMembers)) {
    throw drift('campaign payload is missing a members list');
  }

  return {
    id: requireString(source, 'id'),
    name: requireString(source, 'name'),
    ...parseScope(source),
    status: requireEnum(source, 'status', CAMPAIGN_STATUSES),
    version: requireNumber(source, 'version'),
    gmCount: requireNumber(source, 'gm_count'),
    playerCount: requireNumber(source, 'player_count'),
    viewerUserId: optionalText(source, 'viewer_user_id'),
    viewerRole: requireEnum(source, 'viewer_role', CAMPAIGN_ROLES),
    viewerOwner: requireBoolean(source, 'viewer_owner'),
    inviteCode: typeof source['invite_code'] === 'string' ? source['invite_code'] : undefined,
    passwordSet: typeof source['password_set'] === 'boolean' ? source['password_set'] : undefined,
    members: rawMembers.map(parseMember),
    createdAt: requireString(source, 'created_at'),
    updatedAt: requireString(source, 'updated_at'),
  };
}

export function parseJoinedCampaign(payload: unknown): Campaign {
  const campaign = parseCampaign(payload);
  if (campaign.id === '') {
    throw drift('campaign answer is missing a string id');
  }
  return campaign;
}

export interface CampaignInvite {
  readonly name: string;
  readonly gmUsername: string;
  readonly gameSystem: CampaignGameSystem | undefined;
  readonly templateId: string | undefined;
  readonly scopeName: string;
  readonly templateIds: readonly string[];
  readonly passwordRequired: boolean;
  readonly playerCount: number;
  readonly alreadyMember: boolean;
}

export function parseCampaignInvite(payload: unknown): CampaignInvite {
  const source = requireObject(payload, 'campaign invite');
  const templateId = source['template_id'];
  return {
    name: requireString(source, 'name'),
    gmUsername: optionalText(source, 'gm_username'),
    gameSystem: parseGameSystem(source),
    templateId: typeof templateId === 'string' && templateId !== '' ? templateId : undefined,
    scopeName: requireString(source, 'scope_name'),
    templateIds: parseTemplateIds(source) ?? [],
    passwordRequired: requireBoolean(source, 'password_required'),
    playerCount: requireNumber(source, 'player_count'),
    alreadyMember: requireBoolean(source, 'already_member'),
  };
}

export function parseCampaignSummaries(payload: unknown): CampaignSummary[] {
  if (!Array.isArray(payload)) {
    throw drift('campaign list is not an array');
  }

  return payload.map((entry) => {
    const source = requireObject(entry, 'campaign summary');
    return {
      id: requireString(source, 'id'),
      name: requireString(source, 'name'),
      ...parseScope(source),
      version: requireNumber(source, 'version'),
      viewerRole: requireEnum(source, 'viewer_role', CAMPAIGN_ROLES),
      viewerOwner: requireBoolean(source, 'viewer_owner'),
      updatedAt: requireString(source, 'updated_at'),
    };
  });
}

export function parseCampaignSheets(payload: unknown): CampaignSheet[] {
  if (!Array.isArray(payload)) {
    throw drift('campaign sheet list is not an array');
  }

  return payload.map((entry) => {
    const source = requireObject(entry, 'campaign sheet');
    return {
      id: requireString(source, 'id'),
      name: requireString(source, 'name'),
      templateId: requireString(source, 'template_id'),
      ownerId: requireString(source, 'owner_id'),
      updatedAt: requireString(source, 'updated_at'),
    };
  });
}

export function parseNpcLink(payload: unknown): NpcLink {
  const source = requireObject(payload, 'npc link');

  return {
    id: requireString(source, 'id'),
    sheetId: requireString(source, 'sheet_id'),
    name: requireString(source, 'name'),
    kind: requireEnum(source, 'kind', LINKABLE_SHEET_KINDS),
    ownerId: requireString(source, 'owner_id'),
    ownerName: optionalText(source, 'owner_name'),
    revealed: requireBoolean(source, 'revealed'),
    linkedAt: requireString(source, 'linked_at'),
    isOwner: requireBoolean(source, 'is_owner'),
  };
}

export function parseNpcLinks(payload: unknown): NpcLink[] {
  if (!Array.isArray(payload)) {
    throw drift('npc link list is not an array');
  }

  return payload.map(parseNpcLink);
}
