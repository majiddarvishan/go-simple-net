# neta-go — Engineering Notes

**نام تأییدشده برای کتابخانه Go:** `neta-go` (تصمیم NAM-001، 2026-11-16). مخزن GitHub اکنون `majiddarvishan/neta-go` و مسیر تأییدشده Go Module برابر `github.com/majiddarvishan/neta-go` است؛ Package عمومی ریشه `neta` است. پیاده‌سازی فازهای ۱ تا ۸ کامل و CI-verified شده است.

این پوشه محل ثبت بازبینی‌های فنی، قراردادهای پروتکل، برنامه تسک‌ها و یادداشت‌های ادامه کار پروژه است.

## وضعیت تسک‌ها

- **[TASKS.md — فهرست اصلی تسک‌ها و وضعیت انجام آن‌ها](TASKS.md)** — مرجع واحد برای مشاهده همه فازها، تسک‌های انجام‌شده، باز و مجوزهای پیاده‌سازی.

## بررسی‌ها و مستندات

- [بازبینی فنی Server به زبان Go — 2026-11-16](SERVER_REVIEW_2026-11-16.md)
- [برنامه تسک‌های Go Client/Server سازگار با Pinex](PINEX_GO_PORT_TASKS_2026-11-16.md)
- [قرارداد Wire Protocol سازگار با Pinex 1.5.0](PINEX_WIRE_PROTOCOL_2026-11-16.md)
- [پیش‌نویس طراحی Public API، Config و Timeoutهای neta-go](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md)
- [API-004: قرارداد تأییدشده Functional Options و Explicit Override](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md)
- [API-001: طراحی Public API، Client/Server/Session و Callback (API-001a تأییدشده)](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md)
- [Performance: قرارداد طراحی و Benchmark Matrix نسخه اول](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md)
- [طراحی Q-008 و Q-009: Resource Admission و Incoming Backpressure](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md)
- [تصمیم‌های معماری و Bind Validator تأییدشده](DECISION_LOG.md)
- [وضعیت و Handoff پروژه](HANDOFF.md)
- [گزارش اجرای Phase 7 — Live Interoperability و Fault Injection](NETA_GO_PHASE_7_IMPLEMENTATION_2026-11-17.md)

## قاعده اعتبار منابع

- گزارش Server Review بر اساس آرشیو ss7.tgz با SHA-256 ثبت‌شده در گزارش است؛ نه سورس موجود در مخزن.
- برنامه Go Client/Server از آرشیو ارسالی به‌عنوان نمونه قابلیت‌ها استفاده می‌کند.
- مرجع سازگاری Wire Protocol، مخزن C++ Pinex روی شاخه develop و نسخه 1.5.0 است.
- نمونه Go ارسالی از نظر Header پایه به Pinex نزدیک است، اما Bind Type و تفسیر Command Status با Pinex 1.5.0 اختلاف دارد.
- پیش از شروع پیاده‌سازی باید SHA دقیق HEAD شاخه develop در Pinex ثبت شود.

## وضعیت فعلی

**مرجع واحد تسک‌ها:** [TASKS.md](TASKS.md) در پوشهٔ `.codex`. همهٔ تسک‌های اجرایی، وضعیت انجام و شواهد CI در همین فایل هستند؛ فایل تکراری روت حذف شده است.

Phase 1 تا Phase 8 تکمیل شده‌اند. Phase 9، مستندات عمومی و تست‌های V1/V2 در `main` ادغام شده‌اند؛ انتشار `v1.0.0` هنوز مجوز ندارد. قبل از انتشار، بازبینی جدید `CHANGELOG.md`، کنترل تنظیمات لازم و **تأیید مستقل مالک برای P9-06** الزامی است.


- [گزارش و Baseline عملکرد Phase 8 و تصمیم نهایی PERF-004](NETA_GO_PHASE_8_PERFORMANCE_BASELINE_2026-11-17.md)
- [برنامه اجرایی Phase 9 و معیارهای پذیرش](NETA_GO_PHASE_9_RELEASE_READINESS_PLAN_2026-11-17.md)
- [راهنمای اجرای تست و Performance Automation](NETA_GO_TEST_AUTOMATION_2026-11-17.md)
