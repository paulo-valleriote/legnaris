import { drift, isPlainObject, requireBoolean, requireNumber, requireString } from '../parse';
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

export interface Campaign {
  readonly id: string;
  readonly name: string;
  readonly templateId: string;
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
  readonly templateId: string;
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

export const LINKABLE_SHEET_KINDS = ['npc', 'monster'] as const;

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
    templateId: requireString(source, 'template_id'),
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

export function parseCampaignSummaries(payload: unknown): CampaignSummary[] {
  if (!Array.isArray(payload)) {
    throw drift('campaign list is not an array');
  }

  return payload.map((entry) => {
    const source = requireObject(entry, 'campaign summary');
    return {
      id: requireString(source, 'id'),
      name: requireString(source, 'name'),
      templateId: requireString(source, 'template_id'),
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

export interface JoinedCampaign {
  readonly id: string;
  readonly name: string;
}

export function parseJoinedCampaign(data: unknown): JoinedCampaign {
  if (!isPlainObject(data)) {
    throw drift('campaign answer is not an object');
  }

  const id = data['id'];
  if (typeof id !== 'string' || id === '') {
    throw drift('campaign answer is missing a string id');
  }

  const name = data['name'];
  if (typeof name !== 'string') {
    throw drift('campaign answer is missing a string name');
  }

  return { id, name };
}
