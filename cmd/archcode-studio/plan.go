package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
)

// Comandos do Módulo de Planejamento: backlog e sprints.

// parseArgs aceita flags antes e depois dos argumentos posicionais
// (`claim TASK-1 --push` e `claim --push TASK-1`).
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	pos := []string{}
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// listFlag acumula uma flag repetida (--done a --done b).
type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ", ") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func subcommand(args []string, usage string) (string, []string, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return "", nil, exitCode(0)
	}
	return args[0], args[1:], nil
}

// ---------------------------------------------------------------------------
// backlog
// ---------------------------------------------------------------------------

const backlogUsage = `Uso: archcode-studio backlog <subcomando>

  sync [--dry-run]            Gera/reconcilia o backlog a partir da arquitetura
  list [filtros] [--json]     Lista itens (--status open|pending|in_progress|review|completed|blocked|all,
                              --sprint N, --type bug, --mine, --ready)
  show <ID>                   Mostra um item completo
  add --title "…" [--type task|bug|debt|spike|security] [--description --priority --sprint N
                  --parent ST-RF001 --component <id-do-nó> --estimate 4 --criterion "…"]
  set <ID> [--title --status --sprint N --priority --estimate --assignee]
  archive <ID>                Arquiva (itens gerados) ou apaga (itens manuais)
`

func cmdBacklog(args []string) error {
	sub, rest, err := subcommand(args, backlogUsage)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("backlog "+sub, flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	switch sub {
	case "sync":
		dry := fs.Bool("dry-run", false, "só mostra o que mudaria")
		if _, err := parseArgs(fs, rest); err != nil {
			return err
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		res, err := s.App.SyncBacklog(*dry, hub.SourceCLI)
		if err != nil {
			return err
		}
		verb := "Backlog sincronizado"
		if *dry {
			verb = "Prévia da sincronização (nada gravado)"
		}
		fmt.Printf("✓ %s — fatiamento %s\n", verb, res.Slicing)
		fmt.Printf("  %d novo(s) · %d atualizado(s) · %d arquivado(s) · %d restaurado(s) · %d item(ns) no total\n",
			res.Counts[plan.ChangeAdded], res.Counts[plan.ChangeUpdated], res.Counts[plan.ChangeArchived],
			res.Counts[plan.ChangeRestored], res.Total)
		for _, c := range res.Changes {
			line := fmt.Sprintf("  %-9s %-18s %s", c.Kind, c.ID, c.Title)
			if len(c.Fields) > 0 {
				line += " (" + strings.Join(c.Fields, ", ") + ")"
			}
			fmt.Println(line)
		}
		return nil
	case "list", "ls":
		status := fs.String("status", "open", "filtro de status")
		sprint := fs.Int("sprint", -1, "só itens da sprint (0 = sem sprint)")
		typ := fs.String("type", "", "tipo do item")
		mine := fs.Bool("mine", false, "só os seus")
		ready := fs.Bool("ready", false, "só itens prontos (dependências concluídas)")
		asJSON := fs.Bool("json", false, "saída em JSON")
		if _, err := parseArgs(fs, rest); err != nil {
			return err
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		q := app.BacklogQuery{Status: *status, Type: *typ, OnlyReady: *ready}
		if *sprint >= 0 {
			q.Sprint = sprint
		}
		if *mine {
			q.Assignee = s.App.Person()
		}
		items, total, err := s.App.Backlog(q)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(map[string]any{"items": items, "total": total})
		}
		if total == 0 {
			fmt.Println("Nenhum item. Backlog vazio? Rode: archcode-studio backlog sync")
			return nil
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tTIPO\tSTATUS\tSPRINT\tHORAS\tDONO\tTÍTULO")
		for _, it := range items {
			sp := "—"
			if it.Sprint > 0 {
				sp = model.SprintLabel(it.Sprint)
			}
			status := plan.StatusLabels[it.Status]
			if !it.Ready && it.Status == model.StatusPending && it.Actionable() {
				status += " ⏳"
			}
			if it.Stale {
				status += " (parada)"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", it.ID, model.ItemTypeLabels[it.Type], status, sp,
				hours(it.EstimateHours), orDash(plan.DisplayName(it.Assignee)), it.Title)
		}
		tw.Flush()
		fmt.Printf("\n%d item(ns). ⏳ = aguarda dependências.\n", total)
		return nil
	case "show":
		asJSON := fs.Bool("json", false, "saída em JSON")
		pos, err := parseArgs(fs, rest)
		if err != nil {
			return err
		}
		if len(pos) != 1 {
			return fmt.Errorf("informe o id: archcode-studio backlog show TASK-001")
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		it, err := s.App.Item(pos[0])
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(it)
		}
		md, err := model.RenderWorkItem(it)
		if err != nil {
			return err
		}
		fmt.Print(md)
		return nil
	case "add", "set":
		title := fs.String("title", "", "título")
		typ := fs.String("type", "", "task | bug | debt | spike | security")
		desc := fs.String("description", "", "descrição")
		prio := fs.String("priority", "", "must | should | could | wont")
		status := fs.String("status", "", "status (set)")
		sprint := fs.Int("sprint", -1, "sprint (0 = backlog)")
		parent := fs.String("parent", "", "item pai")
		comp := fs.String("component", "", "id do componente do diagrama")
		est := fs.Float64("estimate", -1, "estimativa em horas")
		assignee := fs.String("assignee", "", "dono (set)")
		var criteria listFlag
		fs.Var(&criteria, "criterion", "critério de aceite (repita a flag)")
		pos, err := parseArgs(fs, rest)
		if err != nil {
			return err
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		in := app.ItemInput{}
		setIf := func(v string, dst **string) {
			if v != "" {
				v := v
				*dst = &v
			}
		}
		setIf(*title, &in.Title)
		setIf(*typ, &in.Type)
		setIf(*desc, &in.Description)
		setIf(*prio, &in.Priority)
		setIf(*status, &in.Status)
		setIf(*parent, &in.Parent)
		setIf(*comp, &in.ComponentID)
		setIf(*assignee, &in.Assignee)
		if *sprint >= 0 {
			in.Sprint = sprint
		}
		if *est >= 0 {
			in.EstimateHours = est
		}
		if len(criteria) > 0 {
			list := []model.Criterion{}
			for _, c := range criteria {
				list = append(list, model.Criterion{Text: c})
			}
			in.Acceptance = &list
		}
		var it *model.WorkItem
		if sub == "add" {
			it, err = s.App.CreateItem(in, hub.SourceCLI)
		} else {
			if len(pos) != 1 {
				return fmt.Errorf("informe o id: archcode-studio backlog set TASK-001 --status completed")
			}
			it, err = s.App.UpdateItem(pos[0], in, hub.SourceCLI)
		}
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s %s — %s (%s)\n", it.ID, it.Title, plan.StatusLabels[it.Status], it.File)
		return nil
	case "archive", "rm":
		pos, err := parseArgs(fs, rest)
		if err != nil {
			return err
		}
		if len(pos) != 1 {
			return fmt.Errorf("informe o id")
		}
		s, err := openProject(*dir)
		if err != nil {
			return err
		}
		if err := s.App.DeleteItem(pos[0], hub.SourceCLI); err != nil {
			return err
		}
		fmt.Printf("✓ %s removido do backlog\n", strings.ToUpper(pos[0]))
		return nil
	}
	return fmt.Errorf("subcomando desconhecido: backlog %s\n\n%s", sub, backlogUsage)
}

// ---------------------------------------------------------------------------
// sprint
// ---------------------------------------------------------------------------

const sprintUsage = `Uso: archcode-studio sprint <subcomando>

  status [N]                     Sprint ativa (ou a N): progresso, carga, itens
  list                           Todas as sprints
  new [--goal --start AAAA-MM-DD --end AAAA-MM-DD --capacity H]
  plan [N] [--goal --capacity H] [--apply]   Propõe (e com --apply, aplica) os itens que cabem
  start N                        Ativa a sprint (uma por vez)
  close [N] [--carry backlog|next|<N>]       Encerra: relatório, transferência, CHANGELOG
`

func cmdSprint(args []string) error {
	sub, rest, err := subcommand(args, sprintUsage)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("sprint "+sub, flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	goal := fs.String("goal", "", "meta da sprint")
	capacity := fs.Float64("capacity", -1, "capacidade em horas")
	start := fs.String("start", "", "início (AAAA-MM-DD)")
	end := fs.String("end", "", "fim (AAAA-MM-DD)")
	apply := fs.Bool("apply", false, "aplica a proposta")
	carry := fs.String("carry", "backlog", "destino dos itens não concluídos: backlog, next ou o número")
	asJSON := fs.Bool("json", false, "saída em JSON")
	pos, err := parseArgs(fs, rest)
	if err != nil {
		return err
	}
	num := 0
	if len(pos) > 0 {
		if num, err = strconv.Atoi(pos[0]); err != nil || num < 0 {
			return fmt.Errorf("número de sprint inválido: %q", pos[0])
		}
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	a := s.App
	switch sub {
	case "status":
		st, err := a.Sprint(num)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(st)
		}
		if st.Sprint == nil {
			fmt.Printf("Nenhuma sprint ativa ou planejada. Backlog: %d item(ns).\nCrie uma: archcode-studio sprint plan --goal \"…\"\n", st.Backlog.Total)
			return nil
		}
		sp := st.Sprint
		fmt.Printf("\n  %s — %s  [%s]\n", sp.Name, orDash(sp.Goal), sp.Status)
		fmt.Printf("  %s a %s · %d dia(s) útil(eis) restante(s)\n", orDash(sp.Start), orDash(sp.End), st.DaysLeft)
		fmt.Printf("  %d/%d concluídas (%d%%) · %d em andamento · %d em revisão · %d bloqueadas\n",
			st.Stats.Completed, st.Stats.Total, st.Stats.Progress, st.Stats.InProgress, st.Stats.Review, st.Stats.Blocked)
		fmt.Printf("  Carga %s de %s de capacidade\n\n", hours(st.Load), hours(sp.CapacityHours))
		tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		for _, it := range st.Items {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", it.ID, plan.StatusLabels[it.Status], orDash(plan.DisplayName(it.Assignee)), it.Title)
		}
		tw.Flush()
		return nil
	case "list", "ls":
		st, err := a.Sprint(0)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(st.Sprints)
		}
		if len(st.Sprints) == 0 {
			fmt.Println("Nenhuma sprint. Crie uma: archcode-studio sprint plan --goal \"…\"")
		}
		for _, sp := range st.Sprints {
			fmt.Printf("  %s  %-8s %s a %s  %s\n", sp.Name, sp.Status, orDash(sp.Start), orDash(sp.End), sp.Goal)
		}
		return nil
	case "new":
		in := app.SprintInput{Number: num}
		if *goal != "" {
			in.Goal = goal
		}
		if *start != "" {
			in.Start = start
		}
		if *end != "" {
			in.End = end
		}
		if *capacity >= 0 {
			in.CapacityHours = capacity
		}
		sp, err := a.CreateSprint(in, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s criada: %s a %s, capacidade %s\n  Planeje: archcode-studio sprint plan %d\n", sp.Name, sp.Start, sp.End, hours(sp.CapacityHours), sp.Number)
		return nil
	case "plan":
		c := 0.0
		if *capacity > 0 {
			c = *capacity
		}
		res, err := a.PlanSprint(num, *goal, c, nil, *apply, hub.SourceCLI)
		if err != nil {
			return err
		}
		if *asJSON {
			return printJSON(res)
		}
		sp := res.Sprint
		fmt.Printf("\n  %s — %s (%s a %s)\n", sp.Name, orDash(sp.Goal), orDash(sp.Start), orDash(sp.End))
		fmt.Printf("  %d item(ns), %s de %s de capacidade\n\n", len(res.Items), hours(res.Proposal.Hours), hours(sp.CapacityHours))
		for _, it := range res.Items {
			fmt.Printf("  %-18s %6s  %s\n", it.ID, hours(it.EstimateHours), it.Title)
		}
		for _, w := range res.Warnings {
			fmt.Printf("\n  ⚠ %s\n", w)
		}
		if len(res.Proposal.Skipped) > 0 {
			fmt.Printf("\n  Prontos que não couberam: %s\n", strings.Join(res.Proposal.Skipped, ", "))
		}
		if res.Applied {
			fmt.Printf("\n✓ Plano aplicado. Inicie com: archcode-studio sprint start %d\n", sp.Number)
		} else {
			fmt.Printf("\nNada foi gravado. Para aplicar: archcode-studio sprint plan %d --apply\n", sp.Number)
		}
		return nil
	case "start":
		if num <= 0 {
			return fmt.Errorf("informe o número: archcode-studio sprint start 1")
		}
		sp, err := a.StartSprint(num, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s ativa (%s a %s)\n", sp.Name, sp.Start, sp.End)
		return nil
	case "close":
		carryTo := 0
		switch *carry {
		case "backlog", "":
		case "next", "proxima", "próxima":
			carryTo = -1
		default:
			if carryTo, err = strconv.Atoi(*carry); err != nil || carryTo <= 0 {
				return fmt.Errorf("--carry deve ser backlog, next ou o número da sprint")
			}
		}
		res, err := a.CloseSprint(num, carryTo, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s encerrada: %d/%d concluídas (%d%%)\n", res.Sprint.Name, res.Stats.Completed, res.Stats.Total, res.Stats.Progress)
		fmt.Printf("  Relatório: %s\n", res.Report)
		if res.Changelog != "" {
			fmt.Printf("  Changelog: %s\n", res.Changelog)
		}
		if len(res.Carried) > 0 {
			dest := "o backlog"
			if res.CarriedTo > 0 {
				dest = model.SprintName(res.CarriedTo)
			}
			fmt.Printf("  Transferidos para %s: %s\n", dest, strings.Join(res.Carried, ", "))
		}
		fmt.Printf("\n  Tag sugerida (criá-la é decisão sua):\n    %s\n", res.TagCommand)
		return nil
	}
	return fmt.Errorf("subcomando desconhecido: sprint %s\n\n%s", sub, sprintUsage)
}

func hours(h float64) string {
	if h <= 0 {
		return "—"
	}
	return strconv.FormatFloat(h, 'f', -1, 64) + " h"
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
