# گزارش نهایی پیاده‌سازی و اعتبارسنجی QA-004 / QA-005 / QA-007

**تاریخ:** 2026-11-18  
**مخزن:** `github.com/majiddarvishan/neta-go`  
**شاخه مستقل:** `feature/qa004-qa005-qa007-implementation-2026-11-18`  
**مبنای main (بدون تغییر):** `24b7c5f6eb4e001f30dd8a564a337ca1f506eac8` — PR #22  
**HEAD کد اعتبارسنجی‌شده:** `eaf022357e369b8d484376a3385b16f2ce4fafd6`  
**تعداد Commit تا HEAD کد اعتبارسنجی‌شده، نسبت به main:** 27  
**CI معتبر برای همین کد:** https://github.com/majiddarvishan/neta-go/actions/runs/44310174076

## ۱. وضعیت دقیق

| بخش | وضعیت | دلیل |
|---|---|---|
| QA-007 — Callback Panic و قراردادهای Completion/OnClose | **[x] تکمیل و اعتبارسنجی‌شده روی Feature Branch** | `gofmt`، `go vet`، Go unit، Go Race، Lifecycle/Accounting، تست‌های Panic/BindValidator/Handshake، C++ golden و smoke همگی PASS |
| QA-004 — Linear Scan / Deadline | **[ ] Qualification باز** | Scan و Detection اجرا شدند، با ۱۲۰۰ نمونه برای هر جمعیت؛ محیط GitHub-hosted فقط advisory است، ۱۰ جفت مرجع و حد مطلق P99 هنوز تصویب/اجرا نشده |
| QA-005 — Performance Qualification | **[ ] Qualification باز** | ماتریس ۸ Core/۲۴ Full، گزارش Core/Full و Comparator با ۱۰ جفت/۹۵٪ آماده و روی hosted runner اجرا/تست شده‌اند؛ **۱۰ جفت مرجع و Soak ۳۰ دقیقه‌ای NOT RUN** |

## ۲. تغییرات پیاده‌سازی‌شده

**QA-007:** Recover همهٔ Callbackهای کاربر شامل OnRequest/OnBound/Completion/BindValidator/OnClose؛ ErrCallbackPanic؛ Session.Diagnostics و ساختارهای Typed و Snapshot مستقل؛ حفظ First، Recent Ring، OnClose Panic و Dropped؛ Byte Budget ۶۴ KiB..۲ MiB (پیش‌فرض ۲۵۶ KiB) و Ring ۱..۳۲ (پیش‌فرض ۸)، تخصیص Lazy؛ safe ValueText حداکثر ۱ KiB، Stack کامل؛ Late Panic بعد از Done؛ Atomic Start Claim برای Application؛ لغو Application شروع‌نشده؛ حفظ Completion تمام Requestهای پذیرفته‌شده در Forced Close؛ رزرو Owner OnClose Credit پیش از پذیرش Session و آزادسازی پس از Callback. Worker/Queue نامحدود اضافه نشده است.

**QA-004:** Linear Scan و Timer Manager دست‌نخورده ماندند؛ سنجش اندازهٔ ۳۲/۱۲۸/۵۱۲/۲۰۴۸/۸۱۹۲ Pending؛ ۲۰۴۸ و ۸۱۹۲ صرفاً Fixture مصنوعی‌اند؛ سنجش P50/P95/P99 از Terminal Timeout Settlement، مستقل از زمان شروع Completion؛ CPU profile و Allocation ثبت شدند.

**QA-005:** ۸ Core + ۲۴ Full منتخب؛ ۱۰ جفت اجرای متناوب؛ Comparator با فاصلهٔ اطمینان ۹۵٪ (Bonferroni برای مقایسه‌های متعدد) و PASS/FAIL/INCONCLUSIVE؛ عدم PASS برای دادهٔ ناقص، Zero Baseline یا عدم تأیید محیط/نسخهٔ Baseline؛ امکان Soak ۱۸۰۰ ثانیه‌ای؛ CI فقط Report-Only؛ Versioning و SHA-256 برای Baseline با اجازهٔ صریح مالک.

## ۳. شواهد CI نهایی

Workflow Run [44310174076](https://github.com/majiddarvishan/neta-go/actions/runs/44310174076)، SHA `eaf022357e369b8d484376a3385b16f2ce4fafd6`:

- Go: **PASS** — `make verify` شامل gofmt، go vet، `go test -count=1 ./...` و `go test -race -count=1 ./...`.
- Smoke و مثال‌ها: **PASS** — دو مثال، Request/Response، SendNoWait و Graceful Shutdown.
- Pinned C++ V1 Golden و `go test ./internal/wire`: **PASS** (این شواهد جایگزین تست TCP واقعی C++ در محیط خصوصی نیستند).
- QA-004 Scan و Detection با حداقل ۱۲۰۰ نمونه، QA-005 Core و Full (مجموعاً ۲۴ سناریو) advisory و ۱۲ Unit Test مقایسه‌گر پایتون: **PASS** به‌عنوان *اجرای ابزار*، نه عبور از Gate عملکرد.
- Coverage بستهٔ اصلی: **80.2%**، Wire: **91.9%**.

### نمونهٔ نتایج QA-004 روی GitHub-hosted runner

| Pending مصنوعی | Scan در ۱ CPU (ns/op) | Detection P50 (µs) | P95 (µs) | P99 (µs) | نمونه‌های Detection |
|---:|---:|---:|---:|---:|---:|
| 32 | 379.6 | 131.406 | 154.762 | 192.557 | 1,200 |
| 128 | 1,403 | 141.516 | 154.065 | 178.933 | 1,200 |
| 512 | 5,549 | 141.234 | 162.957 | 188.518 | 1,200 |
| 2,048 | 23,687 | 205.053 | 229.740 | 253.144 | 1,200 |
| 8,192 | 105,212 | 412.508 | 479.287 | 808.932 | 1,200 |

در Benchmark خالص Scan، **۰ allocs/op** ثبت شد (برای جمعیت ۸۱۹۲، ۳ B/op تحت CPU profiling)؛ Profile CPU برای Scan و Detection در Artifact موجود است. افزایش حجم نمونه به ۱۲۰۰ کیفیت مشاهدهٔ P99 را بهتر می‌کند ولی جایگزین مقایسهٔ آماری ۱۰ جفت روی سخت‌افزار مرجع نیست. هیچ سقف مطلق P99 یا Baseline مرجع مورد تأیید مالک وجود ندارد.

QA-005 هر ۸ سناریوی Core و کل ۲۴ سناریوی Full (شامل همان ۸ Core) اجرا شد؛ نتایج Throughput/P50/P95/P99، تخصیص و **Linux CPU-ns/op** در Artifact موجود است. معیار حافظهٔ منسوب به خود کتابخانه هنوز مشاهدهٔ معتبر ندارد و به‌صورت INCONCLUSIVE حفظ می‌شود. **۱۰ جفت مرجع و آزمون ۳۰دقیقه‌ای NOT RUN**؛ Performance Gate همچنان خاموش است.

## ۴. موارد باز و سیاست مالک

- [ ] انتخاب و تأیید محیط مرجع پایدار برای Benchmarkها؛ مالکیت Baseline تاریخی و تأیید صریح هر نسخهٔ جدید.
- [ ] ۱۰ جفت اجرای واقعی QA-004/005 با Confidence ۹۵٪؛ ۳۰ دقیقه Soak روی مرجع پایدار؛ معیار واقعی حافظهٔ نگه‌داری‌شدهٔ قابل انتساب؛ آستانهٔ مطلق QA-004 P99 و SLAهای دیگر تنها با شواهد و تصمیم مالک.
- [ ] Performance Gate همچنان **DISABLED** است. آستانه‌های نسبی ۱۰٪ و ۱۵٪ صرفاً برای تحلیل گزارش‌اند.
- [ ] تصمیم جداگانهٔ مالک برای PR، Merge، Tag و Release. هیچ‌کدام ایجاد نشده‌اند.

**نکته:** دو اجرای اولیهٔ CI به‌ترتیب به‌دلیل gofmt و ناسازگاری تست‌های قدیمی با قراردادهای جدید شکست خوردند و با Commitهای اصلاحی برطرف شدند. فقط Run نهایی فوق معتبر است. Commit نهاییِ مستندسازی، در صورت ثبت با `[skip ci]`، تغییر کد ندارد و از CI تکراری جلوگیری می‌کند.
