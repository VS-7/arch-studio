package model

import (
	"errors"
	"fmt"
)

// Tipos de erro do domínio. Os adaptadores traduzem o tipo, e não a mensagem:
// a API HTTP responde 404 para ErrNotFound e 400 para ErrInvalid, o MCP e o
// app desktop mostram a mensagem como está.
var (
	ErrNotFound = errors.New("não encontrado")
	ErrInvalid  = errors.New("dados inválidos")
)

type kindError struct {
	kind error
	msg  string
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }

// NotFound cria um erro legível que satisfaz errors.Is(err, ErrNotFound).
func NotFound(format string, args ...any) error {
	return &kindError{kind: ErrNotFound, msg: fmt.Sprintf(format, args...)}
}

// Invalid cria um erro legível que satisfaz errors.Is(err, ErrInvalid).
func Invalid(format string, args ...any) error {
	return &kindError{kind: ErrInvalid, msg: fmt.Sprintf(format, args...)}
}
