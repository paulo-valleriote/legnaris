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

export function memberActions(
  viewer: Pick<Campaign, 'viewerRole' | 'viewerOwner'>,
  member: Pick<CampaignMember, 'role' | 'owner'>,
): readonly (MemberAction | 'remove')[] {
  if (member.owner) {
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

export function atOwnedLimit(usage: { readonly used: number; readonly limit: number }): boolean {
  return usage.used >= usage.limit;
}
