package goobs

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	counterMu      sync.Mutex
	counterCache   = map[string]metric.Int64Counter{}
	histogramMu    sync.Mutex
	histogramCache = map[string]metric.Float64Histogram{}
	gaugeMu        sync.Mutex
	gaugeCache     = map[string]metric.Float64Gauge{}
)

type CounterObs struct {
	name  string
	attrs []attribute.KeyValue
	unit  string
	desc  string
}

func MetricCounter(name string) *CounterObs {
	return &CounterObs{
		name: name,
		unit: "1",
	}
}

func (b *CounterObs) Attr(key string, val any) *CounterObs {
	b.attrs = append(b.attrs, anyToAttr(key, val))
	return b
}

func (b *CounterObs) Attrs(attrs ...attribute.KeyValue) *CounterObs {
	b.attrs = append(b.attrs, attrs...)
	return b
}

func (b *CounterObs) Unit(unit string) *CounterObs {
	if unit != "" {
		b.unit = unit
	}
	return b
}

func (b *CounterObs) Description(desc string) *CounterObs {
	b.desc = desc
	return b
}

func (b *CounterObs) Add(ctx context.Context, value int64) {
	cfg, _, _, _, m := getGlobals()
	if !cfg.EnableMetrics || m == nil {
		return
	}

	counter := getOrCreateCounter(m, b.name, b.unit, b.desc)
	if counter == nil {
		return
	}

	counter.Add(ctx, value, metric.WithAttributes(b.attrs...))
}

func getOrCreateCounter(m metric.Meter, name, unit, desc string) metric.Int64Counter {
	counterMu.Lock()
	defer counterMu.Unlock()

	key := name + "|" + unit + "|" + desc
	if c, ok := counterCache[key]; ok {
		return c
	}

	c, err := m.Int64Counter(
		name,
		metric.WithUnit(unit),
		metric.WithDescription(desc),
	)
	if err != nil {
		return nil
	}
	counterCache[key] = c
	return c
}

type HistogramObs struct {
	name  string
	attrs []attribute.KeyValue
	unit  string
	desc  string
}

func MetricHistogram(name string) *HistogramObs {
	return &HistogramObs{
		name: name,
		unit: "ms",
	}
}

func (b *HistogramObs) Attr(key string, val any) *HistogramObs {
	b.attrs = append(b.attrs, anyToAttr(key, val))
	return b
}

func (b *HistogramObs) Attrs(attrs ...attribute.KeyValue) *HistogramObs {
	b.attrs = append(b.attrs, attrs...)
	return b
}

func (b *HistogramObs) Unit(unit string) *HistogramObs {
	if unit != "" {
		b.unit = unit
	}
	return b
}

func (b *HistogramObs) Description(desc string) *HistogramObs {
	b.desc = desc
	return b
}

func (b *HistogramObs) Record(ctx context.Context, value float64) {
	cfg, _, _, _, m := getGlobals()
	if !cfg.EnableMetrics || m == nil {
		return
	}

	h := getOrCreateHistogram(m, b.name, b.unit, b.desc)
	if h == nil {
		return
	}

	h.Record(ctx, value, metric.WithAttributes(b.attrs...))
}

func getOrCreateHistogram(m metric.Meter, name, unit, desc string) metric.Float64Histogram {
	histogramMu.Lock()
	defer histogramMu.Unlock()

	key := name + "|" + unit + "|" + desc
	if h, ok := histogramCache[key]; ok {
		return h
	}

	h, err := m.Float64Histogram(
		name,
		metric.WithUnit(unit),
		metric.WithDescription(desc),
	)
	if err != nil {
		return nil
	}
	histogramCache[key] = h
	return h
}

type GaugeObs struct {
	name  string
	attrs []attribute.KeyValue
	unit  string
	desc  string
}

func MetricGauge(name string) *GaugeObs {
	return &GaugeObs{
		name: name,
		unit: "1",
	}
}

func (b *GaugeObs) Attr(key string, val any) *GaugeObs {
	b.attrs = append(b.attrs, anyToAttr(key, val))
	return b
}

func (b *GaugeObs) Attrs(attrs ...attribute.KeyValue) *GaugeObs {
	b.attrs = append(b.attrs, attrs...)
	return b
}

func (b *GaugeObs) Unit(unit string) *GaugeObs {
	if unit != "" {
		b.unit = unit
	}
	return b
}

func (b *GaugeObs) Description(desc string) *GaugeObs {
	b.desc = desc
	return b
}

func (b *GaugeObs) Record(ctx context.Context, value float64) {
	cfg, _, _, _, m := getGlobals()
	if !cfg.EnableMetrics || m == nil {
		return
	}

	g := getOrCreateGauge(m, b.name, b.unit, b.desc)
	if g == nil {
		return
	}

	g.Record(ctx, value, metric.WithAttributes(b.attrs...))
}

func getOrCreateGauge(m metric.Meter, name, unit, desc string) metric.Float64Gauge {
	gaugeMu.Lock()
	defer gaugeMu.Unlock()

	key := name + "|" + unit + "|" + desc
	if g, ok := gaugeCache[key]; ok {
		return g
	}

	g, err := m.Float64Gauge(
		name,
		metric.WithUnit(unit),
		metric.WithDescription(desc),
	)
	if err != nil {
		return nil
	}
	gaugeCache[key] = g
	return g
}

func anyToAttr(key string, val any) attribute.KeyValue {
	switch v := val.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}
