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
  readonly viewerRole: CampaignRole;
  readonly viewerOwner: boolean;
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
    viewerRole: requireEnum(source, 'viewer_role', CAMPAIGN_ROLES),
    viewerOwner: requireBoolean(source, 'viewer_owner'),
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
