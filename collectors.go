package main

import (
	"net/http"
	"sync"
	"time"
)

type ExchangeCollector struct {
	exchanges []Exchange
	period    int64
}

func NewExchangeCollector(exchangeLasts map[string]int64, period int64) (*ExchangeCollector, error) {
	exchanges := make([]Exchange, 0, len(exchangeLasts))

	for exchange, last := range exchangeLasts {
		if contructor, ok := ExchangeConstructors[exchange]; ok {
			ex, err := contructor(&http.Client{Timeout: 300 * time.Second}, last, period) // Consider if sharing a single client is better
			if err != nil {
				return nil, err
			}
			exchanges = append(exchanges, ex)
		}
	}

	return &ExchangeCollector{
		exchanges: exchanges,
		period:    period,
	}, nil
}

func (ec *ExchangeCollector) HistoricSync(data chan []DataTick) {
	now := time.Now().Unix()
	wg := new(sync.WaitGroup)
	for _, ex := range ec.exchanges {
		l := ex.LastUpdateTime()
		if now-l <= ec.period {
			continue
		}
		wg.Add(1)
		go func(ex Exchange, wg *sync.WaitGroup) {
			err := ex.Historic(data)
			if err != nil {
				excLog.Error(err)
			} else {
				excLog.Infof("Completed historic sync for %s", ex.Name())
			}
			wg.Done()
		}(ex, wg)
	}

	wg.Wait()
}

func (ec *ExchangeCollector) Collect(data chan []DataTick, wg *sync.WaitGroup, quit chan struct{}) {
	ticker := time.NewTicker(time.Duration(ec.period) * time.Second)
	for {
		select {
		case <-ticker.C:
			excLog.Trace("Triggering exchange collectors")
			for _, ex := range ec.exchanges {
				go ex.Collect(data)
			}
		case <-quit:
			excLog.Infof("Stopping collector")
			wg.Done()
			return
		}

	}
}