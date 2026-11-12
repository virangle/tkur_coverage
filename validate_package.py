"""Development-only smoke check for source distribution assets in a built wheel."""
from pathlib import Path
import zipfile

wheel = sorted((Path(__file__).parent / "wheel-check").glob("*.whl"))[-1]
with zipfile.ZipFile(wheel) as archive:
    names = archive.namelist()
    for name in ("report.css", "adapter.css", "tkur.svg", "jacoco-LICENSE.md", "lucide-LICENSE.txt", "NOTICE.txt", "provenance.json"):
        asset = "tkur_coverage/assets/" + name
        assert asset in names, asset
        assert archive.read(asset) == (Path(__file__).parent / asset).read_bytes(), asset
        print("PACKAGED", asset)
