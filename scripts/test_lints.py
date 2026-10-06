#!/usr/bin/env python3
"""Self-test for the lint scripts in this directory.

A lint rule that never fires proves nothing, so every rule is exercised with
both positive cases (violation detected, correct reason) and negative cases
(the current corpus stays clean). Run: python3 scripts/test_lints.py
"""

import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import check_requirement_ids as rule2
import check_test_assertions as rule1
import check_validation_citations as rule3


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
