"""Rebuild the desktop icon resources from release/pip.png using Pillow and LLVM."""

from pathlib import Path
import subprocess
import tempfile

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "release/pip.png"


def main():
    with Image.open(SOURCE) as icon, tempfile.TemporaryDirectory() as directory:
        work = Path(directory)
        icon.resize((128, 128), Image.Resampling.LANCZOS).save(ROOT / "internal/desktop/pip.png")
        icon.save(ROOT / "release/Dinoscript.icns")
        sizes = [(n, n) for n in (16, 20, 24, 32, 40, 48, 64, 128, 256)]
        icon.save(work / "pip.ico", sizes=sizes)
        (work / "pip.rc").write_text('1 ICON "pip.ico"\n', encoding="ascii")
        subprocess.run(["llvm-rc", "/FO", "pip.res", "pip.rc"], cwd=work, check=True)
        subprocess.run(
            ["llvm-cvtres", "/machine:x64", "/timestamp:0", "/out:pip.syso", "pip.res"],
            cwd=work, check=True,
        )
        (ROOT / "cmd/dinoscript/icon_windows_amd64.syso").write_bytes((work / "pip.syso").read_bytes())


if __name__ == "__main__":
    main()
