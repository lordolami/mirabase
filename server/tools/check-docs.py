"""Validate owned documentation and source boundaries; never runtime acceptance."""
from pathlib import Path
from urllib.parse import unquote
import json, re, sqlite3

root = Path(__file__).resolve().parents[1]
paths = [*root.glob('*.md'), *root.joinpath('docs').rglob('*.md'), *root.joinpath('sdk').glob('*/README.md')]
paths += list(root.joinpath('native/bridge').glob('README.md'))
paths += list(root.joinpath('internal/mbengine/platform/payload').glob('*/README.md'))
errors = []
links = blocks = tables = 0
for path in paths:
    text = path.read_text(encoding='utf-8')
    if len(re.findall(r'^```', text, re.M)) % 2:
        errors.append(f'{path.relative_to(root)}: unmatched code fence')
    for ref in re.findall(r'\[[^\]\n]+\]\(([^)\n]+)\)', text):
        ref = ref.split(' "')[0].strip('<>')
        if re.match(r'^[a-zA-Z][a-zA-Z0-9+.-]*:', ref):
            continue
        raw, _, fragment = ref.partition('#')
        target = (path.parent / unquote(raw)).resolve() if raw else path
        links += 1
        if not target.exists():
            errors.append(f'{path.relative_to(root)}: missing {ref}')
    for body in re.findall(r'^```json\s*\n(.*?)^```\s*$', text, re.M | re.S):
        try:
            json.loads(body)
            blocks += 1
        except ValueError as err:
            errors.append(f'{path.relative_to(root)}: invalid JSON: {err}')
storage = root / 'docs/mirabase/storage-format-reference.md'
for body in re.findall(r'^```sql\s*\n(.*?)^```\s*$', storage.read_text(encoding='utf-8'), re.M | re.S):
    try:
        with sqlite3.connect(':memory:') as conn:
            conn.executescript(body)
            tables += conn.execute("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").fetchone()[0]
    except sqlite3.Error as err:
        errors.append(f'Storage DDL: {err}')
for replacement in re.findall(r'=>\s+(\.{1,2}[/\\]\S+)', (root/'go.mod').read_text()):
    target = (root/replacement).resolve()
    if not target.is_relative_to(root) or not target.exists():
        errors.append('Go replacement escapes source or is missing: '+replacement)
print(json.dumps({'documents':len(paths),'localLinks':links,'jsonBlocks':blocks,'storageTables':tables,'errors':errors},indent=2))
raise SystemExit(1 if errors else 0)
