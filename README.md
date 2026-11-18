<h1 align="center">Dinoscript</h1>

<p align="center"><strong>A free, offline coding editor for kids</strong><br>
Write your first program, make text games, and solve math puzzles with Pip the triceratops.</p>

<p align="center">
  <strong><a href="https://dinoscript.org/#downloads">Download Dinoscript</a></strong> ·
  <a href="https://dinoscript.org/workbook.html">Read the workbook</a> ·
  <a href="book/Dinoscript-with-Pip.pdf">Get the PDF</a>
</p>

<p align="center">
  <img src="book/art/pip-at-the-computer.png" alt="Pip the triceratops typing on a computer among ferns" width="540">
</p>

## Get started

Open Dinoscript and press <kbd>F5</kbd> to run the starter program. **Output** shows `Hello, dinosaur!`. Change the words inside the quotes and run it again.

```text
print("Hello, dinosaur!")
```

- **Try another program.** Choose one from **Examples** or save your own as a `.ds` file. When a program calls `readint()`, type your answer in **Output** and press <kbd>Enter</kbd>.
- **Keep your work organized.** Use **New** for a blank tab, **Open** to add a file, and **My files** to switch between programs. Switching tabs stops the current run. Dinoscript restores your tabs and edits when you reopen the editor.
- **Learn with Pip.** Choose **Learn** or press <kbd>F1</kbd> to browse the illustrated lessons offline. **Try code** opens the lesson example in the editor, and **Save PDF** exports the book. The print edition also comes with the release packages.
- **Choose your language.** The editor opens in Spanish when your system language is Spanish and in English otherwise. Use the language menu to choose English, Español, or your system language. The website opens in English at the main address, with an Español link on every page.

## Build and test

The desktop build requires Go 1.27.3 or later. From the repository root, start the editor, run the bundled hello program in a terminal, or run the test suite.

```sh
go run ./cmd/dinoscript
go run ./cmd/dinoscript-run internal/desktop/examples/hello.ds
go test ./...
```

For programs that use `readint()`, pass `-input answers.txt` before the source path. Put one answer on each line of the input file.

On Linux, install the Ubuntu, Fedora, or Arch package and open **Dinoscript** from the GNOME or KDE app menu. The editor uses Xorg or XWayland, including XWayland sessions that expose their X11 socket only in Linux's abstract socket namespace. [Linux downloads](https://dinoscript.org/#downloads)

The test suite covers the interpreter, workbook programs, and desktop UI. The UI tests send keyboard and mouse events through the desktop toolkit to exercise saving, reopening, draft recovery, browsing lessons, running tutorial code, and exporting the PDF. GitHub Actions runs the suite on Linux, macOS, and Windows and collects the Mac and Windows development builds. [Go and desktop checks](.github/workflows/go.yml)

After editing the shared lesson sources or illustrations, run `python3 scripts/build-book.py` with ReportLab and Pillow installed to rebuild both PDFs, then run the tests. [English lessons](book/book.json) · [Spanish lessons](book/book.es-CL.json)

## Package a development build

| Target | Command |
| --- | --- |
| Mac with Apple silicon | `sh scripts/package-mac.sh arm64` |
| Mac with Intel | `sh scripts/package-mac.sh amd64` |
| Windows x64 | `sh scripts/package-windows.sh` |

The Mac script creates `Dinoscript.app` and, when run on a Mac, a drag-to-install `.dmg`. Run the Windows script from a POSIX shell such as Git Bash, then open `build/windows-amd64/Dinoscript.exe` on Windows.

Use development builds for testing. The desktop plan tracks release signing, notarization, and clean-machine install checks. [Desktop editor plan](docs/go-migration.md)

## Website and releases

The **Go and desktop checks** workflow builds the site on every run. Pushing a `vMAJOR.MINOR.PATCH` tag publishes the tested app packages and PDF to GitHub Releases, then triggers **Publish dinoscript.org** on the default branch. Site changes on `main` also trigger deployment, and you can run the publish workflow manually from `main`. [Pages workflow](.github/workflows/pages.yml)

The site uses the downloads and workbooks from the latest published Dinoscript release. If that release predates the Spanish edition, the site builds that edition from the current checkout. When the download page opens, it checks the latest Linux package release and updates all five installer and checksum links together. An hourly Pages check refreshes the static HTML when either release changes. The prebuilt links are available with JavaScript disabled or when GitHub is unavailable. [Linux package repository](https://github.com/fclaude/fclaude-packages)

In the repository’s Pages settings, use **GitHub Actions** as the publishing source and `dinoscript.org` as the custom domain. The `github-pages` environment deploys from `main`. [Pages settings](https://github.com/fclaude/dinoscript/settings/pages) · [Custom domain setup](https://docs.github.com/en/pages/configuring-a-custom-domain-for-your-github-pages-site/managing-a-custom-domain-for-your-github-pages-site)

For a local preview, install Pillow and run the following command. Find the generated pages and PDF in `build/site/`.

```sh
python3 scripts/build-site.py v0.1.0
```

## Language and plans

- [Go migration and desktop editor plan](docs/go-migration.md)
- [Proposed Scheme subset reference](docs/scheme.md)

## License

Copyright 2023–2026 Francisco Claude-Faust. Dinoscript and the workbook are distributed under the Apache License, Version 2.0. [License](LICENSE) · [Copyright notice](NOTICE) · [Bundled component notices](release/THIRD-PARTY-NOTICES.txt)
