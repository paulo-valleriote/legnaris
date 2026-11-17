import type { MemberAction } from '../api/client';
import type { Campaign, CampaignMember } from '../api/models/campaign';

export const GM_POLL_INTERVAL_MS = 60_000;

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
  campaign: Pick<Campaign, 'id' | 'templateId'>,
): string[] {
  return sheets
    .filter((sheet) => sheet.templateId === campaign.templateId)
    .filter((sheet) => sheet.campaignId === undefined || sheet.campaignId === campaign.id)
    .map((sheet) => sheet.id);
}

export function eligibleNpcSheetIds(
  sheets: readonly { id: string; templateId: string }[],
  campaign: Pick<Campaign, 'templateId'>,
): string[] {
  return sheets
    .filter((sheet) => sheet.templateId === campaign.templateId)
    .map((sheet) => sheet.id);
}

export function unlinkedNpcSheetIds(
  sheets: readonly { id: string; templateId: string }[],
  campaign: Pick<Campaign, 'templateId'>,
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
