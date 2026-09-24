package app

import (
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Precificação
// ---------------------------------------------------------------------------

func (a *App) Estimate(marginOverride float64) (*pricing.Estimate, error) {
	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	ucs, err := a.st.ListUseCases()
	if err != nil {
		return nil, err
	}
	cfg, err := a.st.LoadPricing()
	if err != nil {
		return nil, err
	}
	return pricing.Calculate(d, ucs, cfg, marginOverride), nil
}

func (a *App) SavePricingConfig(cfg *model.PricingConfig, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	if err := a.st.SavePricing(cfg); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventPricing, Source: source, Path: store.FilePricing})
	return nil
}
