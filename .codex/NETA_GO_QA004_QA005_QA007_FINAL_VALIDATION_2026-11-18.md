# گزارش نهایی پیاده‌سازی و اعتبارسنجی QA-004 / QA-005 / QA-007

**تاریخ:** 2026-11-18  
**مخزن:** `github.com/majiddarvishan/neta-go`  
**شاخه مستقل:** `feature/qa004-qa005-qa007-implementation-2026-11-18`  
**مبنای main (بدون تغییر):** `24b7c5f6eb4e001f30dd8a564a337ca1f506eac8` — PR #22  
**HEAD کد اعتبارسنجی‌شده:** `e852392542c20daf5a79cba2aaee7fbd55e454f8`  
**تعداد Commitهای پیاده‌سازی از main:** 17  
**CI معتبر برای همین کد:** https://github.com/majiddarvishan/neta-go/actions/runs/44306325369

## ۱. وضعیت دقیق

| بخش | وضعیت | دلیل |
|---|---|---|
| QA-007 — Callback Panic و قراردادهای Completion/OnClose | **[x] تکمیل و اعتبارسنجی‌شده روی Feature Branch** | `gofmt`، `go vet`، Go unit، Go Race، Lifecycle/Accounting، تست‌های Panic/BindValidator/Handshake، C++ golden و smoke همگی PASS |
| QA-004 — Linear Scan / Deadline | **[ ] Qualification باز** | Scan و Detection instrument/benchmark آماده و اجرا شدند؛ محیط GitHub-hosted فقط advisory است، نمونه‌های P99 کم‌تعدادند و حد مطلق P99 تصویب نشده |
| QA-005 — Performance Qualification | **[ ] Qualification باز** | ماتریس ۸ Core/۲۴ Full و Comparator با ۱۰ جفت/۹۵٪ آماده است؛ Core advisory اجرا شد، اجرای Full، ۱۰ جفت مرجع و Soak ۳۰ دقیقه‌ای انجام نشده |

## ۲. تغییرات پیاده‌سازی‌شده

**QA-007:** Recover همهٔ Callbackهای کاربر شامل OnRequest/OnBound/Completion/BindValidator/OnClose؛ ErrCallbackPanic؛ Session.Diagnostics و ساختارهای Typed و Snapshot مستقل؛ حفظ First، Recent Ring، OnClose Panic و Dropped؛ Byte Budget ۶۴ KiB..۲ MiB (پیش‌فرض ۲۵۶ KiB) و Ring ۱..۳۲ (پیش‌فرض ۸)، تخصیص Lazy؛ safe ValueText حداکثر ۱ KiB، Stack کامل؛ Late Panic بعد از Done؛ Atomic Start Claim برای Application؛ لغو Application شروع‌نشده؛ حفظ Completion تمام Requestهای پذیرفته‌شده در Forced Close؛ رزرو Owner OnClose Credit پیش از پذیرش Session و آزادسازی پس از Callback. Worker/Queue نامحدود اضافه نشده است.

**QA-004:** Linear Scan و Timer Manager دست‌نخورده ماندند؛ سنجش اندازهٔ ۳۲/۱۲۸/۵۱۲/۲۰۴۸/۸۱۹۲ Pending؛ ۲۰۴۸ و ۸۱۹۲ صرفاً Fixture مصنوعی‌اند؛ سنجش P50/P95/P99 از Terminal Timeout Settlement، مستقل از زمان شروع Completion؛ CPU profile و Allocation ثبت شدند.

**QA-005:** ۸ Core + ۲۴ Full منتخب؛ ۱۰ جفت اجرای متناوب؛ Comparator با فاصلهٔ اطمینان ۹۵٪ (Bonferroni برای مقایسه‌های متعدد) و PASS/FAIL/INCONCLUSIVE؛ عدم PASS برای دادهٔ ناقص، Zero Baseline یا عدم تأیید محیط/نسخهٔ Baseline؛ امکان Soak ۱۸۰۰ ثانیه‌ای؛ CI فقط Report-Only؛ Versioning و SHA-256 برای Baseline با اجازهٔ صریح مالک.

## ۳. شواهد CI نهایی

Workflow Run [44306325369](https://github.com/majiddarvishan/neta-go/actions/runs/44306325369)، SHA `e852392542c20daf5a79cba2aaee7fbd55e454f8`:

- Go: **PASS** — `make verify` شامل gofmt، go vet، `go test -count=1 ./...` و `go test -race -count=1 ./...`.
- Smoke و مثال‌ها: **PASS** — دو مثال، Request/Response، SendNoWait و Graceful Shutdown.
- Pinned C++ V1 Golden و `go test ./internal/wire`: **PASS** (این شواهد جایگزین تست TCP واقعی C++ در محیط خصوصی نیستند).
- QA-004 Scan و Detection، QA-005 Core advisory و پنج Unit Test مقایسه‌گر پایتون: **PASS** به‌عنوان *اجرای ابزار*، نه عبور از Gate عملکرد.
- Coverage بستهٔ اصلی: **80.8%**، Wire: **91.9%**.

### نمونهٔ نتایج QA-004 روی GitHub-hosted runner

| Pending مصنوعی | Scan در ۱ CPU (ns/op) | Detection P50 (µs) | P95 (µs) | P99 (µs) | نمونه‌های Detection |
|---:|---:|---:|---:|---:|---:|
| 32 | 371.4 | 129.944 | 199.699 | 244.061 | 55 |
| 128 | 1,416 | 134.369 | 160.712 | 168.701 | 55 |
| 512 | 6,046 | 146.805 | 193.633 | 407.835 | 54 |
| 2,048 | 25,020 | 223.001 | 473.090 | 577.549 | 50 |
| 8,192 | 97,944 | 404.165 | 630.906 | 652.264 | 40 |

در Benchmark خالص Scan، Allocation برابر **۰ B/op و ۰ allocs/op** بود؛ CPU profile در Artifact این Run ذخیره شد. تعداد ۴۰–۵۵ نمونهٔ Detection برای ادعای عبور از P99 Qualification کافی نیست. هیچ سقف P99 مطلق یا Baseline مرجع مورد تأیید مالک وجود ندارد.

QA-005 هشت سناریوی Core اجرا شد؛ نتایج Throughput/P50/P95/P99 و Allocation در Artifact Performance همین Run موجود است. وضعیت Full ۲۴ سناریو، ۱۰ جفت اجرای مقایسه‌ای روی محیط مرجع، و آزمون ۳۰دقیقه‌ای: **NOT RUN**.

## ۴. موارد باز و سیاست مالک

- [ ] انتخاب و تأیید محیط مرجع پایدار برای Benchmarkها؛ مالکیت Baseline تاریخی و تأیید صریح هر نسخهٔ جدید.
- [ ] ۱۰ جفت اجرای واقعی QA-005 با Confidence ۹۵٪؛ Full ۲۴ سناریو؛ ۳۰ دقیقه Soak؛ آستانهٔ مطلق QA-004 P99 و SLAهای دیگر تنها با شواهد و تصمیم مالک.
- [ ] Performance Gate همچنان **DISABLED** است. آستانه‌های نسبی ۱۰٪ و ۱۵٪ صرفاً برای تحلیل گزارش‌اند.
- [ ] تصمیم جداگانهٔ مالک برای PR، Merge، Tag و Release. هیچ‌کدام ایجاد نشده‌اند.

**نکته:** دو اجرای اولیهٔ CI به‌ترتیب به‌دلیل gofmt و ناسازگاری تست‌های قدیمی با قراردادهای جدید شکست خوردند و با Commitهای اصلاحی برطرف شدند. فقط Run نهایی فوق معتبر است. Commit نهاییِ مستندسازی، در صورت ثبت با `[skip ci]`، تغییر کد ندارد و از CI تکراری جلوگیری می‌کند.
