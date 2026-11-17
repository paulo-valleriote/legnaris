import { drift, isPlainObject } from '../parse';

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
