package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/memory"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Memória do projeto: retomada, sessões e memórias (RF050–RF052)
// ---------------------------------------------------------------------------

// Resume monta a resposta de "onde parou" para a pessoa ("" = identidade do
// Git).
func (a *App) Resume(detail, person string) (*memory.Resume, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	manifest, err := a.st.LoadManifest()
	if err != nil {
		return nil, err
	}
	if person == "" {
		person = a.Person()
	}
	sessions, _ := a.st.ListSessions(20)
	notes, _ := a.st.ListNotes()
	installed, _ := a.installedSkills()

	var git *memory.GitState
	var mainCommits []gitx.Commit
	if a.git.Available() {
		st, err := a.git.Status()
		var last *gitx.Commit
		if log, err := a.git.Log("HEAD", 1); err == nil && len(log) > 0 {
			last = &log[0]
		}
		if err == nil {
			git = memory.NewGitState(st, last)
		}
		main := conv.Git.MainBranch
		if a.git.RevExists(main) {
			mainCommits, _ = a.git.Log(main, 30)
		}
	}
	return memory.Build(memory.Input{
		Now: a.now(), Project: manifest.ProjectName, Person: person, Detail: detail, Plan: p, Conventions: conv,
		Sessions: sessions, Notes: notes, Skills: installed, Stacks: a.projectStacks(), Git: git, MainCommits: mainCommits,
		SkillsFor: a.skillsFor,
	}), nil
}

// SessionInput registra uma sessão de trabalho.
type SessionInput struct {
	Summary   string              `json:"summary"`
	Done      []string            `json:"done,omitempty"`
	Decisions []string            `json:"decisions,omitempty"`
	NextSteps []string            `json:"next_steps,omitempty"`
	Blockers  []string            `json:"blockers,omitempty"`
	Files     []string            `json:"files,omitempty"`
	Commands  []string            `json:"commands,omitempty"`
	Tasks     []string            `json:"tasks,omitempty"`
	Checks    []model.CheckResult `json:"checks,omitempty"`
	Agent     string              `json:"agent,omitempty"`
	Started   string              `json:"started,omitempty"`
	Person    string              `json:"person,omitempty"`
}

// LogSession grava o diário da sessão em .arch/sessions/. Sem tarefas
// informadas, valem as tarefas em andamento da pessoa; branch e commits vêm
// do Git.
func (a *App) LogSession(in SessionInput, source string) (*model.Session, error) {
	if strings.TrimSpace(in.Summary) == "" {
		return nil, model.Invalid("resuma a sessão em uma ou duas frases (summary)")
	}
	all := append([]string{in.Summary}, in.Done...)
	all = append(all, in.Decisions...)
	all = append(all, in.NextSteps...)
	all = append(all, in.Blockers...)
	all = append(all, in.Commands...)
	if found := memory.FindSecrets(all...); len(found) > 0 {
		return nil, model.Invalid("a sessão parece conter um segredo (%s); ela vai para o Git — remova antes de gravar", strings.Join(found, ", "))
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	person := strings.TrimSpace(in.Person)
	if person == "" {
		person = a.Person()
	}
	now := a.now().UTC()
	s := &model.Session{
		Author: person, Agent: strings.TrimSpace(in.Agent), Started: strings.TrimSpace(in.Started),
		Ended: now.Format("2006-01-02T15:04:05Z07:00"), Tasks: upperTrim(in.Tasks),
		Summary: strings.TrimSpace(in.Summary), Done: cleanList(in.Done), Decisions: cleanList(in.Decisions),
		NextSteps: cleanList(in.NextSteps), Blockers: cleanList(in.Blockers), Files: cleanList(in.Files),
		Commands: cleanList(in.Commands), Checks: in.Checks,
	}
	if s.Started == "" {
		s.Started = s.Ended
	}
	if act := p.ActiveSprint(); act != nil {
		s.Sprint = act.Number
	}
	if len(s.Tasks) == 0 {
		for _, it := range plan.MyWork(p, person) {
			s.Tasks = append(s.Tasks, it.ID)
		}
	}
	for _, id := range s.Tasks {
		if p.Item(id) == nil {
			return nil, itemNotFound(p, id)
		}
	}
	if a.git.Available() {
		s.Branch = a.git.CurrentBranch()
		main := conv.Git.MainBranch
		if s.Branch != "" && s.Branch != main && a.git.RevExists(main) {
			if log, err := a.git.Log(main+".."+s.Branch, 20); err == nil {
				for _, c := range log {
					s.Commits = append(s.Commits, shortHash(c.Hash))
				}
			}
		}
	}
	// id: data-hora-pessoa-assunto (único e ordenável pelo nome do arquivo).
	who := model.Slugify(strings.Fields(plan.DisplayName(person) + " sessao")[0])
	topic := model.Slugify(firstWords(s.Summary, 4))
	base := now.Format("2006-01-02-1504") + "-" + who
	if topic != "" {
		base += "-" + topic
	}
	s.ID = base
	for i := 2; a.st.Exists(store.DirSessions + "/" + s.ID + ".md"); i++ {
		s.ID = fmt.Sprintf("%s-%d", base, i)
	}
	if err := a.st.SaveSession(s); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventSession, Source: source, Path: s.File,
		Message: fmt.Sprintf("Sessão registrada por %s", plan.DisplayName(person))})
	return s, nil
}

// Sessions lista as sessões mais recentes (limit <= 0 = todas).
func (a *App) Sessions(limit int) ([]model.Session, error) { return a.st.ListSessions(limit) }

// NoteInput grava uma memória.
type NoteInput struct {
	Slug       string   `json:"slug,omitempty"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Type       string   `json:"type,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Components []string `json:"components,omitempty"`
}

// Remember grava (ou atualiza, se o slug já existe) uma memória do projeto e
// regenera o índice MEMORY.md.
func (a *App) Remember(in NoteInput, source string) (*model.Note, error) {
	in.Title, in.Body = strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)
	if in.Title == "" || in.Body == "" {
		return nil, model.Invalid("a memória precisa de título e corpo")
	}
	if found := memory.FindSecrets(in.Title, in.Body); len(found) > 0 {
		return nil, model.Invalid("a memória parece conter um segredo (%s); ela vai para o Git — remova antes de gravar", strings.Join(found, ", "))
	}
	slug := model.Slugify(in.Slug)
	if slug == "" {
		slug = model.Slugify(firstWords(in.Title, 8))
	}
	if slug == "" {
		return nil, model.Invalid("título sem letras para formar o nome do arquivo")
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	notes, err := a.st.ListNotes()
	if err != nil {
		return nil, err
	}
	now := a.timestamp()
	n := &model.Note{Slug: slug, Title: in.Title, Body: in.Body, Type: model.NormalizeNoteType(in.Type),
		Tags: cleanList(in.Tags), Components: cleanList(in.Components), Author: plan.DisplayName(a.Person()),
		Created: now, Updated: now}
	for _, old := range notes {
		if old.Slug == slug {
			n.Created = old.Created
			if n.Author == "" {
				n.Author = old.Author
			}
		}
	}
	if err := a.st.SaveNote(n); err != nil {
		return nil, err
	}
	a.writeMemoryIndex()
	a.emit(hub.Event{Type: hub.EventMemory, Source: source, Path: n.File, Message: "Memória gravada: " + n.Title})
	return n, nil
}

// Recall busca memórias por texto (título, corpo e tags), tipo ou componente.
// Sem filtros, devolve todas.
func (a *App) Recall(query, noteType, component string, limit int) ([]model.Note, error) {
	notes, err := a.st.ListNotes()
	if err != nil {
		return nil, err
	}
	terms := strings.Fields(strings.ToLower(query))
	type scored struct {
		n     model.Note
		score int
	}
	out := []scored{}
	for _, n := range notes {
		if noteType != "" && n.Type != model.NormalizeNoteType(noteType) {
			continue
		}
		if component != "" && !containsFold(n.Components, component) {
			continue
		}
		hay := strings.ToLower(n.Title + " " + n.Body + " " + strings.Join(n.Tags, " ") + " " + strings.Join(n.Components, " "))
		score := 0
		for _, t := range terms {
			if strings.Contains(strings.ToLower(n.Title), t) {
				score += 3
			}
			if strings.Contains(hay, t) {
				score++
			}
		}
		if len(terms) > 0 && score == 0 {
			continue
		}
		out = append(out, scored{n, score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].n.Updated > out[j].n.Updated
	})
	list := []model.Note{}
	for _, s := range out {
		if limit > 0 && len(list) >= limit {
			break
		}
		list = append(list, s.n)
	}
	return list, nil
}

// ForgetNote apaga uma memória.
func (a *App) ForgetNote(slug, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()
	if err := a.st.RemoveNote(slug); err != nil {
		return err
	}
	a.writeMemoryIndex()
	a.emit(hub.Event{Type: hub.EventMemory, Source: source, Message: "Memória apagada: " + slug})
	return nil
}

// writeMemoryIndex regenera o .arch/memory/MEMORY.md (uma linha por memória).
func (a *App) writeMemoryIndex() {
	notes, err := a.st.ListNotes()
	if err != nil {
		return
	}
	var b strings.Builder
	b.WriteString("# Memória do projeto\n\n")
	b.WriteString("Índice gerado pelo ArchCode Studio — uma linha por memória. Grave com `remember` (MCP) ou pela interface; decisões grandes viram ADR.\n\n")
	if len(notes) == 0 {
		b.WriteString("Nenhuma memória ainda.\n")
	}
	for _, n := range notes {
		fmt.Fprintf(&b, "- [%s](%s.md) — %s", n.Title, n.Slug, n.Type)
		if s := model.NoteSummary(&n); s != "" && s != n.Title {
			fmt.Fprintf(&b, " · %s", s)
		}
		b.WriteString("\n")
	}
	_ = a.st.WriteFile(store.FileMemoryIndex, []byte(b.String()))
}

func firstWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ")
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
