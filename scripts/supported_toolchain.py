#!/usr/bin/env python3
import subprocess,sys
from pathlib import Path
root=Path(__file__).resolve().parents[1];pin=(root/'.go-version').read_text().strip();actual=subprocess.check_output(['go','version'],text=True).strip()
if 'go'+pin+' ' not in actual:sys.exit('BLOCKED: release requires pinned Go'+pin+'; actual '+actual)
print(actual)
