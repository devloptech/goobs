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

---

## API Reference

### Init

| Function | Signature | คำอธิบาย |
|---|---|---|
| `Init` | `Init(ctx context.Context, cfg Config) (func(context.Context) error, error)` | ตั้งค่า provider ทั้งหมด (Trace, Metric, Log) คืน shutdown function กลับมา เรียกซ้ำได้ — จะ shutdown provider เดิมก่อนสร้างใหม่ |

---

### Log() → `*LogObs`

สร้าง structured log ที่เขียนไปทั้ง OTel Log Exporter และ Zap พร้อมกัน แนบ `trace_id`/`span_id` อัตโนมัติจาก context

| Method | Signature | คำอธิบาย |
|---|---|---|
| `Log` | `Log() *LogObs` | สร้าง builder ใหม่ (default: Info level) |
| `FromContext` | `.FromContext(ctx context.Context) *LogObs` | แนบ context เพื่อดึง trace/span ID อัตโนมัติ |
| `Debug` | `.Debug() *LogObs` | ตั้ง level เป็น DEBUG |
| `Info` | `.Info() *LogObs` | ตั้ง level เป็น INFO (default) |
| `Warn` | `.Warn() *LogObs` | ตั้ง level เป็น WARN |
| `Error` | `.Error() *LogObs` | ตั้ง level เป็น ERROR |
| `Msg` | `.Msg(msg string) *LogObs` | ตั้งข้อความ log |
| `Field` | `.Field(key string, val any) *LogObs` | เพิ่ม field เดียว (รองรับ `string`, `int`, `int64`, `float64`, `bool`, `any`) |
| `Fields` | `.Fields(fields ...zap.Field) *LogObs` | เพิ่มหลาย field แบบ `zap.Field` |
| `Err` | `.Err(err error) *LogObs` | เพิ่ม error field (ถ้า `err != nil`) |
| **`Send`** | **`.Send()`** | **Terminal** — ส่ง log ออกไปทั้ง OTel และ Zap |

```go
goobs.Log().
    FromContext(ctx).
    Warn().
    Msg("high latency").
    Field("latency_ms", 1500).
    Err(err).
    Send()
```

---

### Trace() → `*TraceObs`

สร้าง distributed tracing span มี 3 โหมด terminal

| Method | Signature | คำอธิบาย |
|---|---|---|
| `Trace` | `Trace() *TraceObs` | สร้าง builder ใหม่ (default: `SpanKindInternal`, tracer `"goobs"`) |
| `Name` | `.Name(name string) *TraceObs` | ตั้งชื่อ span |
| `FromContext` | `.FromContext(ctx context.Context) *TraceObs` | แนบ context (เชื่อมกับ parent span) |
| `Kind` | `.Kind(kind trace.SpanKind) *TraceObs` | ตั้ง span kind เช่น `SpanKindServer`, `SpanKindClient`, `SpanKindConsumer` |
| `TracerName` | `.TracerName(name string) *TraceObs` | ตั้งชื่อ tracer (default: `"goobs"`) |
| `Attr` | `.Attr(key string, val any) *TraceObs` | เพิ่ม attribute (รองรับ `string`, `int`, `int64`, `float64`, `bool`, `any`) |
| `Attrs` | `.Attrs(attrs ...attribute.KeyValue) *TraceObs` | เพิ่มหลาย attribute แบบ OTel `attribute.KeyValue` |
| `RecordError` | `.RecordError(enable bool) *TraceObs` | เปิด/ปิดบันทึก error บน span (default: `true`) |
| `SetStatusOnError` | `.SetStatusOnError(enable bool) *TraceObs` | เปิด/ปิดตั้ง status เป็น Error เมื่อ error (default: `true`) |
| **`Start`** | **`.Start() (context.Context, trace.Span)`** | **Terminal** — คืน ctx + span ต้องเรียก `span.End()` เอง |
| **`StartScope`** | **`.StartScope() *SpanScope`** | **Terminal** — คืน `SpanScope` เรียก `.Done()` เพื่อจบ span |
| **`Run`** | **`.Run(fn func(ctx context.Context) error) error`** | **Terminal** — รัน function ภายใน span จบ span และบันทึก error อัตโนมัติ |

#### SpanScope

| Method | Signature | คำอธิบาย |
|---|---|---|
| `Ctx` | `.Ctx() context.Context` | คืน context ที่มี span อยู่ |
| `Span` | `.Span() trace.Span` | คืน `trace.Span` สำหรับเพิ่ม attribute/event เอง |
| `Done` | `.Done()` | จบ span |

```go
// Run — สะดวกที่สุด
err := goobs.Trace().Name("send.email").FromContext(ctx).
    Run(func(ctx context.Context) error {
        return emailService.Send(ctx, to, body)
    })

// StartScope — ควบคุมปานกลาง
scope := goobs.Trace().Name("process").FromContext(ctx).StartScope()
defer scope.Done()
doWork(scope.Ctx())

// Start — ควบคุมเต็มที่
ctx, span := goobs.Trace().Name("db.query").FromContext(ctx).Start()
defer span.End()
```

---

### MetricCounter() → `*CounterObs`

Counter สำหรับนับจำนวน (เพิ่มขึ้นอย่างเดียว) No-op เมื่อ `EnableMetrics: false`

| Method | Signature | คำอธิบาย |
|---|---|---|
| `MetricCounter` | `MetricCounter(name string) *CounterObs` | สร้าง builder (default unit: `"1"`) |
| `Attr` | `.Attr(key string, val any) *CounterObs` | เพิ่ม attribute |
| `Attrs` | `.Attrs(attrs ...attribute.KeyValue) *CounterObs` | เพิ่มหลาย attribute |
| `Unit` | `.Unit(unit string) *CounterObs` | ตั้งหน่วย เช่น `"1"`, `"requests"` |
| `Description` | `.Description(desc string) *CounterObs` | ตั้งคำอธิบาย metric |
| **`Add`** | **`.Add(ctx context.Context, value int64)`** | **Terminal** — เพิ่มค่า counter |

```go
goobs.MetricCounter("http_requests_total").
    Attr("method", "GET").
    Attr("status", 200).
    Add(ctx, 1)
```

---

### MetricHistogram() → `*HistogramObs`

Histogram สำหรับวัดการกระจายของค่า (เช่น latency, request size) No-op เมื่อ `EnableMetrics: false`

| Method | Signature | คำอธิบาย |
|---|---|---|
| `MetricHistogram` | `MetricHistogram(name string) *HistogramObs` | สร้าง builder (default unit: `"ms"`) |
| `Attr` | `.Attr(key string, val any) *HistogramObs` | เพิ่ม attribute |
| `Attrs` | `.Attrs(attrs ...attribute.KeyValue) *HistogramObs` | เพิ่มหลาย attribute |
| `Unit` | `.Unit(unit string) *HistogramObs` | ตั้งหน่วย เช่น `"ms"`, `"bytes"` |
| `Description` | `.Description(desc string) *HistogramObs` | ตั้งคำอธิบาย metric |
| **`Record`** | **`.Record(ctx context.Context, value float64)`** | **Terminal** — บันทึกค่า |

```go
goobs.MetricHistogram("http_request_duration_ms").
    Attr("method", "POST").
    Record(ctx, 42.5)
```

---

### MetricGauge() → `*GaugeObs`

Gauge สำหรับวัดค่าปัจจุบันที่ขึ้นหรือลงได้ (เช่น active connections, queue depth) No-op เมื่อ `EnableMetrics: false`

| Method | Signature | คำอธิบาย |
|---|---|---|
| `MetricGauge` | `MetricGauge(name string) *GaugeObs` | สร้าง builder (default unit: `"1"`) |
| `Attr` | `.Attr(key string, val any) *GaugeObs` | เพิ่ม attribute |
| `Attrs` | `.Attrs(attrs ...attribute.KeyValue) *GaugeObs` | เพิ่มหลาย attribute |
| `Unit` | `.Unit(unit string) *GaugeObs` | ตั้งหน่วย |
| `Description` | `.Description(desc string) *GaugeObs` | ตั้งคำอธิบาย metric |
| **`Record`** | **`.Record(ctx context.Context, value float64)`** | **Terminal** — บันทึกค่าปัจจุบัน |

```go
goobs.MetricGauge("active_connections").
    Attr("service", "api").
    Record(ctx, float64(pool.ActiveCount()))
```

---

### Propagate() → `*PropagationObs`

ส่งต่อ trace context ข้าม service ผ่าน HTTP, gRPC, และ AMQP

| Method | Signature | คำอธิบาย |
|---|---|---|
| `Propagate` | `Propagate() *PropagationObs` | สร้าง builder |
| `FromContext` | `.FromContext(ctx context.Context) *PropagationObs` | แนบ context ที่มี span |
| `WithLegacyHeaders` | `.WithLegacyHeaders(enable bool) *PropagationObs` | เปิด/ปิด legacy headers (`x-trace-id`, `x-span-id`) |

#### HTTP

| Method | Signature | คำอธิบาย |
|---|---|---|
| **`FromHTTPRequest`** | **`.FromHTTPRequest(r *http.Request) context.Context`** | **Inbound** — ดึง trace context จาก HTTP request headers คืน context ใหม่ |
| **`ToHTTPRequest`** | **`.ToHTTPRequest(r *http.Request)`** | **Outbound** — ฉีด trace context เข้า HTTP request headers |
| **`ToHTTPResponse`** | **`.ToHTTPResponse(w http.ResponseWriter)`** | **Response** — เพิ่ม `x-trace-id`/`x-span-id` ลง response headers |

#### gRPC

| Method | Signature | คำอธิบาย |
|---|---|---|
| **`FromGRPCMetadata`** | **`.FromGRPCMetadata(ctx context.Context, md metadata.MD) context.Context`** | **Inbound** — ดึง trace context จาก gRPC metadata |
| **`ToGRPCMetadata`** | **`.ToGRPCMetadata(md *metadata.MD)`** | **Outbound** — ฉีด trace context เข้า gRPC metadata |

#### AMQP (RabbitMQ)

| Method | Signature | คำอธิบาย |
|---|---|---|
| **`FromAMQP`** | **`.FromAMQP(headers amqp.Table) context.Context`** | **Inbound** — ดึง trace context จาก AMQP message headers |
| **`ToAMQP`** | **`.ToAMQP(headers amqp.Table)`** | **Outbound** — ฉีด trace context เข้า AMQP message headers |

---

### AMQPConsumerInterceptor

| Function | Signature | คำอธิบาย |
|---|---|---|
| `AMQPConsumerInterceptor` | `AMQPConsumerInterceptor(serviceName string, handler AMQPConsumeHandler) func(msg amqp.Delivery)` | สร้าง interceptor สำเร็จรูปที่ครอบ AMQP consumer handler |

โดย `AMQPConsumeHandler` คือ `func(ctx context.Context, msg amqp.Delivery) error`

**สิ่งที่ interceptor ทำอัตโนมัติ:**

| สิ่งที่ทำ | รายละเอียด |
|---|---|
| Context propagation | ดึง trace context จาก AMQP message headers |
| Tracing | สร้าง span `"amqp.consume"` (`SpanKindConsumer`) พร้อม attributes `amqp.queue`, `amqp.exchange` |
| Error recording | บันทึก error บน span ถ้า handler return error |
| Counter metric | `amqp_consume_total` แยกตาม `service`, `queue`, `status` |
| Histogram metric | `amqp_consume_duration_ms` แยกตาม `service`, `queue`, `status` |

```go
wrapped := goobs.AMQPConsumerInterceptor("order-service",
    func(ctx context.Context, msg amqp.Delivery) error {
        return processMessage(ctx, msg)
    },
)

msgs, _ := ch.Consume("orders", "", true, false, false, false, nil)
for msg := range msgs {
    wrapped(msg)
}
```

## สัญญาอนุญาต

MIT
