import type { CreateCampaignInput, MemberAction } from '../api/client';
import type { Campaign, CampaignMember, CampaignRole } from '../api/models/campaign';
import type { SheetKind } from '../api/models/plan';
import type { TemplateSummary } from '../api/models/template';

export const GM_POLL_INTERVAL_MS = 60_000;

export const PLAYER_SHEET_KINDS = ['character'] as const satisfies readonly SheetKind[];

export const GM_SHEET_KINDS = ['npc', 'enemy'] as const satisfies readonly SheetKind[];

export function creatableKinds(role: CampaignRole): readonly SheetKind[] {
  return role === 'gm' ? GM_SHEET_KINDS : PLAYER_SHEET_KINDS;
}

export function allowedKindsFor(
  kinds: readonly SheetKind[],
  role: CampaignRole,
): readonly SheetKind[] {
  const allowed = creatableKinds(role);
  return kinds.filter((kind) => allowed.includes(kind));
}

export function hasLinkedCharacter(
  sheets: readonly { readonly campaignId?: string }[],
  campaignId: string,
): boolean {
  return sheets.some((sheet) => sheet.campaignId === campaignId);
}

export type CampaignScopeOption =
  | { readonly key: string; readonly label: string; readonly gameSystemId: string }
  | { readonly key: string; readonly label: string; readonly templateId: string };

export function campaignScopeOptions(templates: readonly TemplateSummary[]): CampaignScopeOption[] {
  const options: CampaignScopeOption[] = [];
  const seen = new Set<string>();

  for (const template of templates) {
    const gameSystem = template.gameSystem;
    const option: CampaignScopeOption =
      gameSystem === undefined
        ? {
            key: `template:${template.id}`,
            label: template.name,
            templateId: template.id,
          }
        : {
            key: `game-system:${gameSystem.id}`,
            label: gameSystem.name,
            gameSystemId: gameSystem.id,
          };

    if (seen.has(option.key)) {
      continue;
    }
    seen.add(option.key);
    options.push(option);
  }

  return options;
}

export function createCampaignInput(
  name: string,
  option: CampaignScopeOption,
): CreateCampaignInput {
  return 'gameSystemId' in option
    ? { name, gameSystem: option.gameSystemId }
    : { name, templateId: option.templateId };
}

export function isDeleting(campaign: Pick<Campaign, 'status'>): boolean {
  return campaign.status === 'deleting';
}

export function canLinkCharacter(campaign: Pick<Campaign, 'viewerRole'>): boolean {
  return campaign.viewerRole === 'player';
}

export function canDeleteCampaign(campaign: Pick<Campaign, 'viewerOwner'>): boolean {
  return campaign.viewerOwner;
}

export function isSelf(
  viewer: Pick<Campaign, 'viewerUserId'>,
  member: Pick<CampaignMember, 'userId'>,
): boolean {
  return viewer.viewerUserId !== '' && viewer.viewerUserId === member.userId;
}

export function memberActions(
  viewer: Pick<Campaign, 'viewerRole' | 'viewerOwner' | 'viewerUserId'>,
  member: Pick<CampaignMember, 'role' | 'owner' | 'userId'>,
): readonly (MemberAction | 'remove')[] {
  if (member.owner || isSelf(viewer, member)) {
    return [];
  }
  if (viewer.viewerOwner) {
    return member.role === 'gm' ? ['demote', 'remove'] : ['promote', 'remove'];
  }
  if (viewer.viewerRole === 'gm' && member.role === 'player') {
    return ['remove'];
  }
  return [];
}

export function eligibleSheetIds(
  sheets: readonly { id: string; templateId: string; campaignId?: string }[],
  campaign: Pick<Campaign, 'id' | 'templateIds'>,
): string[] {
  const templateIds = new Set(campaign.templateIds);
  return sheets
    .filter((sheet) => templateIds.has(sheet.templateId))
    .filter((sheet) => sheet.campaignId === undefined || sheet.campaignId === campaign.id)
    .map((sheet) => sheet.id);
}

export function linkedCharacterSheet<
  T extends { readonly id: string; readonly campaignId?: string },
>(sheets: readonly T[], campaignId: string): T | undefined {
  return sheets.find((sheet) => sheet.campaignId === campaignId);
}

export function linkableSheetIds(
  sheets: readonly { id: string; templateId: string; campaignId?: string }[],
  campaign: Pick<Campaign, 'id' | 'templateIds'>,
): string[] {
  const linked = linkedCharacterSheet(sheets, campaign.id);
  return eligibleSheetIds(sheets, campaign).filter((id) => id !== linked?.id);
}

export function eligibleNpcSheetIds(
  sheets: readonly { id: string; templateId: string }[],
  campaign: Pick<Campaign, 'templateIds'>,
): string[] {
  const templateIds = new Set(campaign.templateIds);
  return sheets.filter((sheet) => templateIds.has(sheet.templateId)).map((sheet) => sheet.id);
}

export function unlinkedNpcSheetIds(
  sheets: readonly { id: string; templateId: string }[],
  campaign: Pick<Campaign, 'templateIds'>,
  linkedSheetIds: readonly string[],
): string[] {
  const linked = new Set(linkedSheetIds);
  return eligibleNpcSheetIds(sheets, campaign).filter((id) => !linked.has(id));
}

export function atOwnedLimit(usage: { readonly used: number; readonly limit: number }): boolean {
  return usage.used >= usage.limit;
}

export function memberUsername(
  campaign: Pick<Campaign, 'members'>,
  userId: string,
): string | undefined {
  const member = campaign.members.find((candidate) => candidate.userId === userId);
  return member === undefined || member.username === '' ? undefined : member.username;
}
