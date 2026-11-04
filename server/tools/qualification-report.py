"""Index executed Go JSONL evidence without promoting tests to full acceptance.

Run from the owned source root. Later logs override earlier outcomes for the
same test; each original run and its failures remain in the report.
"""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--windows', nargs='+', type=Path, required=True)
parser.add_argument('--linux', nargs='+', type=Path, required=True)
parser.add_argument('--output', type=Path, default=ROOT/'release/feature-qualification.json')
parser.add_argument('--date', default='2026-11-04')
args = parser.parse_args()

def read_run(path):
    tests, packages, starts, package_starts = {}, {}, set(), set()
    malformed = 0
    with path.open(encoding='utf-8-sig') as stream:
        for line in stream:
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                malformed += 1
                continue
            test = event.get('Test')
            key = (event.get('Package'), test)
            if event['Action'] == 'start' and key[0]:
                package_starts.add(key[0])
            if event['Action'] == 'run' and test:
                starts.add(key)
            if event['Action'] in ('pass', 'fail', 'skip'):
                if test:
                    tests[key] = event['Action']
                else:
                    packages[key[0]] = event['Action']
    digest = hashlib.file_digest(path.open('rb'), 'sha256').hexdigest()
    summary = {
        'artifact': path.name, 'sha256': digest,
        'testOutcomesIncludingSubtests': dict(Counter(tests.values())),
        'packageOutcomes': dict(Counter(packages.values())),
        'packagesWithoutTests': sorted(p for p, s in packages.items() if s == 'skip'),
        'failedTests': [f'{p}:{t}' for (p,t),s in tests.items() if s == 'fail'],
        'unfinishedTests': [f'{p}:{t}' for p,t in sorted(starts-tests.keys())],
        'unfinishedPackages': sorted(package_starts-packages.keys()),
        'malformedLines': malformed,
    }
    summary['executionPassed'] = bool(packages) and not malformed and not summary['unfinishedTests'] and not summary['unfinishedPackages'] and 'fail' not in packages.values() and not any(s != 'pass' for s in tests.values())
    return summary, tests, packages

targets = {}
all_tests = {}
for target, paths in [('windows/amd64', args.windows), ('linux/amd64', args.linux)]:
    runs, tests, packages = [], {}, {}
    for path in paths:
        summary, result, package_result = read_run(path)
        runs.append(summary)
        tests.update(result)
        packages.update(package_result)
    targets[target] = {'runs': runs, 'mergedTestOutcomesIncludingSubtests': dict(Counter(tests.values())), 'mergedPackageOutcomes': dict(Counter(packages.values()))}
    all_tests[target] = tests

# These are the selected, existing assertions, not a declaration that an entire
# feature family or every third-party client has been accepted.
groups = [
    ('Collections, CRUD and batch', ['apis/collection*test.go','apis/record_crud*test.go','apis/batch_test.go']),
    ('Authentication and account flows', ['apis/record_auth*test.go']),
    ('File APIs and protected downloads', ['apis/file_test.go']),
    ('S3 protocol and filesystem adapters', ['tools/filesystem/**/*test.go']),
    ('Realtime subscriptions', ['apis/realtime_test.go']),
    ('Backups, settings and SQL console', ['apis/backup_test.go','apis/settings_test.go','apis/sql_test.go']),
    ('Go and JavaScript extension integration', ['plugins/**/*test.go','apis/extensions_test.go']),
    ('Private database HTTP isolation', ['apis/mirabase_isolation_test.go']),
    ('Provisioning, grants, agents and lifecycle', ['internal/application/application_test.go','apis/mirabase_provision_template_test.go']),
    ('Empty app setup, memberships, schema types and derived data', ['apis/mirabase_setup_acceptance_test.go']),
    ('Administrative operations and target backup effects', ['apis/mirabase_admin_acceptance_test.go']),
    ('PocketBase adoption and remote-file manifests', ['apis/mirabase_pbimport_acceptance_test.go','apis/mirabase_import_case_test.go','internal/pbimport/*test.go']),
    ('Search, ranking, cursors and budgets', ['apis/mirabase_search_acceptance_test.go','internal/search/*test.go']),
    ('Record journal update/delete', ['apis/mirabase_records_journal_acceptance_test.go']),
    ('Live-query effects and authorization', ['apis/mirabase_live_acceptance_test.go']),
    ('Reference refresh and secret/filter refusal', ['internal/application/reference_copy_test.go']),
    ('PostgreSQL TLS/SCRAM, simple and extended wire tests', ['internal/pgateway/*test.go']),
    ('WAL/MVCC writers, idempotency, jobs, outbox and crash boundary', ['internal/dbexec/*test.go']),
    ('Shared service admission limits and cancellation', ['internal/admission/*test.go']),
    ('Workspace historical bytes and integrity refusal', ['internal/application/agentfs_test.go']),
    ('Archive symlink and malformed input handling', ['tools/archive/*test.go']),
    ('OpenAPI schema', ['apis/mirabase_openapi_test.go','internal/setup/openapi_test.go']),
    ('Inherited field validation and serialization', ['core/field_*test.go']),
    ('OAuth provider adapters', ['tools/auth/**/*test.go']),
    ('Mail templates and transport', ['mails/*test.go','tools/mailer/*test.go']),
    ('Cron, logs and schema migration tooling', ['apis/cron_test.go','apis/logs_test.go','plugins/migratecmd/*test.go','tools/cron/*test.go']),
    ('Core SQL, tables, query and transaction behavior', ['core/db*test.go']),
    ('Core backup archive contents and concurrent snapshot regressions', ['core/backup*test.go']),
    ('Settings, OTP, MFA and account metadata', ['core/settings*test.go','core/otp*test.go','core/mfa*test.go','core/auth_origin*test.go','core/external_auth*test.go']),
    ('Record query rules, filters and validators', ['core/record_query*test.go','tools/search/*test.go','core/validators/*test.go']),
]
features = []
for name, patterns in groups:
    symbols = []
    for source in sorted({p for pattern in patterns for p in ROOT.glob(pattern)}):
        package = 'mirabase/' + source.parent.relative_to(ROOT).as_posix()
        for test in re.findall(r'^func (Test\w+)\(t \*testing\.T\)', source.read_text(encoding='utf-8'), re.M):
            symbols.append({'source': source.relative_to(ROOT).as_posix(), 'symbol': test, 'targetOutcomes': {target: tests.get((package,test), 'not executed') for target,tests in all_tests.items()}})
    features.append({'id': f'FQ-{len(features)+1:02}', 'area': name, 'claim': 'selected assertions only; full-family release qualification remains separate', 'tests': symbols,
        'targetSummary': {target: dict(Counter(s['targetOutcomes'][target] for s in symbols)) for target in targets}})

output = {'formatVersion': 1, 'date': args.date, 'releaseQualified': False,
    'counting': 'Each Test and subtest is counted once per run. Package skip means no test files; test skips never count as a pass. Latest logs override earlier test outcomes but retain original run failures. A package pass from a focused invocation does not establish a clean full-package run; inspect the original runs and unresolved test outcomes.',
    'targets': targets, 'features': features,
    'limits': ['Linux execution is Ubuntu under WSL on amd64, not a clean physical release machine.',
               'Windows archive run required explicitly approved administrator symlink privilege.',
               'Test names and source groupings identify assertions; inspect the test before making a broader compatibility or durability claim.',
               'macOS and ARM payloads/runners, external PG clients, full offline/AgentFS integration, native hard memory limits, signing, and full load/fault qualification remain separate gates.']}
output['testSourceSHA256'] = {
    source: hashlib.sha256((ROOT/source).read_bytes()).hexdigest()
    for source in sorted({test['source'] for feature in features for test in feature['tests']})
}
output['targetObservations'] = {
    target: {'allLatestObservedAssertionsPassed': data['mergedTestOutcomesIncludingSubtests'].get('fail',0) == 0 and data['mergedTestOutcomesIncludingSubtests'].get('skip',0) == 0,
             'fullFamilyAcceptance': False}
    for target,data in targets.items()
}
args.output.parent.mkdir(parents=True, exist_ok=True)
args.output.write_text(json.dumps(output, indent=2)+'\n', encoding='utf-8')
print(json.dumps({'output': str(args.output), 'areas': len(features), 'targets': {t:v['mergedTestOutcomesIncludingSubtests'] for t,v in targets.items()}}, indent=2))
