#!/usr/bin/env python3
"""Summarize every measured Go benchmark sample without inventing tail latency."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import statistics
import sys

MAX_BYTES = 8 << 20


def summarize(data, expected_samples=7):
    if len(data) > MAX_BYTES or not 1 <= expected_samples <= 100:
        raise ValueError('benchmark input or sample count exceeds its bound')
    groups = {}
    package = None
    for line in data.decode('utf-8').splitlines():
        if line.startswith('pkg: '):
            package = line[5:].strip()
        if line.startswith('FAIL') or '[build failed]' in line:
            raise ValueError('failed benchmark execution cannot be summarized as success')
        if not line.startswith('Benchmark'):
            continue
        match = re.fullmatch(r'(Benchmark\S+)\s+([1-9][0-9]*)\s+(.+)', line)
        if match is None or not package:
            raise ValueError('malformed benchmark sample or missing package')
        name, iterations, metrics = match.groups()
        fields = metrics.split()
        if len(fields) % 2:
            raise ValueError('incomplete metric pairs')
        row = {'iterations': int(iterations)}
        for offset in range(0, len(fields), 2):
            value, unit = float(fields[offset]), fields[offset + 1]
            if not math.isfinite(value) or value < 0 or unit in row:
                raise ValueError('invalid or duplicate measured metric')
            row[unit] = value
        if not {'ns/op', 'B/op', 'allocs/op'} <= row.keys():
            raise ValueError('duration and allocation measurements required')
        groups.setdefault((package, name), []).append(row)
    if not groups:
        raise ValueError('no measured benchmark samples')
    results = []
    for (package, name), samples in sorted(groups.items()):
        if len(samples) != expected_samples or any(row.keys() != samples[0].keys() for row in samples):
            raise ValueError('missing samples or inconsistent metric units')
        metrics = {}
        for unit in sorted(samples[0].keys() - {'iterations'}):
            values = [row[unit] for row in samples]
            quartiles = statistics.quantiles(values, n=4, method='inclusive') if len(values) > 1 else [values[0]] * 3
            metrics[unit] = {'median': statistics.median(values), 'min': min(values), 'max': max(values),
                             'q1': quartiles[0], 'q3': quartiles[2]}
        results.append({'package': package, 'name': name, 'sample_count': len(samples), 'samples': samples, 'metrics': metrics})
    return {'schema_version': 1, 'status': 'PASS', 'input_sha256': hashlib.sha256(data).hexdigest(),
            'input_bytes': len(data), 'expected_samples': expected_samples, 'benchmarks': results,
            'scope': 'Go calibrated aggregate benchmark samples; inclusive quartiles; not individual request latency, p95/p99, native IO or production throughput'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('input', type=Path)
    parser.add_argument('--samples', type=int, default=7)
    parser.add_argument('--out', type=Path)
    args = parser.parse_args()
    try:
        with args.input.open('rb') as stream:
            result = summarize(stream.read(MAX_BYTES + 1), args.samples)
        encoded = (json.dumps(result, indent=2) + '\n').encode('utf-8')
        if args.out:
            with args.out.open('xb') as stream:
                stream.write(encoded)
        else:
            sys.stdout.write(encoded.decode('utf-8'))
    except (OSError, UnicodeError, ValueError) as error:
        print('benchmark summary rejected: ' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
