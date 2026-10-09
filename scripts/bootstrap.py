#!/usr/bin/env python3
import json,shutil,subprocess,sys
from pathlib import Path
missing=[x for x in ('go','git','make','gcc') if not shutil.which(x)]
if sys.version_info<(3,11):missing.append('Python3.11+')
if missing:sys.exit('Install required free tools in your scoped environment: '+', '.join(missing))
root=Path(__file__).resolve().parents[1]
prepare=root/'third_party/raft-patch/prepare.py'
if prepare.exists():subprocess.run([sys.executable,str(prepare)],cwd=root,check=True)
actual=subprocess.check_output(['go','version'],text=True).strip();pin=(Path(__file__).resolve().parents[1]/'.go-version').read_text().strip()
print(json.dumps({'status':'PASS','go':actual,'release_pin':pin,'release_toolchain_matches':('go'+pin+' ') in actual,'note':'A seed source-compatibility run is not supported-toolchain release verification.'}))
