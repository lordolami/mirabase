"""Create local review archives from allowlisted source/artifacts; never publish."""
from pathlib import Path
import hashlib, json, os, tarfile, zipfile

root = Path(__file__).resolve().parents[1]
out = root / 'dist'
out.mkdir(exist_ok=True)
exclude_roots = {'dist', '.builds', '.releasechecks', '.git', '.zcode', 'mb_data', 'mb_hooks', 'mb_public'}
exclude_dirs = {'node_modules', '.dart_tool', '__pycache__'}
manifest = {'formatVersion': 1, 'releaseQualified': False, 'purpose': 'local review; publication blocked pending qualification', 'artifacts': []}

# Package only explicit built artifacts and legal notices, never application
# state. These remain unsigned local review packages.
packages = []
notices = [root/'LICENSE.md', root/'THIRD_PARTY_NOTICES.md']
for target, binary_name in [('windows-amd64','mirabase-windows-amd64.exe'), ('linux-amd64','mirabase-linux-amd64')]:
    binary = out/binary_name
    if not binary.is_file():
        continue
    files = [binary, out/(binary_name+'.manifest.json'), *notices]
    if target.startswith('windows'):
        package = out/('mirabase-'+target+'.zip')
        with zipfile.ZipFile(package,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
            for item in files:
                z.write(item,item.name)
    else:
        package = out/('mirabase-'+target+'.tar.gz')
        with tarfile.open(package,'w:gz') as z:
            for item in files:
                info = z.gettarinfo(item, arcname=item.name)
                info.mode = 0o755 if item == binary else 0o644
                with item.open('rb') as stream:
                    z.addfile(info,stream)
    packages.append(package)
source_zip = out / 'mirabase-source-review.zip'
def source_files():
    for directory, children, names in os.walk(root):
        parent = Path(directory)
        keep = []
        for name in children:
            child = parent/name
            rel = child.relative_to(root)
            if name in exclude_dirs or (len(rel.parts) == 1 and name in exclude_roots) or child.is_symlink():
                continue
            if rel.parts in [('native','engine'), ('native','bridge','target')]:
                continue
            keep.append(name)
        children[:] = sorted(keep)
        for name in sorted(names):
            yield parent/name

with zipfile.ZipFile(source_zip, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as z:
    for path in source_files():
        rel = path.relative_to(root)
        if rel.parts[0] in exclude_roots or any(x in exclude_dirs for x in rel.parts):
            continue
        if rel.parts[:2] == ('native', 'engine') or rel.parts[:3] == ('native','bridge','target'):
            continue
        if not path.is_file() or path.is_symlink():
            continue
        if path.name == 'base.exe' or path.name.startswith('.env') or path.suffix in {'.tgz','.log'}:
            continue
        if not path.resolve().is_relative_to(root):
            raise RuntimeError('Source escaped release root: ' + str(rel))
        z.write(path, 'mirabase/' + rel.as_posix())

for path in dict.fromkeys([source_zip, *packages, *out.glob('mirabase-*.exe'), *out.glob('mirabase-linux-*'), *root.glob('sdk/js/mirabase-sdk-*.tgz')]):
    if not path.is_file() or path.name.endswith('.manifest.json'):
        continue
    body = path.read_bytes()
    manifest['artifacts'].append({'path':path.relative_to(root).as_posix(), 'bytes':len(body), 'sha256':hashlib.sha256(body).hexdigest()})
(out/'launch-review-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n',encoding='utf-8')
print(json.dumps(manifest, indent=2))
