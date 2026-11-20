import { browserFetch } from '../api/transport';

export type HomeFunnelEvent = 'visit' | 'cta_click';

export interface FunnelSink {
  beacon(url: string, body: string): boolean;
  keepalive(url: string, body: string): void;
}

const FUNNEL_PATH = '/funnel/home';

function funnelUrl(baseUrl: string): string {
  return `${baseUrl.replace(/\/+$/, '')}${FUNNEL_PATH}`;
}

export function browserFunnelSink(): FunnelSink {
  return {
    beacon: (url, body) => {
      if (typeof navigator === 'undefined' || typeof navigator.sendBeacon !== 'function') {
        return false;
      }
      try {
        return navigator.sendBeacon(url, body);
      } catch {
        return false;
      }
    },
    keepalive: (url, body) => {
      try {
        void browserFetch(url, {
          method: 'POST',
          body,
          headers: { 'Content-Type': 'text/plain;charset=UTF-8' },
          credentials: 'omit',
          keepalive: true,
          mode: 'cors',
        }).catch(() => undefined);
      } catch {
        return;
      }
    },
  };
}

export function reportHomeFunnel(
  baseUrl: string,
  event: HomeFunnelEvent,
  sink: FunnelSink = browserFunnelSink(),
): void {
  const url = funnelUrl(baseUrl);
  const body = JSON.stringify({ event });

  if (!sink.beacon(url, body)) {
    sink.keepalive(url, body);
  }
}

export function reportCtaClick(baseUrl: string, sink: FunnelSink = browserFunnelSink()): void {
  reportHomeFunnel(baseUrl, 'cta_click', sink);
}

let homeVisitReported = false;

export function reportHomeVisit(baseUrl: string, sink: FunnelSink = browserFunnelSink()): void {
  if (homeVisitReported) {
    return;
  }
  homeVisitReported = true;
  reportHomeFunnel(baseUrl, 'visit', sink);
}

export function resetHomeVisitReport(): void {
  homeVisitReported = false;
}
