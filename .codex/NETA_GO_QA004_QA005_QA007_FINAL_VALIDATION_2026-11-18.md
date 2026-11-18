# گزارش نهایی پیاده‌سازی و اعتبارسنجی QA-004 / QA-005 / QA-007

**تاریخ:** 2026-11-18  
**مخزن:** `github.com/majiddarvishan/neta-go`  
**شاخه مستقل:** `feature/qa004-qa005-qa007-implementation-2026-11-18`  
**مبنای main (بدون تغییر):** `24b7c5f6eb4e001f30dd8a564a337ca1f506eac8` — PR #22  
**HEAD کد اعتبارسنجی‌شده:** `99dd68848eb73dd15ccf0a178861aa7f7cf82fef`  
**تعداد Commit تا HEAD کد اعتبارسنجی‌شده، نسبت به main:** 34  
**CI معتبر برای همین کد:** https://github.com/majiddarvishan/neta-go/actions/runs/44313332548
**آخرین SHA پس از اصلاح فقط تست QA-007:** `2db90faf67923ea796ed611629390197cffb0e43` — ارزیابی مجدد محلی مطابق خروجی ارسال‌شده توسط مالک (Go Race ×100، Coverage ×100 و `make smoke` همگی PASS؛ کد Production تغییر نکرده).

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

Workflow Run [44313332548](https://github.com/majiddarvishan/neta-go/actions/runs/44313332548)، SHA `99dd68848eb73dd15ccf0a178861aa7f7cf82fef`:

- Go: **PASS** — `make verify` شامل gofmt، go vet، `go test -count=1 ./...` و `go test -race -count=1 ./...`.
- Smoke و مثال‌ها: **PASS** — دو مثال، Request/Response، SendNoWait و Graceful Shutdown.
- Pinned C++ V1 Golden و `go test ./internal/wire`: **PASS** (این شواهد جایگزین تست TCP واقعی C++ در محیط خصوصی نیستند).
- QA-004 Scan و Detection با حداقل ۱۲۰۰ نمونه، QA-005 Core و Full (مجموعاً ۲۴ سناریو) advisory و ۱۶ Unit Test مقایسه‌گر پایتون: **PASS** به‌عنوان *اجرای ابزار*، نه عبور از Gate عملکرد.
- Coverage بستهٔ اصلی: **80.2%**، Wire: **91.9%**.

### نمونهٔ نتایج QA-004 روی GitHub-hosted runner

| Pending مصنوعی | Scan در ۱ CPU (ns/op) | Detection P50 (µs) | P95 (µs) | P99 (µs) | نمونه‌های Detection |
|---:|---:|---:|---:|---:|---:|
| 32 | 523.0 | 155.434 | 201.033 | 269.739 | 1,200 |
| 128 | 2,055 | 167.690 | 226.698 | 257.544 | 1,200 |
| 512 | 8,873 | 167.714 | 211.196 | 259.540 | 1,200 |
| 2,048 | 33,474 | 263.042 | 316.617 | 935.637 | 1,200 |
| 8,192 | 140,368 | 530.695 | 675.858 | 808.792 | 1,200 |

در Benchmark خالص Scan، **۰ allocs/op و ۰ B/op** ثبت شد؛ Profile CPU برای Scan و Detection در Artifact موجود است. افزایش حجم نمونه به ۱۲۰۰ کیفیت مشاهدهٔ P99 را بهتر می‌کند ولی جایگزین مقایسهٔ آماری ۱۰ جفت روی سخت‌افزار مرجع نیست. هیچ سقف مطلق P99 یا Baseline مرجع مورد تأیید مالک وجود ندارد.

QA-005 هر ۸ سناریوی Core و کل ۲۴ سناریوی Full (شامل همان ۸ Core) اجرا شد؛ نتایج Throughput/P50/P95/P99، تخصیص و **Linux CPU-ns/op** در Artifact موجود است. **۱۶ تست Python PASS**، کنترل دقیق فهرست سناریوها و داده‌های ناقص/نامعتبر به صورت INCONCLUSIVE، و Snapshot واقعی Owner Resource Accounting اضافه شد. در تمام **۳۲ سطر خروجی Core و Full**، پنج شاخص انتهای بار کاری شامل Retained App/Completion، Queued App، Outstanding Requests و OnClose Credits صفر بودند و ظرفیت ردشده صفر ثبت شد؛ این ارقام **Peak Memory نیستند** و صلاحیت Baseline را ثابت نمی‌کنند. معیار حافظهٔ نگه‌داری‌شدهٔ قابل انتساب هنوز برای قبول نسبی ۱۵٪ آماده نیست. **۱۰ جفت مرجع و آزمون ۳۰دقیقه‌ای NOT RUN**؛ Performance Gate همچنان خاموش است.

## ۴. موارد باز و سیاست مالک

- [ ] انتخاب و تأیید محیط مرجع پایدار برای Benchmarkها؛ مالکیت Baseline تاریخی و تأیید صریح هر نسخهٔ جدید.
- [ ] ۱۰ جفت اجرای واقعی QA-004/005 با Confidence ۹۵٪؛ ۳۰ دقیقه Soak روی مرجع پایدار؛ معیار واقعی حافظهٔ نگه‌داری‌شدهٔ قابل انتساب؛ آستانهٔ مطلق QA-004 P99 و SLAهای دیگر تنها با شواهد و تصمیم مالک.
- [ ] Performance Gate همچنان **DISABLED** است. آستانه‌های نسبی ۱۰٪ و ۱۵٪ صرفاً برای تحلیل گزارش‌اند.
- [ ] تصمیم جداگانهٔ مالک برای PR، Merge، Tag و Release. هیچ‌کدام ایجاد نشده‌اند.

**نکته:** دو اجرای اولیهٔ CI به‌ترتیب به‌دلیل gofmt و ناسازگاری تست‌های قدیمی با قراردادهای جدید شکست خوردند و با Commitهای اصلاحی برطرف شدند. فقط Run نهایی فوق معتبر است. Commit نهاییِ مستندسازی، در صورت ثبت با `[skip ci]`، تغییر کد ندارد و از CI تکراری جلوگیری می‌کند.

## ۵. تأیید مجدد QA-007 روی سیستم مالک — 2026-11-18

اولین اجرای `scripts/local-final-validation.sh` روی سیستم مالک نشان داد که اجرای مستقل `go-smoke` در تست `TestQA007ForcedClosePreservesAllCompletionAndCredit` با پیام `Completion panic not recorded` شکست خورد؛ سایر ده مرحله، از جمله Soak کامل ۳۰دقیقه‌ای، موفق بودند. بررسی مسیر اجرای Callback مشخص کرد آزاد شدن Request Credit پیش از ثبت Diagnostics در بازیابی Panic انجام می‌شود. بررسی فوری Diagnostics در تست یک پنجرهٔ زمان‌بندی حساس داشت.

Commit تستی `2db90faf67923ea796ed611629390197cffb0e43` فقط انتظار محدود حداکثر سه‌ثانیه‌ای و بررسی نام Callback/Stack را در `qa007_callback_panic_test.go` اصلاح کرد؛ هیچ کد تولیدی تغییر نکرد.

**شواهد تکمیلی (گزارش خروجی کنسول توسط مالک؛ اجرای مستقل توسط دستیار انجام نشده):**

- `go test -race -count=100 -run '^TestQA007ForcedClosePreservesAllCompletionAndCredit$' .` → **PASS**؛ `ok github.com/majiddarvishan/neta-go 1.078s`.
- `go test -cover -count=100 -run '^TestQA007ForcedClosePreservesAllCompletionAndCredit$' .` → **PASS**؛ Coverage تست محدود 25.0% (پوشش کل کتابخانه نیست).
- `make smoke` → **PASS**؛ کل package اصلی Coverage **82.7%**، `internal/wire` Coverage **92.0%**؛ Build مثال‌ها، Bind/Loopback، Request/Response، یک‌طرفه و Graceful Shutdown موفق شدند.

**تصمیم ثبت:** مورد Flaky Local QA-007 **CLOSED**؛ QA-007 در Feature Branch همچنان **[x]** است. نتایج QA-004 و QA-005، ۳۰دقیقه Soak و صفر بودن Ledger بعد از بار کاری از اجرای قبلی نگهداری می‌شوند؛ به‌عنوان اجرای مجدد روی Commit جدید معرفی نمی‌شوند. Baseline مرجع، ده جفت مقایسهٔ معتبر، Peak Retained Memory و SLA مطلق هنوز تأیید نشده‌اند؛ QA-004/005 و Performance Gate بسته/فعال نشده‌اند.

## ۶. ابزار جدید A/A Noise Probe (آمادهٔ اجرای محلی)

- افزوده شد: `scripts/qa005_noise_probe.sh` در Commit `63f80470b60be4c27475d3f681b2475d7c37b6db`؛ این اسکریپت فعلاً روی سیستم مالک اجرا/تأیید نشده است.
- ورودی پیش‌فرض `core` و حالت‌های اختیاری `full` / `qa004`؛ اجرای **۱۰ جفت متناوب A/A** با دو Worktree موقت Detached از SHA یکسان؛ Capture Governor/CPU/Memory/PSI/Swap؛ خروجی `comparison.json`، `noise-summary.json` و تنها یک `.tar.gz` برای بازبینی.
- هدف: سنجش نوسان دستگاه مرجع پیشنهادی **بدون** مقایسهٔ نسخهٔ تاریخی، امضای Baseline، فعال‌سازی Gate، اجرای GitHub CI یا اعمال تغییر در Production.
- **تأکید:** A/A حتی در صورت عملکرد خوب، جای ۱۰ جفت مقایسهٔ بین نسخه‌های مختلف با Fixture یکسان، تأیید مالک، متریک حافظهٔ قابل انتساب و SLAهای مطلق را نمی‌گیرد. QA-004/005 همچنان باز، QA-007 بسته و Performance Gate غیرفعال است.

## ۷. نتیجهٔ A/A Core روی سیستم مالک — 2026-11-18

فایل بارگذاری‌شده: `aa-core-20261118T150600Z-1846220.tar.gz`، دو طرف با SHA یکسان `a6c2b1373e01b716bf8bf8f89d71abe0708630fe`، ۱۰ جفت کامل، ۸ سناریو Core، جمعاً ۱۶۰ سطر سناریوی معتبر. Environment comparable و Matrix complete. هیچ Capacity Rejection و هیچ شمارندهٔ انتهای بار کاری غیرصفر در این ۱۶۰ سطر ثبت نشد.

- شش معیارِ قابل اندازه‌گیری برای هشت سناریو: **۳۱ PASS، ۱۷ INCONCLUSIVE، صفر FAIL**. هشت معیار Retained Attributable Memory جداگانه **INCONCLUSIVE** هستند چون اصلاً Instrument نشده‌اند.
- نوسان زیاد P99 برای برخی سناریوهای یک Session: RR-64B حدود **۳۸٫۵٪**، NoWait-64B حدود **۴۳٪**، NoWait-64KiB حدود **۳۸٫۳٪** و RR-64KiB حدود **۴۹٫۸٪** (Sample CV در ۲۰ اجرا). در مقابل RR-4KiB-S32 حدود **۳٫۸٪** و NoWait-4KiB-S256 حدود **۴٫۴٪** بودند.
- Governor پردازنده برای همهٔ CPUها `powersave` بود؛ Swap حدود ۷٫۲ GiB استفاده‌شده ولی در طول تست فقط شش واحد `pswpin` افزایش و `pswpout` بدون تغییر گزارش شد؛ شواهد کافی برای نسبت‌دادن علت نوسان به Swap وجود ندارد.
- **نتیجهٔ رسمی: A/A Collector PASS، Qualification INCONCLUSIVE، دستگاه مرجع فعلاً تأیید نشده، Baseline مصوب ندارد، Performance Gate همچنان DISABLED.**
- برای تشخیص بهتر اثر طول نمونه، گزینهٔ اختیاری `NETA_QA005_BENCHTIME=1s` فقط برای `core/full` به `qa005_qualify.sh` افزوده شد (Commit `24c057c85c113dc248263ae8e38957469e6cd9a6`)، بدون تغییر پیش‌فرض 250ms یا Fixtureها. اجرای تشخیصی بعدی فقط محلی و نیازمند ارسال فایل خروجی است؛ هیچ نتیجه‌ای از پیش PASS اعلام نشده است.
