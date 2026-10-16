#!/usr/bin/env python3
"""Self-test for the lint scripts in this directory.

A lint rule that never fires proves nothing, so every rule is exercised with
both positive cases (violation detected, correct reason) and negative cases
(the current corpus stays clean). Run: python3 scripts/test_lints.py
"""

import re
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_requirement_ids as rule2
import check_test_assertions as rule1
import check_validation_citations as rule3

ROOT = Path(__file__).resolve().parent.parent


class TestVacuousAssertions(unittest.TestCase):
    def assertFlagged(self, source, arg):
        violations = rule1.find_violations(source)
        self.assertEqual([(l, a) for l, a in violations], [(1, arg)], source)

    def assertClean(self, source):
        self.assertEqual(rule1.find_violations(source), [], source)

    def test_flags_bool_and_null_literals(self):
        self.assertFlagged("void f() { expect(true, isTrue); }", "true")
        self.assertFlagged("void f() { expect(false, isFalse); }", "false")
        self.assertFlagged("void f() { expect(null, isNull); }", "null")

    def test_flags_numeric_and_string_literals(self):
        self.assertFlagged("void f() { expect(0, greaterThan(-1)); }", "0")
        self.assertFlagged("void f() { expect(-1, isNot(zero)); }", "-1")
        self.assertFlagged(
            "void f() { expect('same', equals('same')); }", "'<string literal>'"
        )

    def test_flags_multiline_assertion(self):
        source = "void f() {\n  expect(\n    true,\n    isTrue,\n  );\n}"
        violations = rule1.find_violations(source)
        self.assertEqual(violations, [(2, "true")])

    def test_flags_assertions_after_comments_and_strings(self):
        source = "// expect(true, isTrue)\nfinal s = 'expect(true, isTrue)';\nexpect(true, isTrue);"
        self.assertEqual(rule1.find_violations(source), [(3, "true")])

    def test_flags_expect_later_and_raw_string(self):
        self.assertFlagged("void f() { expectLater(true, isTrue); }", "true")
        self.assertFlagged(
            "void f() { expect(r'raw', equals('raw')); }", "'<string literal>'"
        )

    def test_passes_on_observed_values(self):
        self.assertClean("expect(value, isTrue);")
        self.assertClean("expect(find.text('Calculated'), findsOneWidget);")
        self.assertClean("expect(controller.isBlocked, isFalse);")
        self.assertClean("expect(foo(a, b), isTrue);")
        self.assertClean("expect(items.length, greaterThan(0));")

    def test_passes_on_commented_and_string_copies(self):
        self.assertClean("// expect(true, isTrue)")
        self.assertClean("/* expect(false, isFalse); */")
        self.assertClean("print('expect(true, isTrue)');")

    def test_passes_on_interpolated_string(self):
        self.assertClean("expect('$value', equals('x'));")
        self.assertClean("expect('${controller.error}', isNull);")

    def test_preserves_line_numbers_across_multiline_strings(self):
        source = "final doc = '''\nline\nline\n''';\nexpect(true, isTrue);"
        self.assertEqual(rule1.find_violations(source), [(5, "true")])


class TestRequirementIds(unittest.TestCase):
    SPEC = {"PRV-13", "RND-10", "DRB-01", "ISO-8601"}
    STATE = {"AD-001"}

    def violations(self, text):
        return rule2.find_violations(text, self.SPEC, self.STATE)

    def test_defined_id_passes(self):
        self.assertEqual(self.violations("// PRV-13 keeps the base label."), [])

    def test_dangling_id_is_reported(self):
        v = self.violations("// renders the table row (RND-21).")
        self.assertEqual(v, [(1, "RND-21", ".specs/**/spec.md")])

    def test_jira_card_keys_are_skipped(self):
        self.assertEqual(self.violations("// LGN-999 does not exist yet."), [])

    def test_standard_identifiers_are_skipped(self):
        self.assertEqual(self.violations("// stamped_at is ISO-8601."), [])

    def test_decision_ids_resolve_against_state(self):
        self.assertEqual(self.violations("// see AD-001"), [])
        v = self.violations("// see AD-042")
        self.assertEqual(v, [(1, "AD-042", ".specs/STATE.md")])

    def test_reports_line_numbers(self):
        v = self.violations("ok PRV-13\n// dead RND-99")
        self.assertEqual(v, [(2, "RND-99", ".specs/**/spec.md")])

    def test_tsx_citation_resolving_to_a_spec_passes(self):
        spec = {"FND-01", "SEC-07"}
        self.assertEqual(rule2.find_violations("// FND-01 renders the shell", spec, self.STATE), [])

    def test_ts_citation_unknown_fails(self):
        spec = {"FND-01"}
        v = rule2.find_violations("// SEC-99 is not defined", spec, self.STATE)
        self.assertEqual(v, [(1, "SEC-99", ".specs/**/spec.md")])

    def test_client_walker_ignores_node_modules_and_build_output(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            src = root / "web" / "src"
            src.mkdir(parents=True)
            (src / "app.tsx").write_text("// FND-01\n", encoding="utf-8")
            node_modules = root / "web" / "node_modules" / "pkg"
            node_modules.mkdir(parents=True)
            (node_modules / "bad.ts").write_text("// FND-99\n", encoding="utf-8")
            dist = root / "web" / "dist"
            dist.mkdir(parents=True)
            (dist / "out.ts").write_text("// FND-99\n", encoding="utf-8")
            found = {p.name for p in rule2.code_files(root)}
            self.assertEqual(found, {"app.tsx"})


class TestClientTestSuffix(unittest.TestCase):
    def test_only_client_test_files_are_scanned(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            src = root / "web" / "src"
            src.mkdir(parents=True)
            test_file = src / "shell.test.tsx"
            test_file.write_text("expect(true, isTrue);\n", encoding="utf-8")
            source_file = src / "shell.tsx"
            source_file.write_text("expect(true, isTrue);\n", encoding="utf-8")

            files = list(rule1.source_files([root / "web"]))

            self.assertIn(test_file, files)
            self.assertNotIn(source_file, files)

    def test_vacuous_assertion_in_tsx_is_flagged(self):
        self.assertEqual(rule1.find_violations("expect(true, isTrue);"), [(1, "true")])
        self.assertEqual(rule1.find_violations("expect(value, isTruthy);"), [])


class TestWorkflowPathFilters(unittest.TestCase):
    """Contract test for the CI path scoping (SEC-06) and the fail-closed rule (SEC-04)."""

    def pull_request_paths(self, workflow):
        text = (ROOT / ".github" / "workflows" / workflow).read_text(encoding="utf-8")
        lines = text.splitlines()
        pr = next((i for i, line in enumerate(lines) if line.strip() == "pull_request:"), None)
        self.assertIsNotNone(pr, f"{workflow} has no pull_request trigger")
        paths = next(
            (i for i in range(pr + 1, len(lines)) if lines[i].strip() == "paths:"), None
        )
        self.assertIsNotNone(paths, f"{workflow} pull_request has no paths filter")
        indent = len(lines[paths]) - len(lines[paths].lstrip())
        items = []
        for line in lines[paths + 1:]:
            stripped = line.strip()
            if not stripped:
                continue
            if len(line) - len(line.lstrip()) <= indent:
                break
            if stripped.startswith("-"):
                items.append(stripped.lstrip("- ").strip().strip("'\""))
        return set(items)

    def test_client_workflow_triggers_on_web(self):
        paths = self.pull_request_paths("ci-web.yml")
        self.assertIn("web/**", paths)

    def test_go_workflow_does_not_trigger_on_web(self):
        paths = self.pull_request_paths("ci-lint.yml")
        self.assertNotIn("web/**", paths)
        self.assertFalse(any(p.startswith("web/") for p in paths))

    def test_spec_lint_triggers_on_client_sources(self):
        paths = self.pull_request_paths("ci-spec-lint.yml")
        self.assertIn("web/**/*.ts", paths)
        self.assertIn("web/**/*.tsx", paths)

    def test_client_workflow_has_no_continue_on_error(self):
        text = (ROOT / ".github" / "workflows" / "ci-web.yml").read_text(encoding="utf-8")
        # Comments may explain the rule; only a real key would weaken a gate.
        code_lines = [line for line in text.splitlines() if not line.lstrip().startswith("#")]
        self.assertNotIn("continue-on-error", "\n".join(code_lines))


class TestClientGateIntegrity(unittest.TestCase):
    """Contract tests for the client security gates (SEC-02 … SEC-04, ci-gates contract).

    Both cases are regressions found by the first pull request that ran ci-web.yml:
    a pin that no action ever published, and a scanner job whose token could not
    read the change it was supposed to scan.
    """

    def workflow_text(self, workflow):
        return (ROOT / ".github" / "workflows" / workflow).read_text(encoding="utf-8")

    def job_block(self, workflow, job):
        lines = self.workflow_text(workflow).splitlines()
        start = next((i for i, line in enumerate(lines) if line.rstrip() == f"  {job}:"), None)
        self.assertIsNotNone(start, f"{workflow} has no {job} job")
        block = []
        for line in lines[start + 1:]:
            if line.strip() and len(line) - len(line.lstrip()) <= 2:
                break
            block.append(line)
        return "\n".join(block)

    def test_action_refs_are_published_form(self):
        # An unresolvable ref fails the job while it is still preparing steps, so
        # the scanner never runs: every third-party `uses:` must be a v-prefixed
        # tag (how these actions publish releases) or an immutable commit SHA.
        tag = re.compile(r"v\d+(\.\d+)*")
        actions = set()
        for workflow in sorted((ROOT / ".github" / "workflows").glob("*.yml")):
            for line in workflow.read_text(encoding="utf-8").splitlines():
                match = re.match(r"\s*-?\s*uses:\s*(\S+)\s*$", line)
                if match is None or match.group(1).startswith("./"):
                    continue
                action, separator, pin = match.group(1).partition("@")
                actions.add(action)
                pinned = separator == "@" and (
                    tag.fullmatch(pin) is not None or re.fullmatch(r"[0-9a-f]{40}", pin) is not None
                )
                self.assertTrue(pinned, f"{workflow.name}: {action} is pinned to '{pin}'")
        self.assertIn("aquasecurity/trivy-action", actions)

    def test_secret_scan_job_reads_the_pull_request(self):
        block = self.job_block("ci-web.yml", "secrets")
        self.assertIn("gitleaks/gitleaks-action@", block)
        self.assertIn("pull-requests: read", block)

    def test_sast_job_blocks_on_findings(self):
        # A scanner that only reports its findings looks green while finding
        # nothing, so the gate must fail the job on a finding.
        block = self.job_block("ci-web.yml", "sast")
        self.assertIn("semgrep scan", block)
        self.assertIn("--error", block)
        self.assertNotIn("continue-on-error", block)

    def test_trivy_pin_clears_the_pruned_dependency_floor(self):
        # Releases below v0.33.1 pull a setup-trivy tag the publisher has pruned,
        # so the scanner cannot start; a commit SHA is immutable and skips this.
        pin = next(
            (
                line.split("@", 1)[1].strip()
                for line in self.workflow_text("ci-web.yml").splitlines()
                if "aquasecurity/trivy-action@" in line
            ),
            None,
        )
        self.assertIsNotNone(pin, "ci-web.yml has no trivy-action pin")
        if re.fullmatch(r"[0-9a-f]{40}", pin) is not None:
            return
        version = tuple(int(part) for part in pin.lstrip("v").split("."))
        self.assertGreaterEqual(version, (0, 33, 1), f"trivy-action pin {pin} cannot resolve")


class TestValidationCitations(unittest.TestCase):
    def setUp(self):
        self._tmp = tempfile.TemporaryDirectory()
        self.root = Path(self._tmp.name)
        doc = self.root / ".specs" / "features" / "f1"
        doc.mkdir(parents=True)
        self.doc = ".specs/features/f1/validation.md"
        (self.root / self.doc).write_text("# validation\n", encoding="utf-8")
        src = self.root / "src"
        src.mkdir()
        (src / "long.go").write_text("\n".join(f"line {i}" for i in range(1, 11)), encoding="utf-8")
        other = self.root / "other"
        other.mkdir()
        (other / "long.go").write_text("only one line\n", encoding="utf-8")
        (src / "short.md").write_text("a\nb\nc\n", encoding="utf-8")
        self.index = rule3.build_index(self.root)

    def tearDown(self):
        self._tmp.cleanup()

    def write_doc(self, body, doc=None):
        (self.root / (doc or self.doc)).write_text(body, encoding="utf-8")

    def check(self, changed=None, doc=None):
        target = doc or self.doc
        return rule3.check_document(target, self.root, self.index, changed)

    def test_valid_citation_passes(self):
        self.write_doc("see `src/long.go:7` for the assertion\n")
        self.assertEqual(self.check(), [])

    def test_line_beyond_eof_is_reported(self):
        self.write_doc("see `src/long.go:11` for the assertion\n")
        v = self.check()
        self.assertEqual(len(v), 1)
        self.assertIn("line 11 exceeds src/long.go (10 lines)", v[0][3])

    def test_missing_file_is_reported(self):
        self.write_doc("see `src/gone.go:3` here\n")
        v = self.check()
        self.assertEqual(len(v), 1)
        self.assertIn("file not found: src/gone.go", v[0][3])

    def test_shorthand_inherits_last_file(self):
        self.write_doc("`src/long.go:9` and `:10` in one line\n")
        self.assertEqual(self.check(), [])

    def test_bare_file_mention_re_anchors_the_shorthand(self):
        # Mutation-table shape: code location, then the test file as a bare
        # span, then kill lines that belong to the test file.
        self.write_doc("`src/long.go:3` then `src/short.md` then `:3`\n")
        self.assertEqual(self.check(), [])
        self.write_doc("`src/long.go:3` then `src/short.md` then `:9`\n")
        v = self.check()
        self.assertEqual(len(v), 1)
        self.assertIn("src/short.md", v[0][3])

    def test_shorthand_without_context_is_reported(self):
        self.write_doc("orphan `:4` citation\n")
        v = self.check()
        self.assertEqual(len(v), 1)
        self.assertIn("shorthand", v[0][3])

    def test_range_and_list_are_expanded(self):
        self.write_doc("`src/long.go:8-10` plus `src/long.go:1,5`\n")
        self.assertEqual(self.check(), [])
        self.write_doc("`src/long.go:8-11`\n")
        self.assertEqual(len(self.check()), 1)

    def test_en_dash_range_is_accepted(self):
        self.write_doc("`src/long.go:8–10`\n")
        self.assertEqual(self.check(), [])

    def test_non_citation_spans_are_ignored(self):
        self.write_doc("test output `00:22 +326 ~4: All tests passed!` done\n")
        self.assertEqual(self.check(), [])

    def test_ambiguous_basename_is_reported_in_full_mode(self):
        self.write_doc("`long.go:3` is cited\n")
        v = self.check()
        self.assertEqual(len(v), 1)
        self.assertIn("ambiguous basename", v[0][3])

    def test_same_dir_basename_resolves(self):
        local = self.root / ".specs" / "features" / "f1" / "long.go"
        local.write_text("x\ny\nz\n", encoding="utf-8")
        self.index = rule3.build_index(self.root)
        self.write_doc("`long.go:3` is cited\n")
        self.assertEqual(self.check(), [])

    def test_unchanged_doc_is_skipped_except_changed_targets(self):
        self.write_doc("`src/long.go:11` and `src/short.md:9`\n")
        changed = {".specs/other/validation.md"}
        self.assertEqual(self.check(changed=changed), [])
        changed = {"src/long.go"}
        v = self.check(changed=changed)
        self.assertEqual(len(v), 1)
        self.assertIn("src/long.go", v[0][3])

    def test_deleted_file_attributed_by_basename(self):
        self.write_doc("`src/gone.go:3` here\n")
        changed = {"src/gone.go"}
        self.assertEqual(len(self.check(changed=changed)), 1)
        changed = {"src/other_deleted.go"}
        self.assertEqual(self.check(changed=changed), [])

    def test_changed_doc_is_checked_in_full(self):
        self.write_doc("`src/gone.go:3` and `src/long.go:11`\n")
        v = self.check(changed={self.doc})
        self.assertEqual(len(v), 2)

    def test_ambiguity_not_reported_for_unchanged_doc(self):
        self.write_doc("`long.go:3` is cited\n")
        changed = {"src/long.go"}
        self.assertEqual(self.check(changed=changed), [])


if __name__ == "__main__":
    unittest.main(verbosity=2)
