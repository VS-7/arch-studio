package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/memory"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/studio"
)

// Comandos de execução e memória: reservar, retomar, checkpoint, sessão.

func cmdClaim(args []string) error {
	fs := flag.NewFlagSet("claim", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	push := fs.Bool("push", false, "publica a reserva no remoto (commit vazio + push)")
	noSwitch := fs.Bool("no-switch", false, "não troca para a branch da tarefa")
	force := fs.Bool("force", false, "assume mesmo reservada por outra pessoa ou bloqueada")
	agent := fs.String("agent", "", "agente de IA que vai trabalhar em seu nome")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	id := ""
	if len(pos) > 0 {
		id = pos[0]
	} else {
		r, err := s.App.Resume("brief", "")
		if err != nil {
			return err
		}
		if r.Next == nil {
			return fmt.Errorf("nenhuma tarefa pronta para você; informe o id: archcode-studio claim TASK-001")
		}
		id = r.Next.ID
	}
	res, err := s.App.ClaimTask(id, app.ClaimOptions{Switch: !*noSwitch, Push: *push, Force: *force, Agent: *agent}, hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s reservada: %s\n", res.Task.ID, res.Task.Title)
	fmt.Printf("  Branch: %s", res.Branch)
	switch {
	case res.BranchCreated && res.Switched:
		fmt.Print(" (criada, você está nela)")
	case res.BranchCreated:
		fmt.Print(" (criada)")
	case res.Switched:
		fmt.Print(" (você está nela)")
	}
	fmt.Println()
	if res.Pushed {
		fmt.Println("  Reserva publicada no remoto.")
	} else if res.NextCommand != "" {
		fmt.Printf("  Para o time ver a reserva: %s\n", res.NextCommand)
	}
	if len(res.Skills) > 0 {
		fmt.Printf("  Skills: %s\n", strings.Join(res.Skills, ", "))
	}
	for _, w := range res.Warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}
	return nil
}

func cmdRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	reason := fs.String("reason", "", "motivo (visível para o time)")
	force := fs.Bool("force", false, "libera reserva de outra pessoa")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("informe o id: archcode-studio release TASK-001")
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	it, err := s.App.ReleaseTask(pos[0], *reason, *force, hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s liberada (%s). O checkpoint fica para quem assumir.\n", it.ID, plan.StatusLabels[it.Status])
	return nil
}

func cmdResume(args []string) error {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	full := fs.Bool("full", false, "mais detalhes (critérios, descrições, mais sessões)")
	fs.Bool("brief", true, "resumo curto (padrão)")
	asJSON := fs.Bool("json", false, "saída em JSON")
	hook := fs.Bool("hook", false, "modo hook de início de sessão: silencioso fora de um projeto")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		if *hook {
			return nil
		}
		return err
	}
	detail := memory.DetailBrief
	if *full {
		detail = memory.DetailFull
	}
	r, err := s.App.Resume(detail, "")
	if err != nil {
		if *hook {
			return nil
		}
		return err
	}
	if *asJSON {
		return printJSON(r)
	}
	if *hook {
		fmt.Println("Contexto do ArchCode Studio (gerado por `archcode-studio resume --brief` no início da sessão):")
		fmt.Println()
	}
	fmt.Print(memory.Text(r))
	return nil
}

func cmdCheckpoint(args []string) error {
	fs := flag.NewFlagSet("checkpoint", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	last := fs.String("last", "", "último passo feito")
	next := fs.String("next", "", "próximo passo")
	notes := fs.String("notes", "", "observações")
	files := fs.String("files", "", "arquivos (separados por vírgula)")
	failing := fs.String("failing", "", "testes falhando (separados por vírgula)")
	auto := fs.Bool("auto", false, "registra os arquivos alterados da tarefa em andamento (hooks do agente)")
	quiet := fs.Bool("quiet", false, "sem saída e sem erro (hooks)")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		if *quiet {
			return nil
		}
		return err
	}
	if *auto {
		it, err := s.App.AutoCheckpoint(hub.SourceCLI)
		if *quiet {
			return nil
		}
		if err != nil {
			return err
		}
		if it == nil {
			fmt.Println("Nada a registrar: nenhuma tarefa sua em andamento com arquivos alterados.")
			return nil
		}
		fmt.Printf("✓ Checkpoint automático de %s: %s\n", it.ID, strings.Join(it.Handoff.Files, ", "))
		return nil
	}
	id := ""
	if len(pos) > 0 {
		id = pos[0]
	} else {
		r, err := s.App.Resume("brief", "")
		if err != nil {
			return err
		}
		if len(r.MyWork) != 1 {
			return fmt.Errorf("informe a tarefa: archcode-studio checkpoint TASK-001 --next \"…\"")
		}
		id = r.MyWork[0].ID
	}
	in := app.HandoffInput{}
	flagSet := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })
	if flagSet["last"] {
		in.LastStep = last
	}
	if flagSet["next"] {
		in.NextStep = next
	}
	if flagSet["notes"] {
		in.Notes = notes
	}
	if flagSet["files"] {
		list := splitComma(*files)
		in.Files = &list
	}
	if flagSet["failing"] {
		list := splitComma(*failing)
		in.FailingTests = &list
	}
	it, err := s.App.SaveCheckpoint(id, in, hub.SourceCLI)
	if err != nil {
		if *quiet {
			return nil
		}
		return err
	}
	if !*quiet {
		fmt.Printf("✓ Checkpoint de %s gravado\n", it.ID)
	}
	return nil
}

func splitComma(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

const sessionUsage = `Uso: archcode-studio session <subcomando>

  log --summary "…" [--done … --decision … --next … --blocker … --command … --task ID]
                         Registra a sessão em .arch/sessions/ (flags repetíveis)
  list [--limit 10]      Sessões mais recentes
`

func cmdSession(args []string) error {
	sub, rest, err := subcommand(args, sessionUsage)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("session "+sub, flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	switch sub {
	case "log":
		summary := fs.String("summary", "", "resumo da sessão")
		agent := fs.String("agent", "", "agente de IA que participou")
		var done, decisions, next, blockers, commands, tasks, files listFlag
		fs.Var(&done, "done", "feito (repita)")
		fs.Var(&decisions, "decision", "decisão (repita)")
		fs.Var(&next, "next", "próximo passo (repita)")
		fs.Var(&blockers, "blocker", "bloqueio (repita)")
		fs.Var(&commands, "command", "comando e resultado (repita)")
		fs.Var(&tasks, "task", "tarefa da sessão (repita)")
		fs.Var(&files, "file", "arquivo (repita)")
		if _, err := parseArgs(fs, rest); err != nil {
			return err
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		sess, err := s.App.LogSession(app.SessionInput{Summary: *summary, Done: done, Decisions: decisions, NextSteps: next,
			Blockers: blockers, Commands: commands, Tasks: tasks, Files: files, Agent: *agent}, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Sessão registrada em %s\n", sess.File)
		return nil
	case "list", "ls":
		limit := fs.Int("limit", 10, "quantidade")
		if _, err := parseArgs(fs, rest); err != nil {
			return err
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		list, err := s.App.Sessions(*limit)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("Nenhuma sessão registrada.")
		}
		for _, se := range list {
			who := plan.DisplayName(se.Author)
			if se.Agent != "" {
				who += " (" + se.Agent + ")"
			}
			summary, _, _ := strings.Cut(se.Summary, "\n")
			fmt.Printf("  %s  %s: %s\n", se.ID, who, summary)
		}
		return nil
	}
	return fmt.Errorf("subcomando desconhecido: session %s\n\n%s", sub, sessionUsage)
}

func cmdRemember(args []string) error {
	fs := flag.NewFlagSet("remember", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	title := fs.String("title", "", "título")
	body := fs.String("body", "", "o fato, curto e verificável")
	typ := fs.String("type", "contexto", strings.Join(model.NoteTypes, " | "))
	tags := fs.String("tags", "", "tags separadas por vírgula")
	comps := fs.String("components", "", "componentes separados por vírgula")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	n, err := s.App.Remember(app.NoteInput{Title: *title, Body: *body, Type: *typ, Tags: splitComma(*tags), Components: splitComma(*comps)}, hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ Memória gravada em %s\n", n.File)
	return nil
}

func cmdRecall(args []string) error {
	fs := flag.NewFlagSet("recall", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	typ := fs.String("type", "", "tipo")
	comp := fs.String("component", "", "componente")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	notes, err := s.App.Recall(strings.Join(pos, " "), *typ, *comp, 20)
	if err != nil {
		return err
	}
	if len(notes) == 0 {
		fmt.Println("Nenhuma memória encontrada.")
	}
	for _, n := range notes {
		fmt.Printf("\n## %s  [%s]\n%s\n", n.Title, n.Type, n.Body)
	}
	return nil
}

// cmdHook roda um hook do Git (commit-msg, pre-push). Sai com 1 para
// bloquear o commit/push.
func cmdHook(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: archcode-studio hook commit-msg <arquivo> | pre-push")
	}
	if code := studio.RunHook(args[0], args[1:], os.Stdin, os.Stdout, os.Stderr, version); code != 0 {
		return exitCode(code)
	}
	return nil
}

// cmdComplete conclui uma tarefa informando os checks das skills
// (--check skill/check=ok:"evidência"). Sem os obrigatórios, lista o que falta.
func cmdComplete(args []string) error {
	fs := flag.NewFlagSet("complete", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	notes := fs.String("notes", "", "o que foi entregue e como foi testado")
	status := fs.String("status", "", "review ou completed (padrão: review com branch, completed sem)")
	force := fs.Bool("force", false, "conclui mesmo sem os checks obrigatórios (fica registrado)")
	var checks listFlag
	fs.Var(&checks, "check", `resultado de um check: skill/check=ok|fail|na[:evidência] (repita)`)
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	id := ""
	if len(pos) > 0 {
		id = pos[0]
	} else {
		r, err := s.App.Resume("brief", "")
		if err != nil {
			return err
		}
		if len(r.MyWork) != 1 {
			return fmt.Errorf("informe a tarefa: archcode-studio complete TASK-001")
		}
		id = r.MyWork[0].ID
	}
	in := app.CompleteInput{Notes: *notes, Status: *status, Force: *force}
	for _, c := range checks {
		key, value, ok := strings.Cut(c, "=")
		skill, check, ok2 := strings.Cut(key, "/")
		if !ok || !ok2 {
			return fmt.Errorf("check inválido %q: use skill/check=ok[:evidência]", c)
		}
		result, evidence, _ := strings.Cut(value, ":")
		in.Checks = append(in.Checks, model.CheckResult{Skill: skill, Check: check, Result: result, Evidence: evidence})
	}
	res, err := s.App.CompleteTask(id, in, hub.SourceCLI)
	if err != nil {
		return err
	}
	if !res.Completed {
		fmt.Println(res.Message)
		fmt.Println("\nChecks obrigatórios:")
		for _, rc := range res.Required {
			line := fmt.Sprintf("  --check %s/%s=ok:\"…\"   %s", rc.Skill, rc.Check, rc.Text)
			if rc.Command != "" {
				line += "  (" + rc.Command + ")"
			}
			fmt.Println(line)
		}
		return exitCode(1)
	}
	fmt.Printf("✓ %s\n", res.Message)
	for _, w := range res.Warnings {
		fmt.Printf("  ⚠ %s\n", w)
	}
	for _, n := range res.NextSteps {
		fmt.Printf("  → %s\n", n)
	}
	return nil
}
