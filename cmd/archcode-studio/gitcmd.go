package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/skills"
	"github.com/archcode/studio/internal/studio"
)

// Comandos de Git: convenções, commit, PR, lint, hooks, changelog, doctor.

// confirm pergunta sim/não no terminal (padrão: não).
func confirm(question string) bool {
	fmt.Printf("%s [s/N] ", question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "s", "sim", "y", "yes":
		return true
	}
	return false
}

func cmdCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	summary := fs.String("m", "", "resumo (padrão: gerado do título da tarefa)")
	body := fs.String("body", "", "corpo: por que a mudança foi feita")
	coauthor := fs.String("co-author", "", "trailer Co-Authored-By")
	dry := fs.Bool("dry-run", false, "só mostra a mensagem")
	planning := fs.Bool("plan", false, "commit do planejamento: prepara .arch/ e usa o id da sprint ([SPRINT-01])")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	req := app.CommitRequest{Summary: *summary, Body: *body, CoAuthor: *coauthor, Commit: !*dry, Plan: *planning}
	if len(pos) > 0 {
		req.TaskID = pos[0]
	}
	res, err := s.App.ProposeCommit(req)
	if err != nil {
		return err
	}
	if res.Committed {
		fmt.Printf("✓ %s %s\n", shortHash(res.Hash), res.Subject)
	} else {
		fmt.Print(res.Message)
	}
	for _, w := range res.Warnings {
		fmt.Printf("⚠ %s\n", w)
	}
	return nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func cmdPR(args []string) error {
	fs := flag.NewFlagSet("pr", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	dry := fs.Bool("dry-run", false, "só mostra título e corpo")
	draft := fs.Bool("draft", false, "abre como rascunho")
	yes := fs.Bool("yes", false, "não pede confirmação")
	pos, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	pr, err := s.App.PreparePullRequest(pos)
	if err != nil {
		return err
	}
	fmt.Printf("# %s\n\n%s\n", pr.Title, pr.Body)
	for _, w := range pr.Warnings {
		fmt.Printf("⚠ %s\n", w)
	}
	if *dry {
		return nil
	}
	if !*yes && !confirm(fmt.Sprintf("Publicar %s e abrir o PR para %s com o gh?", pr.Head, pr.Base)) {
		fmt.Println("Nada foi publicado.")
		return nil
	}
	opened, err := s.App.OpenPullRequest(pos, *draft)
	if err != nil {
		return err
	}
	fmt.Printf("✓ PR aberto: %s\n", opened.URL)
	return nil
}

const conventionsUsage = `Uso: archcode-studio conventions <subcomando>

  init [--preset archcode-sprint|conventional-commits] [--force]   Cria .arch/conventions.yaml
  show                         Mostra as convenções e exemplos
  apply [--ci]                 Grava o template de PR (e, com --ci, o workflow de git lint)
`

func cmdConventions(args []string) error {
	sub, rest, err := subcommand(args, conventionsUsage)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("conventions "+sub, flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	preset := fs.String("preset", model.PresetArchCode, "archcode-sprint | conventional-commits")
	force := fs.Bool("force", false, "recria o arquivo")
	ci := fs.Bool("ci", false, "grava também .github/workflows/archcode-conventions.yml")
	if _, err := parseArgs(fs, rest); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	switch sub {
	case "init":
		c, err := s.App.InitConventions(*preset, *force, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ .arch/conventions.yaml criado (preset %s)\n", c.Preset)
		printPreview(s.App.PreviewConventions(c))
		return nil
	case "show":
		c, err := s.App.Conventions()
		if err != nil {
			return err
		}
		if !s.App.HasConventions() {
			fmt.Println("Sem .arch/conventions.yaml: valem os padrões abaixo. Crie com: archcode-studio conventions init")
		}
		fmt.Printf("Preset: %s · autonomia da IA: %s · fatiamento: %s · sprint de %d dias úteis\n",
			c.Preset, model.AutonomyLabel(c.AI.Autonomy), c.Planning.Slicing, c.Planning.SprintDays)
		printPreview(s.App.PreviewConventions(c))
		return nil
	case "apply":
		files, err := s.App.ApplyConventions(*ci, hub.SourceCLI)
		if err != nil {
			return err
		}
		for _, f := range files {
			fmt.Printf("✓ %s\n", f)
		}
		return nil
	}
	return fmt.Errorf("subcomando desconhecido: conventions %s\n\n%s", sub, conventionsUsage)
}

func printPreview(p *app.ConventionsPreview) {
	fmt.Printf("  Branch:           %s\n", p.Branch)
	fmt.Printf("  Branch (fora):    %s\n", p.BranchOffSprint)
	fmt.Printf("  Commit:           %s\n", p.Commit)
	fmt.Printf("  Commit (fora):    %s\n", p.CommitOffSprint)
	fmt.Printf("  Pull request:     %s\n", p.PRTitle)
	fmt.Printf("  Tag de sprint:    %s\n", p.SprintTag)
	for _, e := range p.Errors {
		fmt.Printf("  ⚠ %s\n", e)
	}
}

func cmdHooks(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: archcode-studio hooks install [--force] | uninstall")
	}
	fs := flag.NewFlagSet("hooks", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	force := fs.Bool("force", false, "sobrescreve hooks de outras ferramentas")
	if _, err := parseArgs(fs, args[1:]); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	switch args[0] {
	case "install":
		res, err := s.App.InstallHooks(studio.Executable(), *force)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Hooks instalados em %s: %s\n", res.Dir, strings.Join(res.Installed, ", "))
		for _, n := range res.Notes {
			fmt.Printf("  %s\n", n)
		}
		if !s.App.HasConventions() {
			fmt.Println("  Os hooks só passam a validar quando existir .arch/conventions.yaml (archcode-studio conventions init).")
		}
		return nil
	case "uninstall":
		removed, err := s.App.UninstallHooks()
		if err != nil {
			return err
		}
		fmt.Printf("✓ Removidos: %s\n", orDash(strings.Join(removed, ", ")))
		return nil
	}
	return fmt.Errorf("uso: archcode-studio hooks install [--force] | uninstall")
}

func cmdGit(args []string) error {
	if len(args) == 0 || args[0] != "lint" {
		return fmt.Errorf("uso: archcode-studio git lint [--range origin/main..HEAD] [--branch nome] [--pr-title \"…\"] [--json]")
	}
	fs := flag.NewFlagSet("git lint", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	rng := fs.String("range", "", "intervalo de commits (padrão: <principal>..HEAD)")
	branch := fs.String("branch", "", "nome da branch (padrão: a atual)")
	prTitle := fs.String("pr-title", "", "título do pull request")
	asJSON := fs.Bool("json", false, "saída em JSON")
	if _, err := parseArgs(fs, args[1:]); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	res, err := s.App.GitLint(app.LintRequest{Range: *rng, Branch: *branch, PRTitle: *prTitle})
	if err != nil {
		return err
	}
	if *asJSON {
		if err := printJSON(map[string]any{"checked": res.Checked, "issues": res.Issues, "errors": res.Errors()}); err != nil {
			return err
		}
	} else {
		for _, is := range res.Issues {
			level := "aviso"
			if is.Level == conventions.LevelError {
				level = "ERRO "
			}
			ref := ""
			if is.Ref != "" {
				ref = is.Ref + " "
			}
			fmt.Printf("  [%s] %s%q\n          %s\n", level, ref, is.Subject, is.Message)
		}
		if res.OK() {
			fmt.Printf("✓ %d verificação(ões) na convenção\n", res.Checked)
		} else {
			fmt.Printf("\n✗ %d erro(s) em %d verificação(ões)\n", res.Errors(), res.Checked)
		}
	}
	if !res.OK() {
		return exitCode(1)
	}
	return nil
}

func cmdChangelog(args []string) error {
	fs := flag.NewFlagSet("changelog", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	write := fs.Bool("write", false, "grava o CHANGELOG.md")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	content, err := s.App.Changelog(*write)
	if err != nil {
		return err
	}
	if *write {
		fmt.Println("✓ CHANGELOG.md atualizado")
		return nil
	}
	fmt.Print(content)
	return nil
}

func cmdReconcile(args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	apply := fs.Bool("apply", false, "grava as mudanças de status")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	changes, err := s.App.Reconcile(*apply, hub.SourceCLI)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		fmt.Println("✓ O quadro já está de acordo com o Git.")
		return nil
	}
	for _, c := range changes {
		fmt.Printf("  %-18s %s → %s  (%s)\n", c.ID, plan.StatusLabels[c.From], plan.StatusLabels[c.To], c.Reason)
	}
	if *apply {
		fmt.Printf("✓ %d tarefa(s) atualizada(s)\n", len(changes))
	} else {
		fmt.Println("\nNada foi gravado. Para aplicar: archcode-studio reconcile --apply")
	}
	return nil
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	prefer := fs.String("prefer", "", "resolve conflitos ficando com ours (local) ou theirs (o que veio)")
	fix := fs.Bool("fix", false, "regenera os arquivos derivados (tasks.json, MEMORY.md)")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	rep, err := s.App.Doctor(*prefer, *fix)
	if err != nil {
		return err
	}
	for _, c := range rep.Conflicts {
		fmt.Printf("  ✗ conflito de merge: %s\n", c)
	}
	for _, r := range rep.Resolved {
		fmt.Printf("  ✓ resolvido (%s): %s\n", *prefer, r)
	}
	for _, p := range rep.Problems {
		fmt.Printf("  ⚠ %s\n", p)
	}
	for _, r := range rep.Regenerated {
		fmt.Printf("  ✓ regenerado: %s\n", r)
	}
	if len(rep.Conflicts) == 0 && len(rep.Problems) == 0 {
		fmt.Println("✓ Backlog, sprints e memória sem problemas.")
		return nil
	}
	if len(rep.Conflicts) > len(rep.Resolved) {
		fmt.Println("\nPara resolver: archcode-studio doctor --prefer ours|theirs (ou edite os arquivos).")
		return exitCode(1)
	}
	return nil
}

// cmdMergeDriver é o merge driver dos arquivos derivados (tasks.json,
// ai-prd.md, MEMORY.md): mantém a versão local, que o Studio regenera a
// partir do backlog na próxima escrita ou com `doctor --fix`.
func cmdMergeDriver(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("uso: archcode-studio merge-driver %%O %%A %%B [%%P]")
	}
	path := args[len(args)-1]
	fmt.Fprintf(os.Stderr, "archcode-studio: %s é derivado do backlog; mantida a versão local (rode `archcode-studio doctor --fix` para regenerar)\n", path)
	return nil
}

// ---------------------------------------------------------------------------
// Skills e agentes
// ---------------------------------------------------------------------------

const skillsUsage = `Uso: archcode-studio skills <subcomando>

  list                        Instaladas, catálogo e sugestões para o stack
  add <nome…> | --suggested   Instala do catálogo
  update [nome…]              Reinstala do catálogo (perde edições locais)
  remove <nome>               Desinstala
  enable <nome> | disable <nome>
  sync [--targets claude,agents,cursor]   Exporta para os agentes
`

func cmdSkills(args []string) error {
	sub, rest, err := subcommand(args, skillsUsage)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("skills "+sub, flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	suggested := fs.Bool("suggested", false, "instala as sugeridas para o projeto")
	targets := fs.String("targets", "claude,agents", "destinos: claude, agents, cursor")
	pos, err := parseArgs(fs, rest)
	if err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	a := s.App
	switch sub {
	case "list", "ls":
		ov, err := a.Skills()
		if err != nil {
			return err
		}
		fmt.Printf("Stacks detectados: %s\n\nInstaladas:\n", orDash(strings.Join(ov.Stacks, ", ")))
		if len(ov.Installed) == 0 {
			fmt.Println("  nenhuma — instale as sugeridas com: archcode-studio skills add --suggested")
		}
		for _, sk := range ov.Installed {
			state := "ativa"
			if sk.Disabled {
				state = "desligada"
			}
			if sk.UpdateAvailable {
				state += ", atualização disponível"
			}
			fmt.Printf("  %-26s %-14s %s\n", sk.Name, skills.CategoryLabels[sk.Category], state)
		}
		fmt.Println("\nCatálogo:")
		sug := map[string]bool{}
		for _, n := range ov.Suggested {
			sug[n] = true
		}
		for _, sk := range ov.Catalog {
			if sk.Installed {
				continue
			}
			mark := ""
			if sug[sk.Name] {
				mark = " (sugerida)"
			}
			fmt.Printf("  %-26s %-14s%s\n", sk.Name, skills.CategoryLabels[sk.Category], mark)
		}
		for _, w := range ov.Warnings {
			fmt.Printf("\n⚠ %s", w)
		}
		fmt.Println()
		return nil
	case "add", "install", "update":
		names := pos
		if *suggested || (sub == "update" && len(names) == 0) {
			ov, err := a.Skills()
			if err != nil {
				return err
			}
			if *suggested {
				names = append(names, ov.Suggested...)
			} else {
				for _, sk := range ov.Installed {
					if sk.Builtin {
						names = append(names, sk.Name)
					}
				}
			}
		}
		if len(names) == 0 {
			return fmt.Errorf("informe as skills (veja: archcode-studio skills list)")
		}
		done, err := a.InstallSkills(names, sub == "update", hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %d skill(s) %s: %s\n", len(done), map[bool]string{true: "atualizada(s)", false: "instalada(s)"}[sub == "update"], orDash(strings.Join(done, ", ")))
		fmt.Println("  Exporte para os agentes: archcode-studio skills sync")
		return nil
	case "remove", "rm":
		if len(pos) != 1 {
			return fmt.Errorf("informe a skill")
		}
		if err := a.RemoveSkill(pos[0], hub.SourceCLI); err != nil {
			return err
		}
		fmt.Printf("✓ %s removida (rode skills sync para limpar os agentes)\n", pos[0])
		return nil
	case "enable", "disable":
		if len(pos) != 1 {
			return fmt.Errorf("informe a skill")
		}
		if _, err := a.SetSkillEnabled(pos[0], sub == "enable", hub.SourceCLI); err != nil {
			return err
		}
		fmt.Printf("✓ %s %s\n", pos[0], map[bool]string{true: "ativada", false: "desligada"}[sub == "enable"])
		return nil
	case "sync":
		res, err := a.SyncSkills(splitComma(*targets), hub.SourceCLI)
		if err != nil {
			return err
		}
		for _, f := range res.Written {
			fmt.Printf("  ✓ %s\n", f)
		}
		for _, f := range res.Removed {
			fmt.Printf("  − %s\n", f)
		}
		fmt.Printf("✓ Skills sincronizadas (%d escrito(s), %d removido(s))\n", len(res.Written), len(res.Removed))
		return nil
	}
	return fmt.Errorf("subcomando desconhecido: skills %s\n\n%s", sub, skillsUsage)
}

func cmdAgent(args []string) error {
	if len(args) < 2 || args[0] != "setup" {
		return fmt.Errorf("uso: archcode-studio agent setup claude|cursor")
	}
	fs := flag.NewFlagSet("agent setup", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	if _, err := parseArgs(fs, args[2:]); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	res, err := s.App.AgentSetup(args[1], hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s configurado para o ArchCode Studio\n", res.Agent)
	for _, f := range res.Written {
		fmt.Printf("  ✓ %s\n", f)
	}
	for _, n := range res.Notes {
		fmt.Printf("  %s\n", n)
	}
	return nil
}

// cmdCheck roda os comandos dos checks das skills ativas (localmente ou na
// CI). Comandos cujo programa não está instalado aparecem como indisponíveis.
func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	only := fs.String("skill", "", "só esta skill")
	task := fs.String("task", "", "só as skills que valem para esta tarefa")
	if _, err := parseArgs(fs, args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	list, err := s.App.SkillsForTask(*task)
	if err != nil {
		return err
	}
	failed, ran := 0, 0
	for _, sk := range list {
		if *only != "" && sk.Name != *only {
			continue
		}
		for _, c := range sk.Checks {
			if c.Command == "" {
				continue
			}
			bin := strings.Fields(c.Command)[0]
			if _, err := exec.LookPath(bin); err != nil {
				fmt.Printf("  –  %s/%s: %s não instalado (%s)\n", sk.Name, c.ID, bin, c.Command)
				continue
			}
			ran++
			start := time.Now()
			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.Command("cmd", "/C", c.Command)
			} else {
				cmd = exec.Command("sh", "-c", c.Command)
			}
			cmd.Dir = s.Root()
			out, err := cmd.CombinedOutput()
			mark := "✓"
			if err != nil {
				mark = "✗"
				if c.Required {
					failed++
				}
			}
			fmt.Printf("  %s  %s/%s (%s, %.1fs)\n", mark, sk.Name, c.ID, c.Command, time.Since(start).Seconds())
			if err != nil {
				for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
					if line != "" {
						fmt.Printf("       %s\n", line)
					}
				}
			}
		}
	}
	if ran == 0 {
		fmt.Println("Nenhum check com comando nas skills ativas.")
		return nil
	}
	if failed > 0 {
		fmt.Printf("\n✗ %d check(s) obrigatório(s) falharam\n", failed)
		return exitCode(1)
	}
	fmt.Printf("\n✓ %d check(s) executado(s)\n", ran)
	return nil
}
