#!/usr/bin/env python3
"""Generate the offline manifest report paths from the pinned snapshot."""
import json
from pathlib import Path
root = Path(__file__).resolve().parent.parent
snapshot = json.loads((root / 'testdata/openapi-2.31.10.json').read_text())
paths = []
for path, item in sorted(snapshot['paths'].items()):
    name = path.removeprefix('/api/reports/')
    if (path.startswith('/api/reports/') and '/' not in name and '{' not in name
            and name != 'first-date' and 'get' in item
            and 'application/json' in item['get'].get('responses', {}).get('200', {}).get('content', {})):
        paths.append(path)
(root / 'internal/commands/report_paths.json').write_text(json.dumps(paths, indent=2) + '\n')
