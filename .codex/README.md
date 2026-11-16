# Go Simple Net — Engineering Notes

**نام تأییدشده برای کتابخانه Go:** `neta-go` (تصمیم NAM-001، 2026-11-16). نام فعلی مخزن هنوز `go-simple-net` است و تغییری در Module یا کد انجام نشده است.

این پوشه محل ثبت بازبینی‌های فنی، قراردادهای پروتکل، برنامه تسک‌ها و یادداشت‌های ادامه کار پروژه است.

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

## محدودیت فعلی

هیچ Application Source، تست، Build Configuration یا وابستگی‌ای در این مرحله تغییر نکرده است. وضعیت پیاده‌سازی Go Client/Server برابر NOT STARTED است و شروع آن به دستور صریح مالک نیاز دارد.
