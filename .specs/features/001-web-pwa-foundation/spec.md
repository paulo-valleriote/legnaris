# Feature Specification: Web PWA Foundation

**Feature Branch**: `001-web-pwa-foundation`

**Created**: 2026-10-15

**Status**: Draft

**Input**: User description: "Web PWA foundation: installable app shell, design tokens, and the security-gated CI pipeline"

## Clarifications

### Session 2026-10-15

- Q: With no catalog yet, what does the shell's content region render at this foundation stage? → A: A designed empty-workspace state (no demo content), replaced by the catalog in a later feature.
- Q: Which browsers form the target matrix for the installability criterion? → A: The latest two versions of Chrome, Edge, Firefox, and Safari; automation covers the Chromium engine, the others are verified manually per release.
- Q: How does the client switch versions when a release is published mid-session? → A: It notifies the visitor and applies the new version only when they accept the reload.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Open and install the application shell (Priority: P1)

A person opens the product's address in a browser. A dense, consistent workspace shell appears —
navigation, main content, and a contextual panel — with no placeholder scaffolding. The browser
reports the app as installable; installed, it opens in its own window, and a repeat visit still
renders the shell when the network is unavailable.

**Why this priority**: nothing else in the product can be delivered or demonstrated until the
client shell exists and can be reached and installed; it is the foundation every later feature
mounts into.

**Independent Test**: open the served address in a browser, confirm the three shell regions render
in order, confirm the browser's install affordance reports the app as installable, install it,
then reopen it with the network disabled and confirm the shell still renders.

**Acceptance Scenarios**:

1. **Given** a first-time visitor on the served address, **When** the page finishes loading, **Then** the shell renders navigation, main content, and the contextual panel in that order, with no
   placeholder or demo content.
2. **Given** no catalog exists yet, **When** the shell is rendered, **Then** the content region shows
   a designed empty-workspace state that explains what will appear there, instead of demo data.
3. **Given** the app is served over HTTPS, **When** the visitor inspects the browser's install
   affordance, **Then** the app is reported as installable with the product name, an icon set, and
   standalone display.
4. **Given** the app has been opened successfully at least once, **When** the visitor opens it again
   with the network unavailable, **Then** the application shell still renders and states that data
   is unavailable instead of showing an empty or broken page.
5. **Given** an installed app, **When** it is launched from the operating system, **Then** it opens
   in standalone mode without browser chrome.
6. **Given** a new version is published while a visitor has the app open, **When** the update is
   detected, **Then** the visitor is notified and the new version is applied only after they accept
   the reload.
7. **Given** the shell is rendered, **When** the visitor resizes the window down to a narrow
   viewport, **Then** the contextual panel collapses before navigation or content.

---

### User Story 2 - Client changes are gated by quality and security checks (Priority: P2)

A contributor opens a pull request that changes the client. Automated checks block the change
unless the code builds, is formatted and linted, type-checks, passes its tests, and clears
dependency, static-analysis, secret, and lockfile-integrity scanning. A client-only change does not
run the backend pipelines, and a backend-only change does not run the client pipeline.

**Why this priority**: the foundation is the only place where the gate can be established once; if
client code lands before the gate exists, every later feature ships unverified.

**Independent Test**: open a pull request that changes one client file and confirm the client
pipeline runs and the backend pipeline does not; then seed a known-vulnerable dependency and a
secret-like string in separate changes and confirm each is rejected by its own gate.

**Acceptance Scenarios**:

1. **Given** a pull request that touches client files only, **When** CI runs, **Then** the client
   quality and security pipeline runs and the backend pipelines are not triggered.
2. **Given** a change that introduces a dependency with a known published vulnerability, **When**
   the client pipeline runs, **Then** the dependency gate fails and the change cannot merge.
3. **Given** a change that contains a secret-like string, **When** the client pipeline runs,
   **Then** the secret gate fails and the change cannot merge.
4. **Given** a change in which a security gate is removed or configured to continue on error,
   **When** the change is reviewed, **Then** it is rejected as a policy violation.
5. **Given** the dependency ecosystem, **When** a security release becomes available, **Then** an
   update is proposed automatically and is distinguishable from routine updates.
6. **Given** client source files that cite requirement identifiers, **When** the spec lint runs,
   **Then** a citation not defined in the spec corpus fails the pipeline.

---

### User Story 3 - One documented visual system (Priority: P3)

A contributor builds a screen and inherits the same density, spacing, typography, focus, and hover
behavior as the rest of the product, with the rules written down.

**Why this priority**: it multiplies the value of every later screen but delivers no user-facing
journey on its own, so it follows the shell and the gate.

**Independent Test**: build a screen using only the published tokens and documented rules, then
confirm no ad-hoc spacing/typography value is needed and that text and controls meet contrast
requirements.

**Acceptance Scenarios**:

1. **Given** the published design tokens, **When** a contributor builds a screen, **Then** spacing,
   typography, radii, focus, and hover derive from the tokens and no ad-hoc values are introduced.
2. **Given** the shell's text and controls, **When** contrast is measured, **Then** every
   foreground/background pair used meets the AA threshold.
3. **Given** a new contributor, **When** they read the client's contributor rules, **Then** density,
   resizing, focus, hover, and the veto of generic "AI SaaS" styling are documented.

---

### Edge Cases

- **First visit with no network**: the app has never been loaded, so no shell is available; the
  browser's own failure is acceptable, but once the shell exists it MUST load offline (FND-06).
- **Release while a session is open**: a new release replaces the previous shell; the visitor MUST
  NOT end up with half of one version and half of another, and MUST NOT lose in-progress work
  silently.
- **Deep link to an unknown route**: the app MUST render a designed not-found state, never a blank
  page or an unhandled error.
- **Browser without install support**: the app MUST remain fully usable in a tab; installability is
  an enhancement, not a precondition.
- **Scanner unavailable or a gate cannot run**: the pipeline MUST fail rather than skip the gate
  (SEC-04); a gate that silently disappears is worse than a red build.
- **Secret-scanner false positive on a test fixture**: there MUST be a documented, reviewable way to
  allow a specific false positive without disabling the gate for the repository.
- **Backend-only change**: it MUST NOT be blocked by, nor trigger, the client pipeline (SEC-06).

## Requirements *(mandatory)*

### Functional Requirements

**Shell and visual system**

- **FND-01**: The client MUST render an application shell composed of navigation, main content, and
  a contextual panel, presented in that order, and MUST NOT ship placeholder or demo content; until
  the catalog feature lands, the content region MUST render a designed empty-workspace state.
- **FND-02**: The shell MUST derive spacing, typography, radii, focus, and hover behavior from one
  shared token set; screens MUST NOT introduce ad-hoc equivalents.
- **FND-03**: The shell MUST adapt to narrow viewports by collapsing the contextual panel before
  navigation or content.
- **FND-04**: Every interactive control MUST expose a visible keyboard focus state and MUST be
  reachable by keyboard alone.
- **FND-05**: Every foreground/background pair used by the shell MUST meet the AA contrast
  threshold.

**Installable, offline-capable client**

- **FND-06**: The app MUST be served over HTTPS and MUST be reported as installable by the latest two
  versions of Chrome, Edge, Firefox, and Safari, with a product name, icon set, and standalone
  display. Automated verification covers the Chromium engine; the remaining engines are verified
  manually at each release.
- **FND-07**: After one successful load, the application shell MUST render without network access,
  and MUST communicate that data is unavailable rather than presenting an empty or broken view.
- **FND-08**: Every declared in-app route MUST render the same screen on direct navigation (deep
  link) as it does when reached from within the app; undeclared routes MUST render a designed
  not-found state.
- **FND-09**: The client MUST NOT store credentials or authenticated responses in offline storage,
  and a new release MUST replace the previous shell without mixing versions: the client MUST notify
  the visitor of the available update and apply it only when the visitor accepts the reload.
- **FND-10**: The client MUST load its entry scripts from its own origin only, with no inline
  script and no third-party script origin, so that a strict content policy can be enforced later.

**Blocking quality and security gates**

- **SEC-01**: Every pull request that changes client files MUST run formatting/lint, type-check,
  unit tests, and a production build; any failure MUST block merge.
- **SEC-02**: Every such pull request MUST run dependency scanning (known-vulnerability analysis)
  and static application security analysis; a new finding MUST block merge.
- **SEC-03**: Every such pull request MUST run secret scanning and dependency-lockfile integrity
  verification; a finding MUST block merge and MUST NOT be suppressible by repository-wide settings.
- **SEC-04**: Security gates MUST be non-optional: no gate may be removed, skipped, or configured to
  continue on error, and a gate that cannot execute MUST fail the pipeline.
- **SEC-05**: Dependency updates MUST be proposed automatically, with security updates distinguished
  from routine updates.
- **SEC-06**: Client-only changes MUST run only the client pipeline and MUST NOT trigger the backend
  pipelines; backend-only changes MUST NOT trigger the client pipeline.
- **SEC-07**: Requirement identifiers cited in client source MUST be traceable to the spec corpus,
  enforced by the same lint rule that already covers backend sources.
- **SEC-08**: Client test suites MUST be non-vacuous (assertions that cannot fail are rejected), by
  the same rule that already covers backend tests.
- **SEC-09**: The foundation MUST ship a test that proves the shell renders its regions and that the
  token contract is applied, so that the gate is proven to have something to protect.

### Key Entities

- **Shell region**: one of the three always-present areas (navigation, content, contextual panel);
  carries order, collapse priority, and the token subset it uses.
- **Design token**: a named value (spacing, typography, radius, color pair, focus, hover) with a
  purpose; the only sanctioned source of visual values in the client.
- **Declared route**: a path the client serves; carries the screen it renders, its deep-link
  behavior, and whether it is public or requires a session.
- **Quality/security gate**: an automated check on client changes; carries its trigger, what it
  inspects, whether it blocks merge, and the artifact it produces.
- **Offline shell artifacts**: the cached files that let the app boot without network access; carry
  a version identity so a release can replace them atomically.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A first-time visitor sees the complete shell in under 2.5 seconds on a mid-range
  device over a typical connection.
- **SC-002**: 100% of the target browser matrix — the latest two versions of Chrome, Edge, Firefox,
  and Safari — report the app as installable, and an installed app opens in standalone mode.
- **SC-003**: A repeat visit with the network disabled renders the shell in under 1 second.
- **SC-004**: 100% of client pull requests run the full gate set; a seeded vulnerable dependency and
  a seeded secret are each rejected on the first run (0 false negatives in the acceptance test).
- **SC-005**: 100% of declared routes resolve to the same screen on direct navigation as from
  in-app navigation.
- **SC-006**: The shell reports zero critical automated accessibility violations.
- **SC-007**: 0 requirement-identifier citations in client source are untraceable to the corpus.

## Assumptions

- The product backend already exists and requires authentication for its data routes; this feature
  establishes the client shell and does not call those routes yet, so the content region stays on
  the designed empty-workspace state until the catalog feature lands.
- The client is served on its own origin over HTTPS in every environment except local development.
- Automated installability and accessibility checks run on the Chromium engine; the other engines in
  the target matrix are verified manually at each release, which the gate documentation records.
- All user-facing strings and identifiers remain English-only, per the project constitution.
- Strict content-policy header hardening, dynamic runtime scanning, and cross-repository auth
  changes are owned by later features; this feature only guarantees the preconditions they need
  (external (non-inline) entry scripts, and a gate that can be extended without being removed).
- The deprecated desktop client remains in the repository until functional parity is reached, so
  nothing is removed here.
- Contributors have the client toolchain available locally, but CI is the authority on quality and
  security.
