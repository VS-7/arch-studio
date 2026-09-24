package hub

import (
	"sync"
	"testing"
)

func TestAssinantesRecebemEventos(t *testing.T) {
	h := New()
	a, cancelA := h.Subscribe(4)
	b, cancelB := h.Subscribe(4)
	defer cancelB()

	h.Broadcast(Event{Type: EventDocs, Source: SourceUI})
	for _, ch := range []<-chan Event{a, b} {
		ev := <-ch
		if ev.Type != EventDocs || ev.At == "" {
			t.Fatalf("evento inesperado: %+v", ev)
		}
	}

	cancelA()
	if _, ok := <-a; ok {
		t.Fatal("canal deveria fechar ao cancelar a assinatura")
	}
	cancelA() // cancelar duas vezes não pode entrar em pânico
	if h.Count() != 1 {
		t.Fatalf("assinantes = %d, quer 1", h.Count())
	}
}

func TestAssinanteLentoEhDesconectado(t *testing.T) {
	h := New()
	slow, _ := h.Subscribe(1)
	h.Broadcast(Event{Type: EventDocs})
	h.Broadcast(Event{Type: EventDocs}) // fila cheia: desconecta

	if h.Count() != 0 {
		t.Fatalf("assinante lento deveria sair; restam %d", h.Count())
	}
	n := 0
	for range slow {
		n++
	}
	if n != 1 {
		t.Fatalf("recebeu %d eventos, quer 1", n)
	}
}

func TestBroadcastConcorrenteComCancelamento(t *testing.T) {
	h := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		ch, cancel := h.Subscribe(2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range ch {
			}
		}()
		go func() {
			defer wg.Done()
			cancel()
		}()
	}
	for i := 0; i < 200; i++ {
		h.Broadcast(Event{Type: EventDiagram})
	}
	wg.Wait()
}
