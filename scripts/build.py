#!/usr/bin/env python3
"""Build the local binary with measured source and dependency identities."""
import hashlib,json,os,subprocess
from pathlib import Path
from run_evidence import source_identity,sha

root=Path(__file__).resolve().parents[1]
identity=source_identity(root)
module_hash=hashlib.sha256(b'\0'.join((root/name).read_bytes() for name in ('go.mod','go.sum','dependency.lock.json') if (root/name).exists())).hexdigest()
patch_lock=root/'third_party/raft-patch/lock.json'
patch_hash=json.loads(patch_lock.read_text())['patch_sha256'] if patch_lock.exists() else 'unrecorded'
prefix='github.com/C-X1an/faultbraid/internal/buildinfo.'
flags=' '.join('-X '+prefix+key+'='+value for key,value in (
 ('SourceTreeSHA',identity['source_tree_sha256']),('ModuleLockSHA',module_hash),('PatchSHA',patch_hash)))
binary=root/'bin'/('faultbraid.exe' if os.name=='nt' else 'faultbraid')
binary.parent.mkdir(exist_ok=True)
command=['go','build','-trimpath','-ldflags',flags,'-o',str(binary),'./cmd/faultbraid']
subprocess.run(command,cwd=root,check=True)
out=root/'.local-runs';out.mkdir(exist_ok=True)
(out/'build.json').write_text(json.dumps({'command':command,'source':identity,'module_lock_sha256':module_hash,'patch_sha256':patch_hash,'binary_sha256':sha(binary)},indent=2)+'\n')
print(json.dumps({'status':'PASS','source_tree_sha256':identity['source_tree_sha256'],'binary_sha256':sha(binary)}))
