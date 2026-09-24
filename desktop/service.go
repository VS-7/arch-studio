package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/archcode/studio/desktop/internal/workspace"
)

// Desktop expõe ao frontend o que só existe no app nativo: pastas de projeto,
// diálogos do sistema, gravação de arquivos e impressão. O frontend chama estes
// métodos pelo runtime do Wails (Call.ByName("main.Desktop.<Método>")); todo o
// resto continua pela API HTTP de sempre.
type Desktop struct {
	ws     *workspace.Workspace
	prints *printJobs
}

// CurrentProject devolve o projeto aberto (nil = nenhum).
func (d *Desktop) CurrentProject() *workspace.ProjectInfo { return d.ws.Current() }

// RecentProjects lista os projetos abertos recentemente.
func (d *Desktop) RecentProjects() []workspace.RecentProject { return d.ws.Recent().List() }

// ForgetProject tira um projeto da lista de recentes.
func (d *Desktop) ForgetProject(root string) { d.ws.Recent().Remove(root) }

// IsProject informa se a pasta já contém um projeto ArchCode Studio.
func (d *Desktop) IsProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".arch", "manifest.yaml"))
	return err == nil
}

// ChooseFolder abre o seletor de pastas do sistema ("" = cancelado).
func (d *Desktop) ChooseFolder(title string) (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}

// OpenProject abre o projeto da pasta. O frontend recarrega em seguida.
func (d *Desktop) OpenProject(dir string) (*workspace.ProjectInfo, error) {
	info, err := d.ws.Open(dir)
	if err != nil {
		return nil, err
	}
	setWindowTitle(info)
	return info, nil
}

// CreateProject cria um projeto novo na pasta e o abre.
func (d *Desktop) CreateProject(dir, name string) (*workspace.ProjectInfo, error) {
	info, err := d.ws.Create(dir, name)
	if err != nil {
		return nil, err
	}
	setWindowTitle(info)
	return info, nil
}

// CloseProject volta para a tela inicial.
func (d *Desktop) CloseProject() error {
	setWindowTitle(nil)
	return d.ws.Close()
}

// SaveFile pergunta onde salvar e grava o arquivo ("" = cancelado). O runtime
// entrega os bytes como base64, decodificados automaticamente em []byte.
func (d *Desktop) SaveFile(filename string, data []byte) (string, error) {
	path, err := application.Get().Dialog.SaveFile().
		SetFilename(filename).
		CanCreateDirectories(true).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

// PrintHTML abre o documento numa janela própria com o diálogo de impressão.
func (d *Desktop) PrintHTML(title, html string) {
	d.prints.Print(application.Get(), title, html)
}
