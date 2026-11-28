import { appendFileSync, existsSync, readFileSync } from 'node:fs';

const [resultsPath, ledgerPath, infoJSON, capRaw] = process.argv.slice(2);
const info = JSON.parse(infoJSON);
const cap = Number(capRaw);
const problems = [];

if (!Number.isFinite(cap) || cap <= 0) {
  problems.push(`invalid cost cap ${JSON.stringify(capRaw)}`);
}

let stats = { successes: 0, failures: 0, errors: 0 };
if (existsSync(resultsPath)) {
  try {
    const results = JSON.parse(readFileSync(resultsPath, 'utf8'));
    stats = results?.results?.stats ?? stats;
  } catch (err) {
    problems.push(`cannot read promptfoo results: ${err.message}`);
  }
} else {
  problems.push('promptfoo produced no results file');
}

const entries = [];
if (existsSync(ledgerPath)) {
  for (const line of readFileSync(ledgerPath, 'utf8').split('\n')) {
    if (line.trim() === '') {
      continue;
    }
    try {
      entries.push(JSON.parse(line));
    } catch (err) {
      problems.push(`cannot read cost ledger: ${err.message}`);
    }
  }
}

const cases = Number(stats.successes ?? 0) + Number(stats.failures ?? 0) + Number(stats.errors ?? 0);
const rate = cases > 0 ? (Number(stats.successes ?? 0) * 100) / cases : 0;
const known = entries.filter((entry) => entry.known === true && typeof entry.cost_usd === 'number');
const unknown = entries.filter((entry) => entry.known !== true);
const cost = known.reduce((sum, entry) => sum + entry.cost_usd, 0);
const models = [...new Set(entries.map((entry) => entry.model).filter(Boolean))];
const versions = [...new Set(entries.map((entry) => entry.prompt_version).filter(Boolean))];

if (unknown.length > 0) {
  problems.push(`${unknown.length} provider call(s) returned an unknown cost`);
}
if (existsSync(`${ledgerPath}.blocked`)) {
  const reason = readFileSync(`${ledgerPath}.blocked`, 'utf8').trim();
  problems.push(`the eval was aborted: ${reason}`);
}
if (cases > 0 && entries.length === 0) {
  problems.push('the cost ledger has no entries for a completed eval');
}
if (known.length > 0 && cost > cap) {
  problems.push(`cost ${cost.toFixed(4)} USD exceeds the cap ${cap.toFixed(4)} USD`);
}
if (cases > 0 && rate < 90) {
  problems.push(`pass rate ${rate.toFixed(1)}% is below the 90% gate`);
}
if (versions.length > 1) {
  problems.push(`more than one prompt version was recorded: ${versions.join(', ')}`);
}

const gate = problems.length === 0 ? 'PASS' : 'FAIL';
const modelLabel = models.length > 0 ? models.join(', ') : info.model;
const versionLabel = versions.length > 0 ? versions.join(', ') : info.prompt_version;

const lines = [
  '## Generation prompt evals',
  '',
  '| Prompt version | Model | Cases | Passed | Failed | Errors | Pass rate | Cost (USD) | Gate |',
  '| --- | --- | --- | --- | --- | --- | --- | --- | --- |',
  `| ${versionLabel} | ${modelLabel} | ${cases} | ${stats.successes ?? 0} | ${stats.failures ?? 0} | ${stats.errors ?? 0} | ${rate.toFixed(1)}% | ${cost.toFixed(4)} | ${gate} |`,
];
if (problems.length > 0) {
  lines.push('', `Gate failures: ${problems.join('; ')}.`);
}
const report = `${lines.join('\n')}\n`;

process.stdout.write(report);
if (process.env.GITHUB_STEP_SUMMARY) {
  try {
    appendFileSync(process.env.GITHUB_STEP_SUMMARY, report);
  } catch (err) {
    process.stderr.write(`cannot write the step summary: ${err.message}\n`);
  }
}
if (problems.length > 0) {
  process.exit(1);
}
