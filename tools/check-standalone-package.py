"""Check a Windows ZIP using isolated data; stop only owned child processes.

Runs small real HTTP checks, not a load test or full product qualification.
Python and Node are verifier dependencies, not server runtime dependencies.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import zipfile

parser = argparse.ArgumentParser()
parser.add_argument('archive', type=Path)
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
archive_path = args.archive.resolve(strict=True)
report = {'scope': 'Windows extracted-package HTTP/tutorial checks; not full qualification',
          'packageSHA256': hashlib.sha256(archive_path.read_bytes()).hexdigest(), 'checks': []}
child = None


def checked(name, condition):
    if not condition:
        raise RuntimeError(name)
    report['checks'].append(name)


def stop():
    global child
    if child is not None:
        if child.poll() is None:
            child.terminate()
            try:
                child.wait(timeout=15)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=15)
        child = None


try:
    if os.name != 'nt':
        raise RuntimeError('This check must execute on Windows')
    node = shutil.which('node')
    if not node:
        raise RuntimeError('Node is needed only to verify the documented example client')
    with tempfile.TemporaryDirectory(prefix='mirabase-package-') as temp:
        root = Path(temp)
        package = root / 'package'
        package.mkdir()
        with zipfile.ZipFile(archive_path) as archive:
            checked('ZIP CRC', archive.testzip() is None)
            for item in archive.infolist():
                target = (package / item.filename).resolve()
                if not target.is_relative_to(package.resolve()):
                    raise RuntimeError('Archive path escapes package')
            manifest = json.loads(archive.read('manifest.json'))
            checked('All manifest file digests', all(
                hashlib.sha256(archive.read(name)).hexdigest() == expected
                for name, expected in manifest['files'].items()))
            archive.extractall(package)
        binary = package / 'mirabase.exe'
        report['executableSHA256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
        checked('Executable matches manifest', report['executableSHA256'] == manifest['executableSHA256'])
        env = dict(os.environ, PATH=str(Path(os.environ['SystemRoot']) / 'System32'),
                   LOCALAPPDATA=str(root / 'cache'), XDG_CACHE_HOME=str(root / 'cache'))
        report['backendPATH'] = 'Windows System32 only; host VC runtime installed'
        flags = ['--dir=' + str(root / 'data'), '--hooksDir=' + str(root / 'hooks'),
                 '--migrationsDir=' + str(root / 'migrations')]
        password = secrets.token_hex(24) + 'A1!'
        with (root / 'server.log').open('wb') as log:
            initialized = subprocess.run([str(binary), 'superuser', 'upsert',
                'package@example.invalid', password, *flags], cwd=root, env=env,
                stdout=log, stderr=log, timeout=60)
            checked('Isolated operator setup', initialized.returncode == 0)
            with socket.socket() as listener:
                listener.bind(('127.0.0.1', 0))
                port = listener.getsockname()[1]
            base = f'http://127.0.0.1:{port}'

            def request(path, method='GET', body=None, token=None):
                headers = {'Content-Type': 'application/json'}
                if token:
                    headers['Authorization'] = token
                req = urllib.request.Request(base + path, method=method, headers=headers,
                    data=None if body is None else json.dumps(body).encode())
                try:
                    with urllib.request.urlopen(req, timeout=15) as response:
                        raw = response.read()
                        content_type = response.headers.get('Content-Type', '')
                        value = json.loads(raw) if 'application/json' in content_type and raw else raw.decode()
                        return response.status, value, content_type
                except urllib.error.HTTPError as error:
                    return error.code, None, ''

            def start(example):
                global child
                command = [str(binary), 'serve', f'--http=127.0.0.1:{port}', *flags]
                if example:
                    command.append('--publicDir=' + str(package / 'examples/quickstart'))
                child = subprocess.Popen(command, cwd=root, env=env, stdout=log, stderr=log)
                deadline = time.monotonic() + 30
                while time.monotonic() < deadline and child.poll() is None:
                    try:
                        if request('/api/health')[0] == 200:
                            return
                    except OSError:
                        pass
                    time.sleep(.05)
                raise RuntimeError('Package did not become healthy')

            try:
                start(False)
                status, page, kind = request('/')
                checked('Default welcome page outside checkout', status == 200 and 'MiraBase is running.' in page)
                stop()
                start(True)
                status, auth, _ = request('/api/collections/_superusers/auth-with-password', 'POST',
                    {'identity': 'package@example.invalid', 'password': password})
                checked('Operator login', status == 200)
                operator = auth['token']
                status, members, _ = request('/api/collections', 'POST',
                    {'name': 'members', 'type': 'auth', 'passwordAuth': {'enabled': True, 'identityFields': ['email']}}, operator)
                checked('Documented members collection', status == 200)
                rule = '@request.auth.id != "" && owner = @request.auth.id'
                notes = {'name': 'notes', 'type': 'base', 'fields': [
                    {'name': 'title', 'type': 'text', 'required': True, 'max': 200},
                    {'name': 'body', 'type': 'text'},
                    {'name': 'owner', 'type': 'relation', 'collectionId': members['id'], 'required': True, 'maxSelect': 1},
                    {'name': 'created', 'type': 'autodate', 'onCreate': True}],
                    'listRule': rule, 'viewRule': rule, 'createRule': rule, 'deleteRule': rule,
                    'updateRule': rule + ' && @request.body.owner:changed = false'}
                checked('Documented notes schema and five rules', request('/api/collections', 'POST', notes, operator)[0] == 200)
                accounts = {}
                for name in ('alice', 'bob'):
                    email = name + '@example.invalid'
                    status, account, _ = request('/api/collections/members/records', 'POST',
                        {'email': email, 'password': password, 'passwordConfirm': password}, operator)
                    checked(name + ' account created', status == 200)
                    status, login, _ = request('/api/collections/members/auth-with-password', 'POST',
                        {'identity': email, 'password': password})
                    checked(name + ' login', status == 200)
                    accounts[name] = (account['id'], login['token'])
                # Execute the actual packaged example module, not a reimplementation.
                runner = root / 'client-check.mjs'
                runner.write_text('''import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {createClient} = await import(pathToFileURL(process.argv[2]));
const client = createClient(process.argv[3]);
await client.login('alice@example.invalid', process.env.MB_CHECK_PASSWORD);
const note = await client.createNote('package persistence', 'real packaged client');
assert((await client.listNotes()).items.some(x => x.id === note.id));
client.logout(); await assert.rejects(client.listNotes());
await client.login('bob@example.invalid', process.env.MB_CHECK_PASSWORD);
assert(!(await client.listNotes()).items.some(x => x.id === note.id));
await client.login('alice@example.invalid', process.env.MB_CHECK_PASSWORD);
assert((await client.listNotes()).items.some(x => x.id === note.id));
console.log(JSON.stringify({id: note.id}));
''', encoding='utf-8')
                result = subprocess.run([node, str(runner), str(package / 'examples/quickstart/client.mjs'), base],
                    env=dict(os.environ, MB_CHECK_PASSWORD=password), capture_output=True, text=True, timeout=45)
                checked('Exact example client login/create/list/logout/relogin and isolation', result.returncode == 0)
                note_id = json.loads(result.stdout)['id']
                record = '/api/collections/notes/records/' + note_id
                alice, bob = accounts['alice'], accounts['bob']
                checked('Cross-account read denied', request(record, token=bob[1])[0] == 404)
                checked('Forged owner create denied', request('/api/collections/notes/records', 'POST',
                    {'title': 'forged', 'owner': alice[0]}, bob[1])[0] == 400)
                checked('Owner transfer denied', request(record, 'PATCH', {'owner': bob[0]}, alice[1])[0] == 404)
                checked('Required title validation', request('/api/collections/notes/records', 'POST',
                    {'owner': alice[0]}, alice[1])[0] == 400)
                checked('Title length validation', request('/api/collections/notes/records', 'POST',
                    {'title': 'x' * 201, 'owner': alice[0]}, alice[1])[0] == 400)
                for path, expected in [('/', 'text/html'), ('/client.mjs', 'javascript'), ('/_/', 'text/html')]:
                    status, _, kind = request(path)
                    checked('Static serving ' + path, status == 200 and expected in kind)
                stop()
                start(True)
                status, saved, _ = request(record, token=alice[1])
                checked('Record persists after actual stop/restart', status == 200 and saved['title'] == 'package persistence')
                stop()
                smoke = subprocess.run([__import__('sys').executable,
                    str(Path(__file__).with_name('qualify-artifact.py')), str(binary),
                    '--output', str(root / 'smoke.json')], capture_output=True, text=True, timeout=120)
                report['artifactSmoke'] = json.loads((root / 'smoke.json').read_text())
                checked('Artifact CRUD/workspace smoke and negative controls', smoke.returncode == 0)
            finally:
                stop()
        report['result'] = 'PASS'
    report['temporaryDataRemoved'] = True
except Exception as error:
    stop()
    report['result'] = 'FAIL'
    report['error'] = type(error).__name__ + ': ' + str(error)
finally:
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(report, indent=2))
raise SystemExit(0 if report.get('result') == 'PASS' else 1)
