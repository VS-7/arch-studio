package memory

import (
	"strings"
	"testing"
)

func TestFindSecrets(t *testing.T) {
	cases := map[string]string{
		"chave AKIAABCDEFGHIJKLMNOP no log":                "chave de acesso AWS",
		"token ghp_" + strings.Repeat("a", 36):             "token do GitHub",
		"DATABASE_URL=postgres://app:s3nhaForte@db:5432/x": "URL com credencial",
		"password: hunter2hunter2":                         "senha em texto",
		"-----BEGIN RSA PRIVATE KEY-----":                  "chave privada",
	}
	for text, want := range cases {
		got := FindSecrets(text)
		if len(got) == 0 || got[0] != want {
			t.Errorf("FindSecrets(%q) = %v; quer %q", text, got, want)
		}
	}
	for _, clean := range []string{
		"password: ${DB_PASSWORD}",
		"senha = <sua-senha-aqui>",
		"Rode go test ./... e confira o token JWT emitido",
		"postgres://localhost:5432/app",
		"api_key: changeme-please",
	} {
		if got := FindSecrets(clean); len(got) != 0 {
			t.Errorf("falso positivo em %q: %v", clean, got)
		}
	}
}
