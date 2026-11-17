# neta-go — Engineering Notes

**نام تأییدشده برای کتابخانه Go:** `neta-go` (تصمیم NAM-001، 2026-11-16). مخزن GitHub با تغییر نام انجام‌شده توسط مالک اکنون `majiddarvishan/neta-go` است (شاخه `main`). مسیر Go Module هنوز تعیین یا تأیید نشده و هیچ تغییری در کد انجام نشده است.

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

## قاعده اعتبار منابع

- گزارش Server Review بر اساس آرشیو ss7.tgz با SHA-256 ثبت‌شده در گزارش است؛ نه سورس موجود در مخزن.
- برنامه Go Client/Server از آرشیو ارسالی به‌عنوان نمونه قابلیت‌ها استفاده می‌کند.
- مرجع سازگاری Wire Protocol، مخزن C++ Pinex روی شاخه develop و نسخه 1.5.0 است.
- نمونه Go ارسالی از نظر Header پایه به Pinex نزدیک است، اما Bind Type و تفسیر Command Status با Pinex 1.5.0 اختلاف دارد.
- پیش از شروع پیاده‌سازی باید SHA دقیق HEAD شاخه develop در Pinex ثبت شود.

## وضعیت فعلی

وضعیت واقعی هر تسک در فایل مرکزی [TASKS.md](../TASKS.md) ثبت می‌شود. Phase 1 تا 5 پیاده‌سازی و CI آن‌ها کامل است؛ Phase 6 به بعد تا دریافت مجوز جداگانه شروع نمی‌شوند.
