import { describe, expect, it, vi } from 'vitest';
import { reportCtaClick, reportHomeFunnel } from '../landing/funnel_beacon';
import type { FunnelSink } from '../landing/funnel_beacon';

function fakeSink(beaconResult: boolean) {
  return {
    beacon: vi.fn(() => beaconResult),
    keepalive: vi.fn(),
  } satisfies FunnelSink;
}

describe('home funnel beacon', () => {
  it('Happy path: a visit posts its event to the funnel endpoint', () => {
    const sink = fakeSink(true);

    reportHomeFunnel('http://api.test', 'visit', sink);

    expect(sink.beacon).toHaveBeenCalledWith('http://api.test/funnel/home', '{"event":"visit"}');
    expect(sink.keepalive).not.toHaveBeenCalled();
  });

  it('Happy path: a hero CTA click posts its event to the funnel endpoint', () => {
    const sink = fakeSink(true);

    reportHomeFunnel('http://api.test', 'cta_click', sink);

    expect(sink.beacon).toHaveBeenCalledWith(
      'http://api.test/funnel/home',
      '{"event":"cta_click"}',
    );
    expect(sink.keepalive).not.toHaveBeenCalled();
  });

  it('Happy path: reportCtaClick sends a click event through the same sink', () => {
    const sink = fakeSink(true);

    reportCtaClick('http://api.test', sink);

    expect(sink.beacon).toHaveBeenCalledWith(
      'http://api.test/funnel/home',
      '{"event":"cta_click"}',
    );
    expect(sink.keepalive).not.toHaveBeenCalled();
  });

  it('Edge: trailing slashes on the base address never double the separator', () => {
    const sink = fakeSink(true);

    reportHomeFunnel('http://api.test///', 'visit', sink);

    expect(sink.beacon).toHaveBeenCalledWith('http://api.test/funnel/home', '{"event":"visit"}');
  });

  it('Edge: a refused beacon falls back to a keepalive request', () => {
    const sink = fakeSink(false);

    reportHomeFunnel('http://api.test', 'visit', sink);

    expect(sink.keepalive).toHaveBeenCalledWith('http://api.test/funnel/home', '{"event":"visit"}');
  });
});
