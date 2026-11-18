package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/richardwilkes/unison"
)

type fileDialogs interface {
	openPath() (string, bool)
	savePath(extension, defaultName string) (string, bool)
	discardChanges() bool
}

type systemDialogs struct{}

func (systemDialogs) openPath() (string, bool) {
	dialog := unison.NewOpenDialog()
	dialog.SetAllowedExtensions("ds")
	if !dialog.RunModal() {
		return "", false
	}
	return dialog.Path(), true
}

func (systemDialogs) savePath(extension, defaultName string) (string, bool) {
	dialog := unison.NewSaveDialog()
	dialog.SetInitialFileName(defaultName)
	dialog.SetAllowedExtensions(extension)
	if !dialog.RunModal() {
		return "", false
	}
	return unison.ValidateSaveFilePath(dialog.Path(), extension, false)
}

func (systemDialogs) discardChanges() bool {
	return unison.QuestionDialog("Leave this program?", "Save it first if you want to keep your changes.") == unison.ModalResponseOK
}

type draft struct {
	Text      string
	Path      string
	SavedText string
}

func readDraft(path string) (draft, bool) {
	if path == "" {
		return draft{}, false
	}
	file, err := os.Open(path)
	if err != nil {
		return draft{}, false
	}
	defer file.Close()
	// A JSON string can take six bytes per byte of source after escaping.
	const maxDraftBytes = 12*dino.MaxSourceBytes + 8192
	data, err := io.ReadAll(io.LimitReader(file, maxDraftBytes+1))
	if err != nil || len(data) > maxDraftBytes {
		return draft{}, false
	}
	var recovered draft
	if json.Unmarshal(data, &recovered) != nil ||
		len(recovered.Text) > dino.MaxSourceBytes || len(recovered.SavedText) > dino.MaxSourceBytes {
		return draft{}, false
	}
	return recovered, true
}

func readSource(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, dino.MaxSourceBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > dino.MaxSourceBytes {
		return "", errors.New("this program is too large to open")
	}
	if !utf8.Valid(data) {
		return "", errors.New("save the program as UTF-8 text and try again")
	}
	source := strings.TrimPrefix(string(data), "\ufeff")
	source = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(source)
	for _, ch := range source {
		if ch < ' ' && ch != '\n' && ch != '\t' {
			return "", errors.New("this file contains characters the editor cannot display")
		}
	}
	return source, nil
}

func writeFile(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".dinoscript-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(content)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func (e *editor) open() {
	if e.dirty() && !e.dialogs.discardChanges() {
		return
	}
	path, ok := e.dialogs.openPath()
	if !ok {
		return
	}
	source, err := readSource(path)
	if err != nil {
		e.setStatus(fmt.Sprintf("Couldn't open %s: %v", filepath.Base(path), err))
		return
	}
	e.load(draft{Text: source, Path: path, SavedText: source})
	e.setStatus("Opened " + filepath.Base(path) + ".")
}

func (e *editor) save() {
	path := e.document.Path
	current := e.source.Text()
	if path != "" {
		onDisk, err := readSource(path)
		if err != nil || onDisk != e.document.SavedText {
			e.setStatus("The saved file has changed. Choose where to save a copy.")
			path = ""
		}
	}
	if path == "" {
		var ok bool
		path, ok = e.dialogs.savePath("ds", "My program.ds")
		if !ok {
			return
		}
	}
	if err := writeFile(path, []byte(current)); err != nil {
		e.setStatus(fmt.Sprintf("Couldn't save %s: %v", filepath.Base(path), err))
		return
	}
	e.document.Path, e.document.SavedText = path, current
	e.updateDocument()
	e.setStatus("Saved " + filepath.Base(path) + ".")
}

func (e *editor) writeDraft() error {
	if e.draftPath == "" {
		return errors.New("the user's application data folder is unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(e.draftPath), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(e.document)
	if err != nil {
		return err
	}
	return writeFile(e.draftPath, data)
}
