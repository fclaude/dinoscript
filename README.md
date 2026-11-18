# dinoscript
Dinoscript - programming environment for kids

[Download Dinoscript](https://dinoscript.org/#downloads) · [Read the online workbook](https://dinoscript.org/workbook.html)

Open Dinoscript, change the starter program, and press **F5** to run it. Pick a lesson from **Examples** or save your own program as a `.ds` file. When a program calls `readint()`, type your answer in **Output** and press Enter.

Use **My files** to switch between programs. **New** starts a blank tab and **Open** adds a file to the sidebar. Switching tabs stops the current run. Dinoscript recovers your open tabs and their edits when you reopen the editor.

Choose **Learn** or press **F1** to meet Pip and browse the lessons offline. **Try code** opens the current example in the editor, and **Save PDF** exports the illustrated book. The print edition is included with the release packages. [Dinoscript with Pip](book/Dinoscript-with-Pip.pdf)

## Build and test

The Go desktop build uses Go 1.27.3 or later. Start it during development with `go run ./cmd/dinoscript`. Run a file from a terminal with `go run ./cmd/dinoscript-run example.ds`. Pass `-input answers.txt` before the source path to provide input for `readint()`.

On Linux, install the Ubuntu, Fedora, or Arch package and open **Dinoscript** from the GNOME or KDE app menu. The editor uses Xorg or XWayland, including XWayland sessions that expose their X11 socket only in Linux's abstract socket namespace. [Linux downloads](https://dinoscript.org/#downloads)

Run `go test ./...` for the interpreter, book programs, and automated desktop UI tests. The UI suite sends keyboard and mouse events through the desktop toolkit and covers saving, reopening, draft recovery, browsing lessons, running tutorial code, and saving the PDF. The GitHub Actions workflow runs the suite on Linux, macOS, and Windows and collects the Mac and Windows development builds.

The desktop lessons and print edition share [book.json](book/book.json). After changing the lessons or illustrations, run `python3 scripts/build-book.py` with ReportLab and Pillow installed, then run the tests.

## Package a development build

- On **Mac**, run `sh scripts/package-mac.sh arm64` for Apple silicon or `sh scripts/package-mac.sh amd64` for Intel. The script creates `Dinoscript.app`. On a Mac it also creates a drag-to-install `.dmg`.
- For **Windows x64**, run `sh scripts/package-windows.sh` from a POSIX shell such as Git Bash. Double-click `build/windows-amd64/Dinoscript.exe` on Windows.

Development builds are for testing. Release signing, notarization, and clean-machine install checks are tracked in the desktop plan.

## Website and releases

The Go and desktop checks workflow builds the site for every run. A `vMAJOR.MINOR.PATCH` tag publishes the checked app packages and PDF to GitHub Releases, then triggers **Publish dinoscript.org** on the default branch. The site uses the downloads and workbook from the latest published release. Changes to the site source on `main` also trigger deployment. You can rerun **Publish dinoscript.org** manually from `main`.

Linux packages are built and published by [fclaude-packages](https://github.com/fclaude/fclaude-packages). The website picks up the latest verified Linux package set when Pages deploys.

In [Settings → Pages](https://github.com/fclaude/dinoscript/settings/pages), use **GitHub Actions** as the publishing source and `dinoscript.org` as the custom domain. The `github-pages` environment deploys from `main`. [GitHub Pages custom domain setup](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site)

For a local preview, install Pillow and run `python3 scripts/build-site.py v0.1.0`. The generated pages and PDF are in `build/site/`.

## Language and plans

- [Go migration and desktop editor plan](docs/go-migration.md)
- [Proposed Scheme subset reference](docs/scheme.md)

## License

Copyright 2023–2026 Francisco Claude-Faust. Dinoscript and the workbook are distributed under the Apache License, Version 2.0. [License](LICENSE) · [Copyright notice](NOTICE) · [Bundled component notices](release/THIRD-PARTY-NOTICES.txt)
