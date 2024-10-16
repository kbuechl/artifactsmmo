package world

import (
	"artifactsmmo/internal/models"
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
)

type Collector struct {
	Resources   ResourceMap
	tiles       []models.MapTile
	Monsters    []models.Monster
	Items       []client.ItemSchema
	BankItems   []client.SimpleItemSchema
	bankDetails client.BankSchema
	mu          sync.RWMutex
	ctx         context.Context
	client      *client.ClientWithResponses
	Out         chan error
	logger      *slog.Logger
	BankChannel chan models.BankResponse
}

func NewCollector(ctx context.Context, c *client.ClientWithResponses) (*Collector, error) {
	collector := &Collector{
		ctx:         ctx,
		client:      c,
		Out:         make(chan error),
		BankChannel: make(chan models.BankResponse),
		logger:      slog.Default().With("source", "collector"),
	}
	collector.logger.Info("Loading World")
	rData, err := collector.getAllResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all resources: %w", err)
	}
	collector.Resources = rData

	if err = collector.loadMapTiles(); err != nil {
		return nil, fmt.Errorf("update world data: %w", err)
	}

	if err = collector.LoadBankItems(); err != nil {
		return nil, fmt.Errorf("load bank items: %w", err)
	}

	if err = collector.LoadBankDetails(); err != nil {
		return nil, fmt.Errorf("load bank gold: %w", err)
	}

	if err = collector.loadMonsters(); err != nil {
		return nil, fmt.Errorf("load monsters: %w", err)
	}

	if err = collector.loadItems(ctx); err != nil {
		return nil, fmt.Errorf("loading items: %w", err)
	}

	collector.start()

	return collector, nil
}

func (w *Collector) start() {
	go func() {
		for {
			select {
			case <-w.ctx.Done():
				return
			case data := <-w.BankChannel:
				if data.Gold != nil {
					w.UpdateBankGold(*data.Gold)
				}
				if data.Items != nil {
					w.UpdateBankItems(*data.Items)
				}
			}
		}
	}()
}

func (w *Collector) LoadBankItems() error {
	w.logger.Info("Loading Bank Items")
	data := make([]client.SimpleItemSchema, 0)
	size := 100
	for page := 1; ; page++ {
		resp, err := w.client.GetBankItemsMyBankItemsGetWithResponse(w.ctx, &client.GetBankItemsMyBankItemsGetParams{
			ItemCode: nil,
			Page:     &page,
			Size:     &size,
		})
		if err != nil {
			return fmt.Errorf("get all bank items: %w", err)
		}
		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("get all bank items: %d", resp.StatusCode())
		}

		data = append(data, resp.JSON200.Data...)
		if p, err := resp.JSON200.Page.AsDataPageSimpleItemSchemaPage0(); err != nil {
			return fmt.Errorf("get all bank items: %w", err)
		} else if page >= p {
			break
		}
	}

	w.UpdateBankItems(data)

	return nil
}

func (w *Collector) LoadBankDetails() error {
	w.logger.Info("Loading Bank Details")
	resp, err := w.client.GetBankDetailsMyBankGetWithResponse(w.ctx)
	if err != nil {
		return fmt.Errorf("get all bank items: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("get all bank items: %d", resp.StatusCode())
	}

	w.UpdateBankDetails(resp.JSON200.Data)

	return nil
}

func (w *Collector) UpdateBankGold(q int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bankDetails.Gold = q
}

func (w *Collector) UpdateBankItems(schema []client.SimpleItemSchema) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.BankItems = schema
}

func (w *Collector) UpdateBankDetails(details client.BankSchema) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bankDetails = details
}

func (w *Collector) GetResourceByName(name string) *Resource {
	if r, ok := w.Resources[name]; ok {
		return &r
	}
	return nil
}

// GetResourcesBySkill filters out resources based on the skill and the current skill level, ordered by level desc
func (w *Collector) GetResourcesBySkill(skill string, level int) []Resource {
	data := make([]Resource, 0)

	for _, r := range w.Resources {
		if r.Skill == skill && r.Level <= level {
			data = append(data, r)
		}
	}

	slices.SortFunc(data, func(a, b Resource) int {
		return b.Level - a.Level
	})

	return data
}

func (w *Collector) GetMapByContentType(contentType mapContentType) []*models.MapTile {
	cString := contentType.String()

	res := make([]*models.MapTile, 0)
	for _, m := range w.MapTiles() {
		if m.Type == cString {
			res = append(res, &m)
		}
	}
	return res
}
