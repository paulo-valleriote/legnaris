import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react';
import type { ReactNode } from 'react';
import type { PlanUsage } from '../api/models/plan';
import { useAuthSession } from '../app/providers';
import { useCachedPlanUsage } from '../catalog/queries';
import type { AdsConfig } from './ads_config';
import type { ConsentState } from './consent';
import { loadCmp, watchConsent } from './consent';
import { loadAdScript, pushAd } from './loader';

export interface AdsContextValue {
  readonly config: AdsConfig | null;
  readonly signedIn: boolean;
  readonly plan: PlanUsage | undefined;
  readonly consent: ConsentState;
  readonly pushAdSlot: () => void;
  readonly registerSlot: () => () => void;
}

export interface AdsProviderProps {
  readonly config: AdsConfig | null;
  readonly children: ReactNode;
  readonly document?: Document;
  readonly window?: Window;
}

const AdsContext = createContext<AdsContextValue | undefined>(undefined);

export function useAds(): AdsContextValue {
  const ads = useContext(AdsContext);
  if (ads === undefined) {
    throw new Error('useAds must be used within <AdsProvider>.');
  }
  return ads;
}

export function AdsProvider({
  config,
  children,
  document: documentProp,
  window: windowProp,
}: AdsProviderProps) {
  const session = useAuthSession();
  const snapshot = useSyncExternalStore(session.subscribe, session.getSnapshot);
  const planQuery = useCachedPlanUsage();
  const [consent, setConsent] = useState<ConsentState>('pending');
  const [activeSlots, setActiveSlots] = useState(0);
  const activeCount = useRef(0);

  const adDocument = documentProp ?? document;
  const adWindow = windowProp ?? window;
  const signedIn = snapshot.state === 'authenticated';
  const client = config?.client ?? null;

  const registerSlot = useCallback(() => {
    activeCount.current += 1;
    setActiveSlots(activeCount.current);

    return () => {
      activeCount.current = Math.max(0, activeCount.current - 1);
      setActiveSlots(activeCount.current);
    };
  }, []);

  const pushAdSlot = useCallback(() => {
    pushAd(adWindow);
  }, [adWindow]);

  useEffect(() => {
    if (client === null || activeSlots === 0) {
      return;
    }

    let stop: (() => void) | undefined;
    let disposed = false;

    const start = () => {
      if (disposed) {
        return;
      }
      stop?.();
      stop = watchConsent(adWindow, setConsent);
    };

    const script = loadCmp(adDocument, client);
    start();
    script?.addEventListener('load', start);

    return () => {
      disposed = true;
      script?.removeEventListener('load', start);
      stop?.();
    };
  }, [activeSlots, client, adDocument, adWindow]);

  useEffect(() => {
    if (client === null || activeSlots === 0 || consent === 'pending') {
      return;
    }

    loadAdScript(adDocument, adWindow, client, consent);
  }, [activeSlots, client, consent, adDocument, adWindow]);

  const value = useMemo<AdsContextValue>(
    () => ({
      config,
      signedIn,
      plan: planQuery.data,
      consent,
      pushAdSlot,
      registerSlot,
    }),
    [config, signedIn, planQuery.data, consent, pushAdSlot, registerSlot],
  );

  return <AdsContext.Provider value={value}>{children}</AdsContext.Provider>;
}
