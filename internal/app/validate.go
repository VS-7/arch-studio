package app

import (
	"github.com/archcode/studio/internal/lint"
)

// ---------------------------------------------------------------------------
// Validação arquitetural
// ---------------------------------------------------------------------------

func (a *App) Validate() (*lint.Report, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	return lint.Run(lint.Input{
		Diagram:      snap.Diagram,
		Requirements: snap.Requirements,
		UseCases:     snap.UseCases,
		Endpoints:    snap.Endpoints,
	}), nil
}
