"""Package an existing Windows standalone artifact; never publish or start it."""
import argparse
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--executable', type=Path, required=True)
parser.add_argument('--version', required=True)
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
binary = args.executable.resolve(strict=True)
build = json.loads(binary.with_name(binary.name + '.manifest.json').read_text())
digest = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
if build['target'] != 'windows/amd64' or build['executableSHA256'] != digest(binary):
    raise SystemExit('Target or executable digest mismatch')
args.output.parent.mkdir(parents=True, exist_ok=True)
files = {'mirabase.exe': binary}
for name in ['README.md', 'ROADMAP.md', 'LICENSE.md', 'THIRD_PARTY_NOTICES.md']:
    files[name] = ROOT / name
for folder in ['docs', 'examples/quickstart', 'third_party/turso-notices']:
    for item in sorted((ROOT / folder).rglob('*')):
        if item.is_file() and not item.is_symlink():
            files[item.relative_to(ROOT).as_posix()] = item
manifest = {'formatVersion': 1, 'version': args.version, 'target': 'windows/amd64',
    'executableSHA256': digest(binary), 'nativeSHA256': build['nativeSHA256'],
    'signatureStatus': 'unsigned', 'distribution': 'current standalone, no hosted cloud',
    'runtimeRequirements': ['Windows x64', 'Microsoft Visual C++ v14 x64 runtime'],
    'knownLimitations': 'docs/current-product.md', 'roadmap': 'ROADMAP.md',
    'files': {name: digest(path) for name, path in files.items()}}
instructions = r'''MiraBase standalone for Windows x64

1. Extract the entire ZIP into a directory you control.
2. Double-click start-mirabase.cmd, or open PowerShell here and run:
   .\mirabase.exe serve --http=127.0.0.1:8090
3. Open the private setup URL printed by the server and create your superuser.
4. Administration: http://127.0.0.1:8090/_/
5. Welcome page: http://127.0.0.1:8090/

No Go, Rust, Node or separate database service is needed to run this package.
The native engine requires the Microsoft Visual C++ v14 x64 runtime. If Windows
reports VCRUNTIME140.dll missing, install Microsoft's official x64 runtime:
https://aka.ms/vc14/vc_redist.x64.exe
Keep mb_data and your other application files across restarts. Stop with Ctrl+C.
For the included notes frontend restart with:
   .\mirabase.exe serve --http=127.0.0.1:8090 --publicDir=./examples/quickstart
Then follow docs/user/quickstart.md from the operator/collection setup steps.

This executable is unsigned. Compare this archive with SHA256SUMS from the
GitHub release. Follow your machine's execution policy; do not disable security
settings. Known endpoint/format limitations: docs/current-product.md.
Deferred integrations: ROADMAP.md. No MiraBase-hosted cloud is required/offered.
'''
launcher = '@echo off\r\ncd /d "%~dp0"\r\nmirabase.exe serve --http=127.0.0.1:8090\r\nif errorlevel 1 pause\r\n'
welcome = '''<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>MiraBase</title><style>body{font:18px system-ui;max-width:780px;margin:70px auto;padding:24px;color:#15263c;background:#f0f5fa}h1{font-size:48px}a{color:#145ca4}li{margin:16px 0}</style><h1>MiraBase is running.</h1><p>One backend. A private database for every user, tenant, and agent.</p><ol><li>Use the private setup URL in the server terminal to create your operator.</li><li><a href="/_/">Open the administration dashboard</a> and configure your collections and rules.</li><li>Follow the included docs/user/quickstart.md to connect the notes example.</li></ol><p>Your data stays in this server's persistent storage. This is the standalone backend.</p><p><a href="https://github.com/lordolami/mirabase/blob/main/docs/user/downloads.md">Download/start guide</a> · <a href="https://github.com/lordolami/mirabase/blob/main/ROADMAP.md">Open-source roadmap</a></p></html>'''
with zipfile.ZipFile(args.output, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    for name, path in files.items():
        archive.write(path, name)
    archive.writestr('manifest.json', json.dumps(manifest, indent=2))
    archive.writestr('START-HERE.txt', instructions)
    archive.writestr('start-mirabase.cmd', launcher)
    archive.writestr('mb_public/index.html', welcome)
with zipfile.ZipFile(args.output) as archive:
    if archive.testzip():
        raise SystemExit('Archive integrity failure')
    for name, expected in manifest['files'].items():
        if hashlib.sha256(archive.read(name)).hexdigest() != expected:
            raise SystemExit('Source changed while packaging: ' + name)
print(json.dumps({'archive': str(args.output), 'bytes': args.output.stat().st_size,
    'sha256': digest(args.output), 'executableSHA256': digest(binary)}, indent=2))
