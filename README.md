# GOOBS

**Go Observability SDK** — ไลบรารี observability สำหรับ Go ที่รวม OpenTelemetry และ Zap ไว้ในที่เดียว

`goobs` ครอบ tracing, structured logging, metrics และ context propagation ไว้เบื้องหลัง fluent builder API เพื่อให้เซอร์วิสของคุณไม่ต้องใช้ OTel SDK โดยตรง

## การติดตั้ง

```bash
go get github.com/devloptech/goobs
```

ต้องใช้ **Go 1.25.5+**

## เริ่มต้นใช้งาน

### เริ่มต้นระบบ (Initialize)

เรียก `Init()` ครั้งเดียวตอนแอปพลิเคชันเริ่มทำงาน จะได้ shutdown function กลับมาสำหรับปิดระบบอย่างสมบูรณ์

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"

    "github.com/devloptech/goobs"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    shutdown, err := goobs.Init(ctx, goobs.Config{
        ServiceName:  "my-service",
        Environment:  "production",
        OtelEndpoint: "otel-collector:4317",
        EnableMetrics: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)

    // ... เริ่มเซิร์ฟเวอร์ของคุณ
}
```

### Tracing

สร้าง span ได้ 3 แบบ:

```go
// 1. Run — จัดการ span lifecycle และบันทึก error ให้อัตโนมัติ
err := goobs.Trace().
    Name("process-order").
    FromContext(ctx).
    Attr("order_id", orderID).
    Run(func(ctx context.Context) error {
        // ctx มี span ใหม่แนบอยู่แล้ว
        return processOrder(ctx, orderID)
    })

// 2. StartScope — ควบคุมเองด้วย Done()
scope := goobs.Trace().
    Name("fetch-user").
    FromContext(ctx).
    StartScope()
defer scope.Done()
// ใช้ scope.Ctx() สำหรับเรียก downstream

// 3. Start — ได้ ctx + span ดิบ (ยืดหยุ่นที่สุด)
ctx, span := goobs.Trace().
    Name("db-query").
    FromContext(ctx).
    Kind(trace.SpanKindClient).
    Start()
defer span.End()
```

### Logging

เขียน log พร้อมกันทั้ง OpenTelemetry log exporter และ Zap แนบ trace/span ID จาก context ให้อัตโนมัติ

```go
goobs.Log().
    FromContext(ctx).
    Info().
    Msg("order processed").
    Field("order_id", orderID).
    Field("amount", 99.90).
    Send()

goobs.Log().
    FromContext(ctx).
    Error().
    Msg("payment failed").
    Err(err).
    Send()
```

### Metrics

Counter, histogram และ gauge สร้าง instrument แบบ lazy ไม่ทำงานเมื่อ `EnableMetrics` เป็น `false`

```go
// Counter
goobs.MetricCounter("http_requests_total").
    Attr("method", "POST").
    Attr("path", "/api/orders").
    Attr("status", 200).
    Add(ctx, 1)

// Histogram
goobs.MetricHistogram("http_request_duration_ms").
    Attr("method", "POST").
    Attr("path", "/api/orders").
    Record(ctx, 42.5)

// Gauge — สำหรับค่าที่ขึ้นลงได้ (active connections, queue depth ฯลฯ)
goobs.MetricGauge("active_connections").
    Attr("service", "api").
    Record(ctx, 42)
```

### Context Propagation

inject/extract trace context ข้ามขอบเขต HTTP, gRPC และ AMQP

```go
// HTTP ขาเข้า — extract จาก request ที่เข้ามา
ctx := goobs.Propagate().FromHTTPRequest(r)

// HTTP ขาออก — inject เข้า request ที่จะส่งออก
goobs.Propagate().
    FromContext(ctx).
    WithLegacyHeaders(true). // เพิ่ม x-trace-id, x-span-id ด้วย
    ToHTTPRequest(outReq)

// HTTP response — เขียน trace ID ลง response header
goobs.Propagate().
    FromContext(ctx).
    ToHTTPResponse(w)

// gRPC — extract จาก metadata
ctx = goobs.Propagate().FromGRPCMetadata(ctx, md)

// gRPC — inject เข้า metadata
goobs.Propagate().
    FromContext(ctx).
    ToGRPCMetadata(&md)

// AMQP — extract จาก message headers
ctx = goobs.Propagate().
    FromContext(ctx).
    FromAMQP(msg.Headers)

// AMQP — inject เข้า message headers
goobs.Propagate().
    FromContext(ctx).
    ToAMQP(headers)
```

### AMQP Consumer Interceptor

interceptor สำเร็จรูปที่ครอบ AMQP consumer ด้วย tracing, metrics และ context propagation

```go
handler := goobs.AMQPConsumerInterceptor("my-service", func(ctx context.Context, msg amqp.Delivery) error {
    // ctx มี trace context ที่ extract จาก message headers มาแล้ว
    // span "amqp.consume" ทำงานอยู่พร้อม attribute ของ queue/exchange
    return processMessage(ctx, msg)
})

// ใช้ handler กับ AMQP consumer ของคุณ
// แต่ละ message จะได้: trace span, counter (amqp_consume_total), histogram (amqp_consume_duration_ms)
```

## การตั้งค่า

```go
type Config struct {
    ServiceName     string   // OTel resource attribute: service.name
    Environment     string   // OTel resource attribute: deployment.environment
    OtelEndpoint    string   // OTLP gRPC collector endpoint (เช่น "localhost:4317")
    EnableMetrics   bool     // เปิดใช้งานการเก็บ metric (counter/histogram)
    SkipCallerPkgs  []string // prefix ของ package ที่จะข้ามตอนหา caller
    SkipCallerFiles []string // substring ของชื่อไฟล์ที่จะข้ามตอนหา caller
}
```

`SkipCallerPkgs` และ `SkipCallerFiles` ควบคุมว่า frame ไหนจะถูกข้ามตอนหา caller สำหรับ log entry มีประโยชน์เมื่อคุณครอบ goobs ด้วย logging middleware ของตัวเอง และต้องการให้ caller ชี้ไปที่โค้ดแอปพลิเคชัน ไม่ใช่ wrapper

## สถาปัตยกรรม

```
┌─────────────────────────────────────────────────────────┐
│                     goobs.Init()                        │
│  ┌─────────────┐ ┌──────────────┐ ┌──────────────────┐  │
│  │ TracerProvider│ │ MeterProvider│ │ LoggerProvider   │  │
│  │  (เปิดเสมอ)  │ │(ถ้าเปิดใช้งาน)│ │  (เปิดเสมอ)     │  │
│  └──────┬───────┘ └──────┬───────┘ └───────┬──────────┘  │
│         │                │                 │             │
│         ▼                ▼                 ▼             │
│  ┌──────────────────────────────────────────────────┐   │
│  │       OTLP/gRPC Exporter (endpoint เดียว)         │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘

Fluent Builders:
  Trace()          → TraceObs     → Start() / StartScope() / Run()
  Log()            → LogObs       → Send()      [OTel + Zap เขียนพร้อมกัน]
  MetricCounter()  → CounterObs   → Add()
  MetricHistogram()→ HistogramObs → Record()
  MetricGauge()    → GaugeObs     → Record()
  Propagate()      → PropagationObs → HTTP / gRPC / AMQP inject/extract
```

## Dependencies

| Dependency | หน้าที่ |
|---|---|
| `go.opentelemetry.io/otel` | Tracing, metrics, logging, propagation |
| `go.uber.org/zap` | Structured logging (แสดงผลในเครื่อง) |
| `google.golang.org/grpc` | OTLP gRPC transport |
| `github.com/rabbitmq/amqp091-go` | AMQP carrier สำหรับ context propagation |

## สัญญาอนุญาต

MIT
