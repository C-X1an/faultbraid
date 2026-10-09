#!/usr/bin/env python3
"""Remove only marked synthetic demos; preserve evidence and user inputs."""
import shutil
from pathlib import Path
root=Path(__file__).resolve().parents[1]/'.local-runs';removed=0
if root.exists() and not root.is_symlink():
 for p in root.glob('demo-*'):
  if p.is_symlink() or not p.is_dir():continue
  marker=p/'.faultbraid-owned'
  if marker.is_file() and not marker.is_symlink() and marker.read_text()=='synthetic-demo-v1\n':shutil.rmtree(p);removed+=1
print('Removed marked synthetic demo directories:',removed,'; preserved evidence and other paths')
