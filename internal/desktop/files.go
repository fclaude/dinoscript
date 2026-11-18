package desktop

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/fclaude/dinoscript/internal/dino"
	"github.com/fclaude/dinoscript/internal/locale"
	"github.com/richardwilkes/unison"
)

type fileDialogs interface {
	openPath() (string, bool)
	savePath(extension, defaultName string) (string, bool)
	discardChanges() bool
}

type systemDialogs struct{ editor *editor }

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

func (s systemDialogs) discardChanges() bool {
	e := s.editor
	message := unison.NewMessagePanel(e.text("Leave this program?"), e.text("Save it first if you want to keep your changes."))
	cancel := unison.NewCancelButtonInfo()
	cancel.Title = e.text("Cancel")
	ok := unison.NewOKButtonInfoWithTitle(e.text("Discard changes"))
	theme := unison.DefaultDialogTheme
	dialog, err := unison.NewDialog(theme.QuestionIcon, theme.QuestionIconInk, message, []*unison.DialogButtonInfo{cancel, ok})
	return err == nil && dialog.RunModal() == unison.ModalResponseOK
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

var (
	errSourceSize     = errors.New("this program is too large to open")
	errSourceEncoding = errors.New("save the program as UTF-8 text and try again")
	errSourceControl  = errors.New("this file contains characters the editor cannot display")
)

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
		return "", errSourceSize
	}
	if !utf8.Valid(data) {
		return "", errSourceEncoding
	}
	source := strings.TrimPrefix(string(data), "\ufeff")
	source = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(source)
	for _, ch := range source {
		if ch < ' ' && ch != '\n' && ch != '\t' {
			return "", errSourceControl
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
		name := filepath.Base(path)
		switch err {
		case errSourceSize, errSourceEncoding, errSourceControl:
			e.setStatus("Couldn't open %s: %v", name, statusMessage(err.Error()))
		default:
			if e.language != locale.English {
				e.setStatus("Couldn't open %s.", name)
			} else {
				e.setStatus("Couldn't open %s: %v", name, err)
			}
		}
		return
	}
	e.openTab(draft{Text: source, Path: path, SavedText: source})
	e.setStatus("Opened %s.", filepath.Base(path))
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
		path, ok = e.dialogs.savePath("ds", e.tabTitle(e.document))
		if !ok {
			return
		}
		if tab := e.tabForPath(path); tab != nil && tab != e.document {
			e.setStatus("That file is open in another tab. Choose a different name.")
			return
		}
	}
	if err := writeFile(path, []byte(current)); err != nil {
		name := filepath.Base(path)
		if e.language != locale.English {
			e.setStatus("Couldn't save %s.", name)
		} else {
			e.setStatus("Couldn't save %s: %v", name, err)
		}
		return
	}
	e.document.Path, e.document.SavedText = path, current
	e.updateDocument()
	e.setStatus("Saved %s.", filepath.Base(path))
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
