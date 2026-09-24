package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/pricing"
)

func cmdExport(args []string) error {
	if len(args) == 0 {
		return errors.New("informe o formato: svg | mermaid | openapi | proposal | requirements")
	}
	format := args[0]
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	out := fs.String("out", "", "arquivo de saída (padrão: stdout, quando aplicável)")
	mode := fs.String("mode", "engineering", "svg: executive | engineering")
	theme := fs.String("theme", "light", "svg: light | dark")
	transparent := fs.Bool("transparent", false, "svg: fundo transparente")
	style := fs.String("style", "canvas", "svg: canvas | document (figura de documento, preto no branco)")
	client := fs.String("client", "", "proposal: nome do cliente")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}
	application := s.App

	switch format {
	case "svg":
		svg, filename, err := application.DiagramSVG(app.DiagramSVGOptions{
			Executive:   *mode == "executive",
			Dark:        *theme == "dark",
			Transparent: *transparent,
			Document:    *style == "document",
		})
		if err != nil {
			return err
		}
		return writeOut(*out, svg, filename)

	case "mermaid", "mmd":
		text, err := application.DiagramMermaid()
		if err != nil {
			return err
		}
		return writeOut(*out, text, "")

	case "openapi":
		path, err := application.ExportOpenAPI(hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s gerado\n", path)
		return nil

	case "proposal", "proposta":
		res, err := application.GenerateProposal(app.ProposalOptions{
			ClientName: *client, IncludeDiagram: true, IncludeCloud: true,
		}, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s gerado — total: %s\n", res.FilePath, pricing.Money(res.Currency, res.TotalCost))
		return nil

	case "requirements", "requisitos", "reqdoc":
		// --out é relativo à raiz do projeto (ou absoluto dentro dela), pois o
		// Markdown referencia as figuras pela subpasta diagramas/ ao lado.
		target := *out
		if target != "" && filepath.IsAbs(target) {
			rel, err := filepath.Rel(s.Root(), target)
			if err != nil || strings.HasPrefix(rel, "..") {
				return fmt.Errorf("--out precisa ficar dentro do projeto (%s)", s.Root())
			}
			target = filepath.ToSlash(rel)
		}
		res, err := application.GenerateRequirementsDocument(target, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s gerado — %s\n", res.File, res.Summary())
		for _, img := range res.Images {
			fmt.Printf("  figura   %s\n", img)
		}
		return nil

	default:
		return fmt.Errorf("formato desconhecido: %q (use svg, mermaid, openapi, proposal ou requirements)", format)
	}
}

func writeOut(out, content, defaultName string) error {
	if out == "" {
		if defaultName == "" {
			fmt.Print(content)
			return nil
		}
		out = defaultName
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return err
	}
	abs, _ := filepath.Abs(out)
	fmt.Printf("✓ %s\n", abs)
	return nil
}
