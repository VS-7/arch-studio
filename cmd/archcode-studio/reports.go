package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/lint"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/store"
)

// Comandos que compilam ou imprimem relatórios do projeto.

func cmdPRD(args []string) error {
	fs := flag.NewFlagSet("prd", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	stack := fs.String("stack", "", "stack alvo, ex.: \"Go / React / PostgreSQL\"")
	granularity := fs.String("granularity", "detailed", "detailed | summary")
	noTests := fs.Bool("no-tests", false, "não gerar critérios Given-When-Then nem a tarefa E2E")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	application := s.App
	res, err := application.GenerateAIPRD(prd.Options{
		TargetStack:          *stack,
		IncludeTestScenarios: !*noTests,
		Granularity:          *granularity,
	}, hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s gerado\n", res.FilePath)
	fmt.Printf("  Tarefas: %d | Hash de integridade: %s | Progresso: %d%%\n",
		res.TotalTasks, res.Hash, res.Progress)
	fmt.Printf("  Espelho legível por máquina: %s\n", store.FileTasks)
	return nil
}

// ---------------------------------------------------------------------------
// estimate
// ---------------------------------------------------------------------------

func cmdEstimate(args []string) error {
	fs := flag.NewFlagSet("estimate", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	margin := fs.Float64("margin", -1, "margem de contingência (0.20 ou 20); padrão: pricing.yaml")
	asJSON := fs.Bool("json", false, "saída em JSON")
	detailed := fs.Bool("detailed", false, "mostrar de onde vem cada hora")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	est, err := s.App.Estimate(*margin)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(est)
	}

	cur := est.Currency
	fmt.Printf("\n  Estimativa do projeto\n")
	fmt.Printf("  ─────────────────────────────────────────────\n")
	fmt.Printf("  Componentes          %10s\n", pricing.Hours(est.NodeHours))
	fmt.Printf("  Integrações/APIs     %10s\n", pricing.Hours(est.EdgeHours))
	fmt.Printf("  Casos de uso         %10s\n", pricing.Hours(est.UseCaseHours))
	fmt.Printf("  Subtotal             %10s\n", pricing.Hours(est.BaseHours))
	fmt.Printf("  Margem (%3.0f%%)        %10s\n", est.RiskMarginPercentage, pricing.Hours(est.MarginHours))
	fmt.Printf("  TOTAL                %10s\n\n", pricing.Hours(est.TotalHours))

	for _, r := range est.Roles {
		fmt.Printf("  %-18s %8s × %12s = %14s\n",
			r.Role, pricing.Hours(r.Hours), pricing.Money(cur, r.Rate), pricing.Money(cur, r.Subtotal))
	}
	fmt.Printf("\n  Desenvolvimento      %18s\n", pricing.Money(cur, est.PersonnelCost))
	fmt.Printf("  Impostos (%3.0f%%)      %18s\n", est.TaxPercentage, pricing.Money(cur, est.TaxAmount))
	fmt.Printf("  TOTAL DO PROJETO     %18s\n", pricing.Money(cur, est.TotalCost))
	if est.CloudMonthlyCost > 0 {
		fmt.Printf("  Infraestrutura       %18s/mês\n", pricing.Money(cur, est.CloudMonthlyCost))
	}
	fmt.Printf("  Prazo                %.0f dias úteis (~%.1f meses, equipe de %.0f)\n",
		est.WorkingDays, est.CalendarMonths, est.TeamSize)

	if *detailed {
		fmt.Printf("\n  Detalhamento\n  ─────────────────────────────────────────────\n")
		for _, it := range est.Items {
			flag := " "
			if it.Explicit {
				flag = "*"
			}
			fmt.Printf("  %s %-30s %-12s %8s\n", flag, truncate(it.Label, 30), it.Kind, pricing.Hours(it.Hours))
		}
		fmt.Printf("\n  * horas definidas explicitamente no modelo\n")
	}
	for _, warn := range est.Warnings {
		fmt.Printf("\n  ⚠ %s", warn)
	}
	fmt.Println()
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	asJSON := fs.Bool("json", false, "saída em JSON")
	strict := fs.Bool("strict", false, "sair com código 1 também em avisos")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	rep, err := s.App.Validate()
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		icons := map[string]string{lint.SeverityError: "✗", lint.SeverityWarning: "▲", lint.SeverityInfo: "·"}
		fmt.Printf("\n  Validação da arquitetura — score %d/100\n", rep.Score)
		fmt.Printf("  ─────────────────────────────────────────────\n")
		if len(rep.Findings) == 0 {
			fmt.Printf("  ✓ Nenhum problema encontrado.\n\n")
		}
		for _, f := range rep.Findings {
			target := f.Target
			if target == "" {
				target = "arquitetura"
			}
			fmt.Printf("  %s [%s] %s\n      %s\n", icons[f.Severity], f.Rule, target, f.Message)
			if f.Fix != "" {
				fmt.Printf("      → %s\n", f.Fix)
			}
		}
		fmt.Printf("\n  %d erro(s), %d aviso(s), %d informativo(s)\n\n", rep.Errors, rep.Warnings, rep.Infos)
	}
	if rep.Errors > 0 || (*strict && rep.Warnings > 0) {
		return exitCode(1)
	}
	return nil
}
