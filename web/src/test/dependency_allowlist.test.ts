import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const here = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(here, '../..');

interface AllowEntry {
  readonly name: string;
  readonly version: string;
}

interface AllowList {
  readonly version: number;
  readonly allow: readonly AllowEntry[];
  readonly deny: Readonly<Record<string, readonly string[]>>;
}

interface PackageJson {
  readonly dependencies?: Readonly<Record<string, string>>;
  readonly devDependencies?: Readonly<Record<string, string>>;
}

interface Lockfile {
  readonly packages: Readonly<Record<string, { readonly version?: string }>>;
}

const allowList = JSON.parse(
  readFileSync(path.join(webRoot, 'dependency-allowlist.json'), 'utf8'),
) as AllowList;
const packageJson = JSON.parse(
  readFileSync(path.join(webRoot, 'package.json'), 'utf8'),
) as PackageJson;
const lockfile = JSON.parse(
  readFileSync(path.join(webRoot, 'package-lock.json'), 'utf8'),
) as Lockfile;

const declared: Readonly<Record<string, string>> = {
  ...packageJson.dependencies,
  ...packageJson.devDependencies,
};

const EXACT_VERSION = /^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/;

const RULE_DEPENDENCY =
  'web/AGENTS.md "Dependencies": no new package without strict necessity, an exact pin, and a justification in the PR description (LGN-294).';
const RULE_STATE =
  'web/AGENTS.md "Standing decisions": no state library, per-path subscriptions use useSyncExternalStore (AD-001, LGN-294).';
const RULE_COMPONENTS =
  'web/AGENTS.md "Dependencies": design-system dependencies (component libraries) are prohibited; the tokens plus plain CSS are sufficient (LGN-294).';
const RULE_FONTS =
  'web/AGENTS.md "Dependencies": design-system dependencies (font packages) are prohibited; the font is vendored, not depended on (LGN-294).';

const RULE_BY_GROUP: Readonly<Record<string, string>> = {
  state: RULE_STATE,
  components: RULE_COMPONENTS,
  fonts: RULE_FONTS,
};

const REQUIRED_DENY: Readonly<Record<string, readonly string[]>> = {
  state: ['zustand', 'redux', '@reduxjs/*', 'mobx', 'jotai', 'recoil', 'valtio'],
  components: [
    '@mui/*',
    'antd',
    '@chakra-ui/*',
    '@mantine/*',
    'react-bootstrap',
    '@radix-ui/*',
    'shadcn',
  ],
  fonts: ['@fontsource/*'],
};

const allowedVersion = new Map(allowList.allow.map((entry) => [entry.name, entry.version]));

function deniedBy(name: string): { group: string; pattern: string } | undefined {
  for (const [group, patterns] of Object.entries(allowList.deny)) {
    for (const pattern of patterns) {
      const matches = pattern.endsWith('/*')
        ? name.startsWith(pattern.slice(0, -1))
        : name === pattern;
      if (matches) {
        return { group, pattern };
      }
    }
  }
  return undefined;
}

function installedVersion(name: string): string | undefined {
  return lockfile.packages[`node_modules/${name}`]?.version;
}

function audit(
  deps: Readonly<Record<string, string>>,
  installedFor: (name: string) => string | undefined = installedVersion,
): string[] {
  const violations: string[] = [];

  for (const [name, version] of Object.entries(deps)) {
    const denial = deniedBy(name);
    if (denial !== undefined) {
      violations.push(
        `"${name}" matches the denied pattern "${denial.pattern}". ${RULE_BY_GROUP[denial.group] ?? RULE_DEPENDENCY}`,
      );
      continue;
    }

    if (!EXACT_VERSION.test(version)) {
      violations.push(`"${name}" is "${version}", not an exact pin. ${RULE_DEPENDENCY}`);
      continue;
    }

    const pinned = allowedVersion.get(name);
    if (pinned === undefined) {
      violations.push(`"${name}" is not in dependency-allowlist.json. ${RULE_DEPENDENCY}`);
      continue;
    } else if (pinned !== version) {
      violations.push(
        `"${name}" is pinned to "${version}" but the allow-list allows "${pinned}". ${RULE_DEPENDENCY}`,
      );
      continue;
    }

    const installed = installedFor(name);
    if (installed !== version) {
      violations.push(
        `"${name}" is pinned to "${version}" but package-lock installs "${installed ?? 'nothing'}". ${RULE_DEPENDENCY}`,
      );
    }
  }

  for (const entry of allowList.allow) {
    if (!Object.hasOwn(deps, entry.name)) {
      violations.push(
        `dependency-allowlist.json allows "${entry.name}", which package.json does not declare. ${RULE_DEPENDENCY}`,
      );
    }
  }

  return violations;
}

describe('the versioned dependency allow-list (LGN-294)', () => {
  it('Happy path: the allow-list is versioned and covers every declared dependency at its installed pin', () => {
    expect(allowList.version, RULE_DEPENDENCY).toBe(1);
    expect(Object.keys(declared).length).toBeGreaterThan(0);
    expect(audit(declared), RULE_DEPENDENCY).toEqual([]);
  });

  it('Edge: every declared dependency resolves to its own allow-list entry, exactly once', () => {
    const names = allowList.allow.map((entry) => entry.name);
    expect(new Set(names).size, RULE_DEPENDENCY).toBe(names.length);
    for (const name of Object.keys(declared)) {
      expect(allowedVersion.has(name), `"${name}" must be allow-listed. ${RULE_DEPENDENCY}`).toBe(
        true,
      );
    }
  });

  it('Edge: the deny-list names the state, component and font families it forbids', () => {
    for (const [group, patterns] of Object.entries(REQUIRED_DENY)) {
      for (const pattern of patterns) {
        expect(
          allowList.deny[group],
          `the ${group} deny-list must still forbid "${pattern}". ${RULE_BY_GROUP[group] ?? RULE_DEPENDENCY}`,
        ).toContain(pattern);
      }
    }
  });

  it('Error: a package missing from the allow-list is reported with the dependency rule cited', () => {
    const violations = audit({ ...declared, 'left-pad': '1.3.0' });
    expect(violations).toHaveLength(1);
    expect(violations[0]).toContain('"left-pad" is not in dependency-allowlist.json');
    expect(violations[0]).toContain(RULE_DEPENDENCY);
  });

  it('Error: a caret, tilde, wildcard or comparator range is refused with the dependency rule cited', () => {
    const ranges = ['^1.2.3', '~1.2.3', '*', '1.x', '>1.0.0', '<2.0.0', '>=1.0.0', 'latest'];

    for (const version of ranges) {
      const violations = audit({ ...declared, react: version });
      expect(violations, `"${version}" must not pass as an exact pin`).toHaveLength(1);
      expect(violations[0]).toContain('not an exact pin');
      expect(violations[0]).toContain(RULE_DEPENDENCY);
    }
  });

  it('Error: a pin that drifts from the allow-list or the lockfile is refused', () => {
    const drifted = audit({ ...declared, react: '19.3.2' });
    expect(drifted).toHaveLength(1);
    expect(drifted[0]).toContain('the allow-list allows "19.3.1"');

    const unlocked = audit(declared, (name) =>
      name === 'react' ? '19.3.2' : installedVersion(name),
    );
    expect(unlocked).toHaveLength(1);
    expect(unlocked[0]).toContain('package-lock installs "19.3.2"');
  });

  it('Error: a state library is refused with the standing-decision rule cited', () => {
    for (const name of [
      'zustand',
      'redux',
      '@reduxjs/toolkit',
      'mobx',
      'jotai',
      'recoil',
      'valtio',
    ]) {
      const violations = audit({ ...declared, [name]: '1.0.0' });
      expect(violations[0], `${name} must be denied`).toContain('matches the denied pattern');
      expect(violations[0]).toContain(RULE_STATE);
    }
  });

  it('Error: a component library is refused with the dependency rule cited', () => {
    for (const name of [
      '@mui/material',
      'antd',
      '@chakra-ui/react',
      '@mantine/core',
      'react-bootstrap',
      '@radix-ui/react-dialog',
      'shadcn',
    ]) {
      const violations = audit({ ...declared, [name]: '1.0.0' });
      expect(violations[0], `${name} must be denied`).toContain('matches the denied pattern');
      expect(violations[0]).toContain(RULE_COMPONENTS);
    }
  });

  it('Error: a font package is refused with the dependency rule cited', () => {
    const violations = audit({ ...declared, '@fontsource/inter': '5.0.0' });
    expect(violations[0]).toContain('matches the denied pattern');
    expect(violations[0]).toContain(RULE_FONTS);
  });

  it('Edge: an allow-list entry package.json no longer declares is reported as stale', () => {
    const violations = audit({ ...declared, react: declared['react'] });
    const withoutIdb = Object.fromEntries(
      Object.entries(declared).filter(([name]) => name !== 'idb'),
    );

    expect(violations).toEqual([]);
    const stale = audit(withoutIdb);
    expect(stale.some((violation) => violation.includes('allows "idb"'))).toBe(true);
    expect(stale.every((violation) => violation.includes(RULE_DEPENDENCY))).toBe(true);
  });
});
