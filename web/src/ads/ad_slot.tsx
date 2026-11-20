import { useEffect, useRef } from 'react';
import { message } from '../l10n/strings';
import type { AdSurface } from './ads_config';
import { useAds } from './ads_provider';
import { adsEligible } from './eligibility';
import './ads.css';

export interface AdSlotProps {
  readonly surface: AdSurface;
}

export function AdSlot({ surface }: AdSlotProps) {
  const { config, signedIn, plan, consent, pushAdSlot, registerSlot } = useAds();
  const slotId = config?.slots[surface];
  const eligible = adsEligible({ config, signedIn, plan });
  const active = eligible && slotId !== undefined;
  const pushed = useRef(false);

  useEffect(() => {
    if (!active) {
      return;
    }
    return registerSlot();
  }, [active, registerSlot]);

  useEffect(() => {
    if (!active) {
      pushed.current = false;
      return;
    }
    if (consent === 'pending' || pushed.current) {
      return;
    }
    pushed.current = true;
    pushAdSlot();
  }, [active, consent, pushAdSlot]);

  if (!active || config === null || slotId === undefined) {
    return null;
  }

  return (
    <aside className="lg-ad-slot" aria-label={message('ads.label')}>
      <span className="lg-ad-slot__caption" aria-hidden="true">
        {message('ads.label')}
      </span>
      <ins
        className="adsbygoogle lg-ad-slot__unit"
        data-ad-client={config.client}
        data-ad-slot={slotId}
        data-ad-format="horizontal"
        data-full-width-responsive="false"
      />
    </aside>
  );
}
