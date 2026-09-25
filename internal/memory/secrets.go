package memory

import (
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Segredos em memória, sessões e checkpoints (RNF011)
// ---------------------------------------------------------------------------
//
// Tudo o que a memória grava vai para o Git e é lido por outras pessoas e
// agentes. Antes de gravar, o texto passa por estes padrões; um achado recusa
// a escrita. Não substitui um scanner completo (a skill segredos-e-config
// roda o gitleaks), mas pega os vazamentos mais comuns na origem.

var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"chave privada", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"chave de acesso AWS", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"token do GitHub", regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b|\bgithub_pat_[A-Za-z0-9_]{50,}\b`)},
	{"token do GitLab", regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`)},
	{"chave de API da Anthropic", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`)},
	{"chave de API", regexp.MustCompile(`\bsk-(live|proj|test)?[_-]?[A-Za-z0-9]{24,}\b`)},
	{"token do Slack", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`)},
	{"chave do Google", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{"senha em texto", regexp.MustCompile(`(?i)\b(password|passwd|senha|secret|api[_-]?key|token)\s*[:=]\s*['"]?[^\s'"<>{}$]{8,}`)},
	{"URL com credencial", regexp.MustCompile(`[a-z][a-z0-9+.-]*://[^/\s:@]+:[^/\s:@]{4,}@`)},
}

// placeholders comuns que não são segredos de verdade.
var secretPlaceholders = []string{"example", "exemplo", "changeme", "xxxx", "****", "<", "${", "placeholder", "dummy", "fake"}

// FindSecrets devolve o nome dos tipos de segredo encontrados no texto.
func FindSecrets(texts ...string) []string {
	found := []string{}
	seen := map[string]bool{}
	for _, t := range texts {
		for _, p := range secretPatterns {
			for _, m := range p.re.FindAllString(t, -1) {
				if looksLikePlaceholder(m) || seen[p.name] {
					continue
				}
				seen[p.name] = true
				found = append(found, p.name)
			}
		}
	}
	return found
}

func looksLikePlaceholder(s string) bool {
	low := strings.ToLower(s)
	for _, ph := range secretPlaceholders {
		if strings.Contains(low, ph) {
			return true
		}
	}
	return false
}
