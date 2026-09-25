package main

import (
	"flag"
	"fmt"

	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/studio"
)

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	name := fs.String("name", "", "nome do projeto")
	desc := fs.String("description", "", "descrição curta do projeto")
	author := fs.String("author", "", "autor principal")
	currency := fs.String("currency", "BRL", "moeda da precificação (BRL, USD, EUR)")
	empty := fs.Bool("empty", false, "não criar a arquitetura de exemplo")
	force := fs.Bool("force", false, "sobrescrever o manifest de um projeto existente")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := studio.Init(*dir, project.Options{
		ProjectName: *name, Description: *desc, Author: *author,
		Currency: *currency, Empty: *empty, Force: *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("✓ Projeto ArchCode Studio criado em %s\n\n", res.Root)
	for _, f := range res.Created {
		fmt.Printf("  criado   %s\n", f)
	}
	for _, f := range res.Skipped {
		fmt.Printf("  mantido  %s\n", f)
	}
	fmt.Printf("\nPróximos passos:\n")
	fmt.Printf("  1. archcode-studio serve                 # abrir o canvas e o planejamento no browser\n")
	fmt.Printf("  2. archcode-studio agent setup claude    # MCP, hooks de retomada e skills no Claude Code\n")
	fmt.Printf("  3. archcode-studio sprint plan --goal \"…\" --apply && archcode-studio sprint start 1\n")
	fmt.Printf("  4. archcode-studio hooks install         # valida commits e branches (num repositório Git)\n")
	return nil
}
