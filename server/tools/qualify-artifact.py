"""Exercise a built executable with disposable data and report real effects.

Uses only Python's standard library. Never touches the default application
directory, provisions credentials only in its temporary root, and stops only
the child process it started. This is a smoke test, not a load benchmark.
"""
import argparse
from contextlib import ExitStack
import hashlib
import json
import os
from pathlib import Path
import platform
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

parser = argparse.ArgumentParser()
parser.add_argument('executable', type=Path)
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
binary = args.executable.resolve(strict=True)
report = {'target': platform.system()+'/'+platform.machine(), 'kernel': platform.release(),
          'executableSHA256': hashlib.file_digest(binary.open('rb'), 'sha256').hexdigest(),
          'releaseQualified': False, 'kind': 'isolated artifact smoke, not full acceptance'}
proc = None

def stop_child(child):
    if child.poll() is not None:
        return
    child.terminate()
    try:
        child.wait(timeout=15)
    except subprocess.TimeoutExpired:
        child.kill()
        child.wait(timeout=15)

def retain_failure_log(child, log_path):
    if not report.get('passed'):
        report['serverExitBeforeCleanup'] = child.poll()
        destination = args.output.with_suffix('.server.log')
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(log_path.read_bytes())
        report['privateServerLog'] = destination.name

try:
    with ExitStack() as lifecycle:
        temp = lifecycle.enter_context(tempfile.TemporaryDirectory(prefix='mirabase-artifact-'))
        root = Path(temp)
        flags = ['--dir='+str(root/'data'), '--migrationsDir='+str(root/'migrations'), '--hooksDir='+str(root/'hooks')]
        env = dict(os.environ, XDG_CACHE_HOME=str(root/'cache'))
        password = secrets.token_hex(24)+'A1!'
        email = 'artifact@example.invalid'
        with (root/'server.log').open('wb') as log:
            init = subprocess.run([str(binary),'superuser','upsert',email,password,*flags], cwd=root, env=env, stdout=log, stderr=log, timeout=60)
            if init.returncode:
                raise RuntimeError('isolated superuser provisioning failed')
            with socket.socket() as port_socket:
                port_socket.bind(('127.0.0.1',0))
                port = port_socket.getsockname()[1]
            url = f'http://127.0.0.1:{port}'
            proc = subprocess.Popen([str(binary),'serve',f'--http=127.0.0.1:{port}',*flags], cwd=root, env=env, stdout=log, stderr=log)
            lifecycle.callback(stop_child, proc)
            lifecycle.callback(retain_failure_log, proc, root/'server.log')
            started = time.monotonic()
            def request(path, method='GET', body=None, token=None):
                headers = {'Content-Type':'application/json'}
                if token:
                    headers['Authorization'] = token
                req = urllib.request.Request(url+path, data=None if body is None else json.dumps(body).encode(), method=method, headers=headers)
                try:
                    with urllib.request.urlopen(req, timeout=15) as resp:
                        raw = resp.read()
                        return resp.status, json.loads(raw) if raw else None
                except urllib.error.HTTPError as exc:
                    return exc.code, None
            while True:
                try:
                    if request('/api/health')[0] == 200:
                        break
                except OSError:
                    pass
                if time.monotonic()-started > 30 or proc.poll() is not None:
                    raise RuntimeError('artifact did not become healthy')
                time.sleep(.05)
            report['observedReadyMs'] = round(1000*(time.monotonic()-started))
            report['health'] = 'passed'
            status, auth = request('/api/collections/_superusers/auth-with-password','POST',{'identity':email,'password':password})
            assert status == 200 and auth.get('token'), 'authentication failed'
            token = auth['token']
            status, collection = request('/api/collections','POST',{'name':'artifact_notes','type':'base','fields':[{'name':'title','type':'text','required':True}]},token)
            assert status == 200, 'collection creation failed'
            record_path = '/api/collections/artifact_notes/records'
            status, record = request(record_path,'POST',{'title':'original'},token)
            assert status == 200, 'record create failed'
            path = record_path+'/'+record['id']
            assert request(path,token=token)[1]['title'] == 'original', 'record bytes differ'
            assert request(path)[0] == 403, 'locked record did not refuse anonymous access'
            assert request(path,'DELETE',token=token)[0] == 204, 'delete failed'
            assert request(path,token=token)[0] == 404, 'deleted record still visible'
            report['crudAndAuthorization'] = 'passed'
            base = '/api/mirabase/v1/agentfs/workspaces'
            status, workspace = request(base,'POST',{'ownerCollectionId':auth['record']['collectionId'],'ownerRecordId':auth['record']['id'],'displayName':'artifact'},token)
            assert status == 200, 'workspace creation failed'
            base += '/'+workspace['id']
            assert request(base+'/files','PUT',{'path':'report.txt','content':'historical'},token)[0] == 200
            status, snapshot = request(base+'/snapshots','POST',{},token)
            assert status == 200
            assert request(base+'/files','PUT',{'path':'report.txt','content':'overwritten'},token)[0] == 200
            restore = base+'/snapshots/'+snapshot['snapshotId']+'/restore'
            assert request(restore,'POST',{},token)[0] == 200
            assert request(base+'/files?path=report.txt',token=token)[1]['content'] == 'historical'
            assert request(base+'/files?path=report.txt','DELETE',token=token)[0] == 204
            assert request(restore,'POST',{},token)[0] == 200
            assert request(base+'/files?path=report.txt',token=token)[1]['content'] == 'historical'
            assert request(base+'/files','PUT',{'path':'../escape','content':'bad'},token)[0] == 400
            assert request(base+'/files?path=report.txt')[0] in (401,403)
            report['workspaceRestoreAndNegativeControls'] = 'passed'
            status_path = Path(f'/proc/{proc.pid}/status')
            if status_path.exists():
                report['singleSampleResidentMemory'] = {line.split(':')[0]:line.split(':',1)[1].strip() for line in status_path.read_text().splitlines() if line.startswith(('VmRSS:','VmHWM:'))}
            report['passed'] = True
            proc.terminate()
            proc.wait(timeout=15)
            proc = None
except Exception as exc:
    report['passed'] = False
    report['error'] = type(exc).__name__+': '+str(exc)
finally:
    if proc is not None:
        proc.terminate()
        try:
            proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=15)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report,indent=2)+'\n', encoding='utf-8')
    print(json.dumps(report,indent=2))
raise SystemExit(0 if report.get('passed') else 1)
