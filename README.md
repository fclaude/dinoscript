<h1 align="center">Dinoscript</h1>

<p align="center"><strong>A free coding editor for kids</strong><br>
Write your first program, make text games, and solve math puzzles with Pip the triceratops.</p>

<p align="center">
  <strong><a href="https://dinoscript.org/play/">Try Dinoscript in your browser</a></strong> ·
  <a href="https://dinoscript.org/#downloads">Download Dinoscript</a> ·
  <a href="https://dinoscript.org/workbook.html">Read the workbook</a> ·
  <a href="book/Dinoscript-with-Pip.pdf">Get the PDF</a>
</p>

<p align="center">
  <img src="book/art/pip-at-the-computer.png" alt="Pip the triceratops typing on a computer among ferns" width="540">
</p>

## Get started

Open Dinoscript and press <kbd>F5</kbd> to run the starter program. **Output** shows `Hello, dinosaur!`. Change the words inside the quotes and run it again.

The [browser playground](https://dinoscript.org/play/) runs programs on your device and keeps drafts in the browser. Use **Save** to download a `.ds` file you can open in either edition.

```text
print("Hello, dinosaur!")
```

- **Try another program.** Choose **Pip Reads Your Mind** or another program from **Examples**, or save your own as a `.ds` file. When a program calls `pause()`, press <kbd>Enter</kbd> to continue. When it calls `readint()`, type your answer in **Output** and press <kbd>Enter</kbd>.
- **Keep your work organized.** Use **New** for a blank tab, **Open** to add a file, and **My files** to switch between programs. Switching tabs stops the current run. Dinoscript restores your tabs and edits when you reopen the editor.
- **Learn with Pip.** Choose **Learn** or press <kbd>F1</kbd> to browse the illustrated lessons offline. **Try code** opens the lesson example in the editor, and **Save PDF** exports the book. The print edition also comes with the release packages.
- **Choose your language.** The editor follows your system language for English, Chilean Spanish, and Polish, with English as the fallback. Use the language menu to pick one yourself. The website opens in English at the main address and offers Español and Polski on every page.

## Build and test

The desktop build requires Go 1.27.3 or later. From the repository root, start the editor, run the bundled hello program in a terminal, or run the test suite.

```sh
go run ./cmd/dinoscript
go run ./cmd/dinoscript-run internal/desktop/examples/hello.ds
go test ./...
```

For programs that use `readint()` or `pause()`, pass `-input answers.txt` before the source path. Put each answer on its own line and include a blank line for each pause.

On Linux, install the Ubuntu, Fedora, or Arch package and open **Dinoscript** from the GNOME or KDE app menu. The editor uses Xorg or XWayland, including XWayland sessions that expose their X11 socket only in Linux's abstract socket namespace. [Linux downloads](https://dinoscript.org/#downloads)

The test suite covers the interpreter, workbook programs, and desktop UI. The UI tests send keyboard and mouse events through the desktop toolkit to exercise saving, reopening, draft recovery, browsing lessons, running tutorial code, and exporting the PDF. GitHub Actions runs the suite on Linux, macOS, and Windows and collects the Mac and Windows development builds. [Go and desktop checks](.github/workflows/go.yml)

After editing the shared lesson sources or illustrations, run `python3 scripts/build-book.py` with ReportLab and Pillow installed to rebuild the PDFs, then run the tests. [English lessons](book/book.json) · [Spanish lessons](book/book.es-CL.json) · [Polish lessons](book/book.pl.json)

## Package a development build

| Target | Command |
| --- | --- |
| Mac with Apple silicon | `sh scripts/package-mac.sh arm64` |
| Mac with Intel | `sh scripts/package-mac.sh amd64` |
| Windows x64 | `sh scripts/package-windows.sh` |

The Mac script creates `Dinoscript.app` and, when run on a Mac, a drag-to-install `.dmg`. Run the Windows script from a POSIX shell such as Git Bash, then open `build/windows-amd64/Dinoscript.exe` on Windows.

Use development builds for testing. The desktop plan tracks release signing, notarization, and clean-machine install checks. [Desktop editor plan](docs/go-migration.md)

## Website and releases

The **Go and desktop checks** workflow builds the site on every run. Pushing a `vMAJOR.MINOR.PATCH` tag publishes the tested app packages and PDFs to GitHub Releases, then triggers **Publish dinoscript.org** on the default branch. Site changes on `main` also trigger deployment, and you can run the publish workflow manually from `main`. [Pages workflow](.github/workflows/pages.yml)

The site uses the downloads and workbooks from the latest published Dinoscript release. For a language that has yet to ship in that release, the site takes the workbook from the current checkout. When the download page opens, it checks the latest Linux package release and updates all five installer and checksum links together. An hourly Pages check refreshes the static HTML when either release changes. The prebuilt links are available with JavaScript disabled or when GitHub is unavailable. [Linux package repository](https://github.com/fclaude/fclaude-packages)

In the repository’s Pages settings, use **GitHub Actions** as the publishing source and `dinoscript.org` as the custom domain. The `github-pages` environment deploys from `main`. [Pages settings](https://github.com/fclaude/dinoscript/settings/pages) · [Custom domain setup](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site)

For a local preview, install Pillow and Go, then build and serve the site. Open [the playground](http://localhost:8000/play/) in your browser.

```sh
python3 scripts/build-site.py v0.1.0
python3 -m http.server 8000 --bind 127.0.0.1 --directory build/site
```

## Language and plans

- [Go migration and desktop editor plan](docs/go-migration.md)
- [Proposed Scheme subset reference](docs/scheme.md)

## License

Copyright 2023–2026 Francisco Claude-Faust. Dinoscript and the workbook are distributed under the Apache License, Version 2.0. [License](LICENSE) · [Copyright notice](NOTICE) · [Bundled component notices](release/THIRD-PARTY-NOTICES.txt)
