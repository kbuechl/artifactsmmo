package world

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"net/http"
	"sync"
)

type BankResponse struct {
	Gold  *int
	Items *[]client.SimpleItemSchema
}

type Collector struct {
	Resources   ResourceMap
	tiles       []MapTile
	Monsters    []Monster
	bankItems   []client.SimpleItemSchema
	bankGold    int
	mu          sync.RWMutex
	ctx         context.Context
	client      *client.ClientWithResponses
	Out         chan error
	logger      slog.Logger
	BankChannel chan BankResponse
}

func NewCollector(ctx context.Context, c *client.ClientWithResponses) (*Collector, error) {
	collector := &Collector{
		ctx:         ctx,
		client:      c,
		Out:         make(chan error),
		BankChannel: make(chan BankResponse),
	}

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

	if err = collector.LoadBankGold(); err != nil {
		return nil, fmt.Errorf("load bank gold: %w", err)
	}

	if err = collector.loadMonsters(); err != nil {
		return nil, fmt.Errorf("load monsters: %w", err)
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
	data := make([]client.SimpleItemSchema, 0)
	for page := 1; ; page++ {
		resp, err := w.client.GetBankItemsMyBankItemsGetWithResponse(w.ctx, &client.GetBankItemsMyBankItemsGetParams{
			ItemCode: nil,
			Page:     nil,
			Size:     nil,
		})
		if err != nil {
			return fmt.Errorf("get all bank items: %w", err)
		}
		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("get all bank items: %d", resp.StatusCode())
		}

		data = append(data, resp.JSON200.Data...)
		if p, err := resp.JSON200.Pages.AsDataPageSimpleItemSchemaPages0(); err != nil {
			return fmt.Errorf("get all bank items: %w", err)
		} else if p == page {
			break
		}
	}

	w.UpdateBankItems(data)

	return nil
}

func (w *Collector) LoadBankGold() error {

	resp, err := w.client.GetBankGoldsMyBankGoldGetWithResponse(w.ctx)
	if err != nil {
		return fmt.Errorf("get all bank items: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("get all bank items: %d", resp.StatusCode())
	}

	w.UpdateBankGold(resp.JSON200.Data.Quantity)

	return nil
}

func (w *Collector) UpdateBankItems(schema []client.SimpleItemSchema) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bankItems = schema
}

func (w *Collector) UpdateBankGold(q int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bankGold = q
}

func (w *Collector) GetGatherableMapTiles(playerSkills map[string]int) []MapTile {
	var mapData []MapTile
	resourceTypeString := ResourceMapContentType.String()

	for _, m := range w.MapTiles() {
		if m.Type != resourceTypeString {
			continue
		}
		resourceData, foundResource := w.Resources[m.Code]
		if !foundResource {
			continue
		}
		playerLevel, foundPlayer := playerSkills[resourceData.Skill]
		if !foundPlayer || playerLevel < resourceData.Level {
			continue
		}
		mapData = append(mapData, m)
	}

	return mapData
}

func (w *Collector) GetResourceByName(name string) (*Resource, error) {
	if r, ok := w.Resources[name]; ok {
		return &r, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

func (w *Collector) GetMapByContentType(contentType MapContentType) []*MapTile {
	cString := contentType.String()

	res := make([]*MapTile, 0)
	for _, m := range w.MapTiles() {
		if m.Type == cString {
			res = append(res, &m)
		}
	}
	return res
}
