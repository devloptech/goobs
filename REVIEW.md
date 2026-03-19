# REVIEW.md

ผลการ review โค้ดของไลบรารี observability `goobs`

---

## แก้ไขแล้ว

### ~~1. `logCaller()` ถูกเรียก 2 ครั้งต่อ log entry~~ (`logger.go`)

**แก้แล้ว:** เรียก `logCaller()` ครั้งเดียวที่ด้านบนของ `Send()` แล้วนำผลไปใช้ซ้ำทั้ง OTel path และ Zap path

### ~~2. Shutdown function กลืน error ทั้งหมด~~ (`goobs.go`)

**แก้แล้ว:** แยก `shutdownProviders()` ออกมาใช้ `errors.Join` รวม error จากทุก provider พร้อม cleanup globals เป็น nil

### ~~3. `grpc.WithBlock()` บล็อกไม่มีกำหนด~~ (`goobs.go`)

**แก้แล้ว:** ลบ `grpc.WithBlock()` ออกจากทุก exporter (trace, metric, log) OTel SDK จะ retry เชื่อมต่อเองเบื้องหลัง

### ~~4. ไม่มี concurrency protection บน global state~~ (`goobs.go`)

**แก้แล้ว:** เพิ่ม `sync.RWMutex` (`initMu`) ป้องกัน globals ทั้งหมด `Init()` และ `shutdown` ใช้ write lock, ฝั่ง read ใช้ `getGlobals()` ที่ถือ read lock

### ~~5. `Init()` ไม่เป็น idempotent — เรียกซ้ำจะ leak provider~~ (`goobs.go`)

**แก้แล้ว:** เพิ่ม flag `initialized` เมื่อเรียก `Init()` ซ้ำจะ shutdown provider เดิมก่อน re-initialize

### ~~6. field `err` ใน `PropagationObs` ไม่ถูกใช้งาน~~ (`http.go`)

**แก้แล้ว:** ลบ field `err interface{}` ออก

### ~~7. ชื่อ meter ถูก hardcode เป็น `"eto"`~~ (`goobs.go`)

**แก้แล้ว:** เปลี่ยนเป็นใช้ `cfg.ServiceName` สำหรับทั้ง meter และ OTel logger

### ~~8. Metric instrument cache ไม่สนใจการเปลี่ยนแปลง `unit` และ `desc`~~ (`metric.go`)

**แก้แล้ว:** cache key เปลี่ยนจาก `name` อย่างเดียว เป็น `name|unit|desc` ทั้ง counter, histogram และ gauge

### ~~9. Error message อ้างชื่อ package ผิด~~ (`trace.go`)

**แก้แล้ว:** เปลี่ยนจาก `"eto.Trace().Run: fn is nil"` เป็น `"goobs.Trace().Run: fn is nil"`

### ~~10. ไม่มีเมธอด `.Err(error)` สำหรับ structured error logging~~ (`logger.go`)

**แก้แล้ว:** เพิ่มเมธอด `Err(err error) *LogObs` ที่ใช้ `zap.Error(err)` ภายใน

### ~~11. ไม่มี metric ประเภท Gauge~~ (`metric.go`)

**แก้แล้ว:** เพิ่ม `MetricGauge()` → `GaugeObs` builder พร้อม `Record(ctx, value)` ใช้ pattern เดียวกับ counter/histogram

### ~~12. `FromHTTPRequest` ไม่ใช้ context ของ builder~~ (`http.go`)

**แก้แล้ว:** `FromHTTPRequest` ใช้ `p.ctx` เป็น base context ถ้ามีค่า fallback ไปที่ `r.Context()` สอดคล้องกับ `FromAMQP`

### ~~14. `zapFieldsToOtelAttrs` ทิ้ง field โดยไม่แจ้งเตือน~~ (`logger.go`)

**แก้แล้ว:** เพิ่ม case สำหรับ `zapcore.ErrorType` และ default case ที่ใช้ `f.Interface` เป็น fallback เมื่อ `f.String` ว่าง

### เพิ่มเติม: Propagation ใช้ global โดยตรงโดยไม่ผ่าน lock

**แก้แล้ว:** ทุกเมธอดใน `http.go` และ `amqp.go` เปลี่ยนจากอ่าน `globalPropagator` โดยตรง เป็นใช้ `getGlobals()` ที่มี read lock ป้องกัน data race

---

### ~~13. ไม่มี test~~

**แก้แล้ว:** เพิ่ม test 70 cases ครอบคลุม 83.3% ของ statements ประกอบด้วย:

- `goobs_test.go` — Init, shutdown, globals, idempotency (4 tests)
- `logger_test.go` — Builder chain, field types, Err(), Send(), caller resolution, severity (16 tests)
- `trace_test.go` — Builder chain, Start/StartScope/Run, error recording, span attrs (16 tests)
- `metric_test.go` — Counter/Histogram/Gauge, caching, no-op, anyToAttr (17 tests)
- `http_test.go` — HTTP/gRPC propagation, carrier impl, legacy headers, no-init safety (17 tests)
- `amqp_test.go` — AMQP round-trip, interceptor span/metrics, context propagation (6 tests)
- `testing_helper_test.go` — Test infrastructure: in-memory OTel exporters, `setupTestEnv()`

---

## สรุป

| ระดับความรุนแรง | ทั้งหมด | แก้แล้ว | ค้างอยู่ |
|---|---|---|---|
| วิกฤต (Critical) | 3 | 3 | 0 |
| คำเตือน (Warning) | 6 | 6 | 0 |
| ข้อเสนอแนะ (Suggestion) | 5 | 5 | 0 |

**Test Coverage: 83.3%** (70 tests, ทุก test PASS)
