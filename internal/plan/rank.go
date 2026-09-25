// Package plan é o motor puro do Módulo de Planejamento: gera o backlog a
// partir da arquitetura, sincroniza sem sobrescrever edições humanas, ordena
// por ranks fracionários, calcula prontidão, capacidade e relatórios de
// sprint. Não lê nem grava disco — isso é papel do pacote app.
package plan

import "strings"

// ---------------------------------------------------------------------------
// Ranks fracionários
// ---------------------------------------------------------------------------
//
// A posição de um item no backlog é uma string comparável em ordem
// lexicográfica. Mover um item escolhe uma chave entre as dos vizinhos, então
// só o arquivo do item movido muda — sem reescrever a ordem inteira, o que
// causaria conflitos de merge a cada reordenação (RNF009).

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func rankIndex(c byte) int { return strings.IndexByte(rankDigits, c) }

// validRank informa se a chave só usa dígitos base 62 e não termina em '0'.
func validRank(k string) bool {
	if k == "" || k[len(k)-1] == '0' {
		return false
	}
	for i := 0; i < len(k); i++ {
		if rankIndex(k[i]) < 0 {
			return false
		}
	}
	return true
}

// Between devolve uma chave estritamente entre a e b. a == "" é o início da
// lista e b == "" o fim. Chaves inválidas ou fora de ordem são tratadas como
// ausentes, para que um arquivo editado à mão nunca trave a reordenação.
func Between(a, b string) string {
	if a != "" && !validRank(a) {
		a = ""
	}
	if b != "" && !validRank(b) {
		b = ""
	}
	if a != "" && b != "" && a >= b {
		b = ""
	}
	return midpoint(a, b)
}

// After devolve uma chave depois de a.
func After(a string) string { return Between(a, "") }

// Before devolve uma chave antes de b.
func Before(b string) string { return Between("", b) }

func midpoint(a, b string) string {
	if b != "" {
		n := 0
		for n < len(b) {
			ca := byte('0')
			if n < len(a) {
				ca = a[n]
			}
			if ca != b[n] {
				break
			}
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + midpoint(rest, b[n:])
		}
	}
	da := 0
	if a != "" {
		da = rankIndex(a[0])
	}
	db := len(rankDigits)
	if b != "" {
		db = rankIndex(b[0])
	}
	if db-da > 1 {
		return string(rankDigits[(da+db)/2])
	}
	if len(b) > 1 {
		return b[:1]
	}
	rest := ""
	if len(a) > 1 {
		rest = a[1:]
	}
	return string(rankDigits[da]) + midpoint(rest, "")
}

// Spread devolve n chaves crescentes, igualmente espaçadas e curtas, para
// ordenar de uma vez um backlog recém-gerado.
func Spread(n int) []string {
	if n <= 0 {
		return []string{}
	}
	width, space := 2, 62*62
	for (n+1)*2 > space {
		width++
		space *= 62
	}
	step := space / (n + 1)
	out := make([]string, n)
	for i := range out {
		v := step * (i + 1)
		key := encodeRank(v, width)
		if key[len(key)-1] == '0' {
			key = encodeRank(v+1, width)
		}
		out[i] = key
	}
	return out
}

func encodeRank(v, width int) string {
	buf := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		buf[i] = rankDigits[v%62]
		v /= 62
	}
	return string(buf)
}
