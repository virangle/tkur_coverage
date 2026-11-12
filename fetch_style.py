"""Development helper: inspect pinned public upstream styling, not a runtime dependency."""
import base64
import hashlib
import json
from pathlib import Path
import urllib.request

BASE = "https://api.github.com/repos/virangle/tkur_jacoco"
PINNED_COMMIT = "fbd7e252d9a424db1e5a7dd685dd25f520a3db15"
with urllib.request.urlopen(BASE + "/git/trees/" + PINNED_COMMIT + "?recursive=1", timeout=30) as response:
    tree = json.load(response)
sha = tree["sha"]
print("SHA", sha)
root = Path(__file__).parent
assets = root / "tkur_coverage" / "assets"
reference = root / "upstream-reference"
assets.mkdir(exist_ok=True)
reference.mkdir(exist_ok=True)
resource_prefix = "org.jacoco.report/src/org/jacoco/report/internal/html/resources/"
reference_names = {"ReportPage.java", "NodePage.java", "TablePage.java", "tkur.properties", "LICENSE.md", "BarColumn.java", "Table.java", "Resources.java"}
manifest = {"repository": "https://github.com/virangle/tkur_jacoco", "commit": sha, "files": []}
for entry in tree["tree"]:
    path = entry["path"]
    name = path.rsplit("/", 1)[-1]
    is_asset = path.startswith(resource_prefix) and (name.endswith(".svg") or name in {"report.css", "lucide-LICENSE.txt"})
    if entry["type"] != "blob" or not (is_asset or (name in reference_names and not path.startswith("org.jacoco.report.test/"))):
        continue
    url = f"https://raw.githubusercontent.com/virangle/tkur_jacoco/{sha}/{path}"
    # Public raw host is unavailable in this environment; Git blob API returns exact bytes.
    with urllib.request.urlopen(BASE + "/git/blobs/" + entry["sha"], timeout=30) as response:
        blob = json.load(response)
        data = base64.b64decode(blob["content"])
    target = (assets if is_asset else reference) / name
    target.write_bytes(data)
    manifest["files"].append({"upstream_path": path, "file": str(target.relative_to(root)), "url": url,
                              "git_blob_sha": entry["sha"], "sha256": hashlib.sha256(data).hexdigest()})
    print(path, len(data))
(assets / "provenance.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
