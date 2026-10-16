// Scratch seed (never merged): a canonical eval sink, to prove the `sast` and
// `quality` gates fail closed. Deleted with this branch.
export function runSeededSink(expression: string): unknown {
  return eval(expression);
}
