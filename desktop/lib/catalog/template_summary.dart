/// Field summary derivation for the creation surface (TPL-02, TPL-03, TPL-05).
library;

import 'package:legnaris/api/models/template.dart';

/// One line of the template summary: a top-level field key and the number of
/// leaf fields it contains.
typedef FieldSummary = ({String prefix, int leafCount});

/// Derives the summary of [fields]: one [FieldSummary] per top-level key, in
/// the order the API returned the keys (map insertion order).
///
/// A group counts every leaf beneath it recursively; a top-level leaf counts
/// as 1 under its own key (TPL-03). No prefix is hardcoded, so any template
/// renders without code changes (TPL-05). An empty [fields] tree yields an
/// empty summary.
List<FieldSummary> summarizeFields(FieldGroup fields) => [
  for (final entry in fields.entries)
    (prefix: entry.key, leafCount: _countLeaves(entry.value)),
];

int _countLeaves(FieldNode node) {
  if (node.schema != null) return 1;
  final children = node.children;
  if (children == null) return 0;
  var total = 0;
  for (final child in children.values) {
    total += _countLeaves(child);
  }
  return total;
}
