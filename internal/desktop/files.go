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
	Name      string
	Color     int
}

type workspaceDraft struct {
	Tabs   []draft
	Active int
}

const maxWorkspaceBytes = 64 << 20

func readWorkspace(path string) (workspaceDraft, bool) {
	file, err := os.Open(path)
	if err != nil {
		return workspaceDraft{}, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxWorkspaceBytes+1))
	var recovered workspaceDraft
	if err != nil || len(data) > maxWorkspaceBytes || json.Unmarshal(data, &recovered) != nil {
		return workspaceDraft{}, false
	}
	if recovered.Tabs == nil {
		// Single-document installations stored their draft directly in this file.
		var legacy draft
		if json.Unmarshal(data, &legacy) != nil {
			return workspaceDraft{}, false
		}
		recovered.Tabs = []draft{legacy}
	}
	if recovered.Active < 0 || recovered.Active >= len(recovered.Tabs) {
		return workspaceDraft{}, false
	}
	for _, tab := range recovered.Tabs {
		if len(tab.Text) > dino.MaxSourceBytes || len(tab.SavedText) > dino.MaxSourceBytes {
			return workspaceDraft{}, false
		}
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
	path, ok := e.dialogs.openPath()
	if !ok {
		return
	}
	if tab := e.tabForPath(path); tab != nil {
		e.selectTab(tab)
		return
	}
	source, err := readSource(path)
	if err != nil {
		e.setStatus(fmt.Sprintf("Couldn't open %s: %v", filepath.Base(path), err))
		return
	}
	e.openTab(draft{Text: source, Path: path, SavedText: source})
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
		path, ok = e.dialogs.savePath("ds", e.document.title())
		if !ok {
			return
		}
		if tab := e.tabForPath(path); tab != nil && tab != e.document {
			e.setStatus("That file is open in another tab. Choose a different name.")
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
	session := workspaceDraft{Tabs: make([]draft, 0, len(e.tabs))}
	for i, tab := range e.tabs {
		if tab == e.document {
			session.Active = i
		}
		session.Tabs = append(session.Tabs, tab.draft)
	}
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	if len(data) > maxWorkspaceBytes {
		return errors.New("the workspace is too large to recover automatically")
	}
	return writeFile(e.draftPath, data)
}
