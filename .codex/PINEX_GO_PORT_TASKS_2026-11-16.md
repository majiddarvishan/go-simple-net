# برنامه تسک‌های پیاده‌سازی Go Client/Server سازگار با Pinex

> **LATEST OWNER REVISION (2026-11-16):** Q-003-R1 approves bounded, configurable *parallel Application callbacks* per Session; concurrency=1 serial. Old unconditional serial-callback references below are superseded. Internal TCP/Protocol/Timeout/Close engine does not wait on user handlers. See [Decision Log](DECISION_LOG.md).


**تاریخ:** 2026-11-16  
**وضعیت:** ANALYSIS RECORDED — IMPLEMENTATION NOT AUTHORIZED  
**مخزن مقصد:** majiddarvishan/neta-go  
**هدف:** پیاده‌سازی یک کتابخانه TCP Messaging در Go با سازگاری Wire Protocol با Pinex C++ نسخه 1.5.0، شاخه develop.

---

## 1. هدف و محدوده

در این پروژه یک Go Client و Go Server مستقل ساخته می‌شود که بتوانند با Client و Server کتابخانه C++ Pinex ارتباط دوطرفه داشته باشند.

کد ارسالی در فایل ss7.tgz مرجع شناخت قابلیت‌ها و رفتار مورد انتظار است؛ کد آن نباید مستقیماً منتقل یا به‌عنوان مرجع نهایی Wire Protocol در نظر گرفته شود. مرجع پروتکل، سورس و مستندات Pinex در شاخه develop نسخه 1.5.0 است.

### در محدوده

- TCP Client و TCP Server
- Bind/Unbind و مدیریت Session
- ارسال و دریافت Stream Request/Response
- Sequence Number و Correlation
- Timeout درخواست‌ها
- Reconnect قابل تنظیم در Client
- Keepalive و Enquire Link
- Backpressure و محدودیت منابع
- Graceful Shutdown
- تست واقعی Go ↔ C++ Pinex

### خارج از محدوده نسخه اول

- منطق SS7 و SMSC
- وابستگی به Protobufهای برنامه نمونه
- مدل داده و Handlerهای اختصاصی برنامه قبلی
- تغییر Wire Protocol برای افزودن قابلیت‌های اختصاصی
- پیاده‌سازی Client Group مشابه messaging_client_group؛ این مورد می‌تواند بعداً تسک جداگانه باشد.

Payload در سطح کتابخانه opaque است. برنامه مصرف‌کننده می‌تواند Protobuf یا هر قالب دیگری را داخل Payload قرار دهد، اما خود کتابخانه نباید به آن وابسته باشد.

---

## 2. مبنای سازگاری

### مرجع C++ Pinex

- Repository: https://github.com/majiddarvishan/pinex
- Branch: develop
- Version: 1.5.0
- API سطح بالا: messaging_client و messaging_server
- مستندات معماری: https://github.com/majiddarvishan/pinex/blob/develop/docs/architecture.md
- چرخه عمر: https://github.com/majiddarvishan/pinex/blob/develop/docs/lifecycle.md
- Thread Safety: https://github.com/majiddarvishan/pinex/blob/develop/docs/thread-safety.md
- تغییرات پروتکل: https://github.com/majiddarvishan/pinex/blob/develop/docs/migration.md

نسخه 1.5.0 در CMakeLists.txt شاخه develop ثبت شده است. پیش از شروع پیاده‌سازی، SHA دقیق HEAD شاخه develop باید دوباره ثبت شود تا تست‌های سازگاری به یک Snapshot مشخص متصل باشند.

### مرجع کد Go ارسالی

- فایل: ss7.tgz
- SHA-256: 25b3e0cdfed10c170cf1a4aa02945bfe765bfa1378ef979ed5546883dadd8aa3
- شامل 24 فایل Go، از جمله 14 فایل اصلی package و فایل‌های generated Protobuf
- فایل‌های اصلی: client.go، server.go، connection.go، connection_reader.go، connection_writer.go، protocol_handler.go و client_protocol_handler.go

این آرشیو نمونه رفتار و قابلیت‌هاست؛ نه مرجع نهایی Bind wire format.

---

## 3. معماری پیشنهادی

Client و Server باید از یک لایه مشترک استفاده کنند:

1. Wire Codec: Encode/Decode فریم و PDU
2. Session: یک TCP connection، یک Reader، یک Writer و State Machine
3. Request Tracker: تخصیص Sequence، نگهداری Pending Request و Timeout
4. Client: Dial، Bind، Reconnect و ارسال پیام
5. Server: Listen، Accept، Bind admission، Registry و مدیریت Peerها

هیچ منطق Frame، Sequence، Bind یا Unbind نباید جداگانه در Client و Server تکرار شود.

### قراردادهای طراحی پیشنهادی

- API عمومی Go تا حد امکان Thread-safe باشد؛ نوشتن روی هر TCP connection فقط از یک Writer انجام شود.
- Callbackهای یک Session به‌صورت ترتیبی اجرا شوند تا ترتیب پیام‌ها قابل پیش‌بینی باشد؛ سیاست نهایی در تسک API ثبت و تأیید می‌شود.
- Payload در Go از نوع []byte باشد؛ در صورت نیاز Helper برای string ارائه شود.
- هیچ Header یا Envelope اختصاصی خارج از Wire Protocol Pinex اضافه نشود.
- سیستم‌عامل و TCP مسئول انتقال بایت‌ها هستند؛ مرز هر پیام فقط از command_length استخراج می‌شود.
- هیچ Deduplication بر اساس Sequence Number به‌صورت پیش‌فرض انجام نشود؛ Pinex آن را بخشی از Wire Contract تعریف نکرده است.

---

## 4. فهرست تسک‌ها

| شناسه | تسک | اولویت | وضعیت |
|---|---|---|---|
| GO-PINEX-001 | تثبیت Wire Protocol و Compatibility Baseline | P0 | ANALYZED |
| GO-PINEX-002 | طراحی Public API و قرارداد Callbackها | P0 | OPEN |
| GO-PINEX-003 | پیاده‌سازی Wire Codec و PDUها | P0 | OPEN |
| GO-PINEX-004 | پیاده‌سازی TCP Session و Reader/Writer | P0 | OPEN |
| GO-PINEX-005 | Bind Handshake و State Machine | P0 | OPEN |
| GO-PINEX-006 | Control PDUها و Graceful Unbind | P0 | OPEN |
| GO-PINEX-007 | Sequence و Request/Response Correlation | P0 | OPEN |
| GO-PINEX-008 | پیاده‌سازی Go Server | P1 | OPEN |
| GO-PINEX-009 | پیاده‌سازی Go Client | P1 | OPEN |
| GO-PINEX-010 | Backpressure و Resource Admission | P1 | OPEN |
| GO-PINEX-011 | Timeout، Reconnect و Keepalive | P1 | OPEN |
| GO-PINEX-012 | تست‌های Cross-Language با Pinex C++ | P0 | OPEN |
| GO-PINEX-013 | تست پایداری، Race و Churn | P1 | OPEN |
| GO-PINEX-014 | Documentation، Examples و Release Readiness | P2 | OPEN |

### GO-PINEX-001 — تثبیت Wire Protocol

**وضعیت:** مبنای پروتکل تحلیل شده است؛ Q-001 (گزینه A) در 2026-11-16 تأیید شد. تکمیل Golden Frame و تأیید سایر جزئیات همچنان باز است. پیاده‌سازی مجاز نیست.

خروجی‌ها:
- ثبت Header ده‌بایتی
- ثبت تمام Command IDها و Statusها
- ثبت Bind Type و Payloadهای Bind
- ثبت Stream Request/Response
- ثبت رفتار Unbind و Enquire Link
- ثبت اختلاف‌های نمونه Go با Pinex 1.5.0

معیار خروج:
- هیچ اختلاف حل‌نشده‌ای در Byte Layout پروتکل وجود نداشته باشد.
- Golden Frameهای مرجع برای تست Go و C++ آماده باشند.

### GO-PINEX-002 — طراحی Public API

**آخرین وضعیت طراحی:** پیش‌نویس API، Config، Callback، Lifecycle و Timeoutها در [NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md) ثبت شد. مدل Config، Override و Validation در [API-004](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md) با **گزینه A (Functional Options) تأیید و بسته شد**. پیش‌نویس API متدها و Callbackها در [API-001](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md) ثبت شد و هنوز نیازمند تصمیم مالک است. **Q-006 (30s)، Q-007 (10s)، Q-008 (صف خروجی 4 MiB + Fail-Fast)، Q-009 (بستن Session هنگام سرریز صف ورودی) و Q-010 (سقف‌های قابل تنظیم) بسته شدند**؛ قرارداد دقیق API هنوز باز است؛ این سند به معنی آغاز کدنویسی نیست.

موارد لازم:
- تعریف Configهای Client و Server
- تعریف Handlerهای Bind، Request، Response، Timeout، Close و Error
- تعریف Session و روش ارسال Request/Response
- تعریف API برای ارسال Request عادی و ارسال بدون انتظار پاسخ
- تعریف رفتار Send قبل از Bind، بعد از Unbind و هنگام Queue Full
- تعیین Thread Safety و ترتیب Callbackها
- تعیین package/module naming و سازگاری API با سبک Go

معیار خروج:
- API Client و Server با مثال کوتاه قابل استفاده باشد.
- رفتار هر متد در حالت Connected، Disconnected و Closing مشخص باشد.
- هیچ جزئیات غیرضروری TCP یا Codec به مصرف‌کننده تحمیل نشود.

### GO-PINEX-003 — Wire Codec و PDUها

موارد لازم:
- Encode/Decode Header با Big Endian
- اعتبارسنجی command_length
- Encode/Decode Bind Request و Bind Response
- Encode/Decode Stream Request و Stream Response
- Encode/Decode Unbind و Enquire Link
- حفظ command_status در تمام مسیرها
- اعتبارسنجی طول و مقدار Bind Type
- پشتیبانی از Payload خالی و باینری

معیار خروج:
- تست‌های Golden Frame با Byteهای Pinex 1.5.0 برابر باشند.
- Frame ناقص، طول نامعتبر و Command ناشناخته به‌صورت کنترل‌شده مدیریت شوند.
- هیچ Decode نامعتبر باعث Panic نشود.

### GO-PINEX-004 — TCP Session و Reader/Writer

موارد لازم:
- یک Reader Loop برای هر Session
- یک Writer Loop برای هر Session
- خواندن کامل Header و Body بدون فرض یکسان بودن مرز Read و Frame
- نوشتن کامل Frame و مدیریت Short Write
- Deadlineهای Read و Write
- صف ارسال محدود بر اساس Byte Budget
- مدیریت Cancel و Close
- جلوگیری از Goroutine و Socket Leak

معیار خروج:
- Fragmentation و Coalescing در TCP بدون خطا کار کند.
- چند Producer هم‌زمان نتوانند Frameها را در TCP Stream درهم‌تنیده کنند.
- Close و Send هم‌زمان باعث ارسال به Writer متوقف‌شده یا نشت منابع نشود.

### GO-PINEX-005 — Bind Handshake و State Machine

موارد لازم:
- Stateهای Connected/Binding/Bound/Unbinding/Closed
- Bind Request با Bind Type و System ID
- Bind Response با Bind Type و Server System ID
- پذیرش یا رد Bind با Command Status
- جلوگیری از ارسال Stream قبل از تکمیل Bind
- پشتیبانی از uni_direction و bi_direction
- محدودیت طول System ID مطابق Codec در Pinex
- Registry امن برای System IDهای Bind شده

معیار خروج:
- Bind موفق و ناموفق با C++ Pinex کار کند.
- Bind ردشده به Session نیمه‌باز یا Registry باقی‌مانده منجر نشود.
- رفتار uni_direction دقیقاً مطابق قرارداد Pinex باشد.

### GO-PINEX-006 — Control PDU و Graceful Unbind

موارد لازم:
- پاسخ Enquire Link با همان Sequence Number
- Unbind Request و Unbind Response
- بستن Socket پس از ارسال کامل Unbind Response
- انتظار برای Unbind Response در سمت آغازکننده
- Deadline ده‌ثانیه‌ای Unbind مطابق Pinex
- بستن اجباری در صورت Peer بدون پاسخ
- تضمین Close Callback دقیقاً یک بار

معیار خروج:
- Unbind دوطرفه در Go ↔ C++ بدون TCP Reset غیرضروری کامل شود.
- Peer متوقف یا بی‌پاسخ، Session را برای همیشه نگه ندارد.

### GO-PINEX-007 — Sequence و Request/Response Correlation

موارد لازم:
- Sequence افزایشی 32 بیتی در هر Session
- رزرو مقدار صفر و عبور صحیح از Wraparound
- ثبت Pending Request پیش از ارسال
- Rollback Pending در Send Failure
- تطبیق Response با Sequence اصلی
- Timeout مستقل هر Request
- تضمین یک نتیجه نهایی برای هر Request
- تعیین رفتار Response دیررس یا ناشناخته
- پاک‌سازی Pendingها هنگام Close

معیار خروج:
- Sequence Collision باعث از دست رفتن Pending قبلی نشود.
- Send Failure و Timeout Callback دقیقاً یک نتیجه نهایی تولید کنند.
- هیچ Pending Request پس از بسته شدن Session باقی نماند.

### GO-PINEX-008 — Go Server

موارد لازم:
- Listen و Accept
- کنترل خطای Accept با Backoff
- مدیریت تمام Sessionهای در حال Bind و Bind شده
- محدودیت تعداد Pending Bindها
- Handshake Deadline مطلق
- محدودیت اختیاری Pending per source IP
- جلوگیری از Bind هم‌زمان یک System ID
- ارسال Request به Peer مشخص
- ارسال Response با Sequence دریافتی
- ارسال بدون انتظار پاسخ
- Stop و Graceful Shutdown برای تمام Sessionها

Baseline فعلی Pinex برای Pre-Bind:
- Handshake Timeout پیش‌فرض: 30 ثانیه
- حداکثر Pending Session پیش‌فرض: 128
- محدودیت per-IP به‌صورت اختیاری و با مقدار پیش‌فرض صفر (غیرفعال)

معیار خروج:
- Connection بدون Bind منابع را نامحدود نگه ندارد.
- Duplicate System ID رد شود و اتصال فعال قبلی حذف نشود.
- Stop تمام Listener و Sessionها را با Deadline مشخص جمع کند.

### GO-PINEX-009 — Go Client

موارد لازم:
- Dial و Connect Timeout
- ارسال Bind و انتظار برای Bind Response
- ارسال/دریافت Request و Response
- Reconnect اختیاری
- Retry Delay قابل تنظیم
- جلوگیری از Reconnect بعد از Stop صریح
- مدیریت Bind ردشده
- Close و Graceful Unbind
- Callbackهای Session و Error

Baseline فعلی Pinex:
- Connect Timeout پیش‌فرض: 30 ثانیه
- Bind Response Timeout پیش‌فرض: 30 ثانیه
- Reconnect Delay فعلی: 5 ثانیه، در حالت Reconnect فعال

معیار خروج:
- Go Client بتواند به C++ messaging_server متصل شود.
- C++ messaging_client بتواند به Go Server متصل شود.
- Server Restart و Reconnect بدون Session یا Registry قدیمی بازیابی شود.

### GO-PINEX-010 — Backpressure و Resource Admission

موارد لازم:
- محدودیت Queue بر اساس تعداد Byte و نه صرفاً تعداد پیام
- محدودیت اندازه Frame
- محدودیت Pending Handshakeها
- محدودیت تعداد Connectionها
- سیاست روشن برای Queue Full
- کنترل دریافت Data PDU در زمان Backpressure
- ادامه دریافت Control PDUها و تشخیص Disconnect/Unbind
- سقف حافظه برای هر Session و کل Server

معیار خروج:
- بار زیاد باعث رشد نامحدود حافظه یا Goroutine نشود.
- Backpressure موجب نادیده گرفتن Unbind یا Disconnect نشود.
- رفتار Overload مستند و قابل اندازه‌گیری باشد.

### GO-PINEX-011 — Timeout، Reconnect و Keepalive

موارد لازم:
- Timeout درخواست‌های تجاری
- TCP Keepalive قابل تنظیم
- پاسخ به Enquire Link مستقل از مقدار Interval (حتی در حالت 0)
- Enquire Link فقط پس از بی‌ترافیکی واقعیِ دوطرفه Session به مدت Idle Interval قابل پیکربندی؛ 0 یعنی عدم ارسال خودکار
- ریست زمان بی‌ترافیکی با هر Frame واقعاً دریافت‌شده یا ارسال‌شده، بعد از Bind موفق
- تشخیص Peer بی‌پاسخ؛ سیاست Timeout/Failure Threshold نیازمند بررسی جداگانه است
- جلوگیری از چند Enquire Link هم‌زمان
- Retry و Backoff برای Client
- تفکیک خطای Connect، Bind، Session و Request Timeout

نکته: Pinex C++ فعلی به Enquire Link ورودی پاسخ می‌دهد و به‌صورت پیش‌فرض Enquire Link خودکار ارسال نمی‌کند. طبق تصمیم Q-002 مالک، Go می‌تواند Enquire Link را **فقط هنگام Idle بودن** به‌صورت Configurable ارسال کند. این رفتار Wire Protocol را تغییر نمی‌دهد. پیش‌فرض عددی Interval و سیاست Timeout پاسخ هنوز تصویب نشده‌اند.

### GO-PINEX-012 — تست Cross-Language

چهار مسیر اصلی باید تست شوند:
1. Go Client → C++ Pinex Server
2. C++ Pinex Client → Go Server
3. Go Client ↔ Go Server
4. C++ Pinex Client ↔ C++ Pinex Server به‌عنوان کنترل Baseline

سناریوها:
- Bind دوطرفه و یک‌طرفه
- Bind ردشده
- Duplicate System ID
- Payload باینری، خالی و بزرگ
- Request/Response با Sequence
- Status موفق و ناموفق
- Enquire Link
- Unbind
- Fragmented Frame و چند Frame در یک Read
- قطع ناگهانی TCP
- Timeout و Reconnect

معیار خروج:
- تست واقعی Socket بین دو زبان موفق باشد؛ تست Codec به‌تنهایی کافی نیست.
- Byteهای Frame در هر دو جهت با Golden Frameهای Pinex تطبیق داده شوند.

### GO-PINEX-013 — پایداری، Race و Churn

موارد لازم:
- go test -race
- تست Connect/Disconnect پرتعداد
- تست Concurrent Send
- تست Queue Full و Slow Reader
- تست قطع Peer در زمان Backpressure
- تست Shutdown در زمان Bind و در زمان Session فعال
- تست عدم نشت Goroutine، File Descriptor و Pending Request
- تست فشار حافظه و تعداد Connection

معیار خروج:
- تست‌های پایداری تکرارپذیر باشند.
- هیچ Race، Goroutine Leak یا Connection Leak مشاهده نشود.
- سقف منابع در تست بار مشخص و مستند باشد.

### GO-PINEX-014 — مستندات و Release Readiness

موارد لازم:
- README و راهنمای Quick Start
- مثال Go Client و Go Server
- مثال اتصال به C++ Pinex
- مستند API و Lifecycle
- مستند Wire Compatibility و نسخه Pinex مرجع
- مستند Config و Timeoutها
- نسخه‌گذاری و Changelog
- CI برای Format، Vet، Test و Race

معیار خروج:
- مصرف‌کننده بدون مطالعه کد داخلی بتواند Client و Server بسازد.
- نسخه و Compatibility Baseline قابل ردیابی باشد.

---

## 5. قابلیت‌هایی که نباید کورکورانه از نمونه Go منتقل شوند

این قابلیت‌ها در نمونه وجود دارند، اما جزو Wire Protocol Pinex نیستند و باید جداگانه درباره آن‌ها تصمیم گرفته شود:

| قابلیت نمونه | پیشنهاد نسخه اول |
|---|---|
| Circuit Breaker | خارج از Core؛ در صورت نیاز تسک جدا |
| Rate Limiter | خارج از Core؛ قابل افزودن در لایه Policy |
| Deduplicator | به‌صورت پیش‌فرض پیاده‌سازی نشود |
| Priority Queue | فعلاً FIFO محدود و قابل پیش‌بینی |
| Prometheus Metrics | Hook/Interface ساده ابتدا؛ Integration بعدی |
| Application Enquire Link | در Core: فقط هنگام Idle؛ Idle Interval قابل تنظیم، 0 = بدون ارسال خودکار (Q-002 تأیید شد) |
| Protobuf Handlerها | خارج از کتابخانه؛ Payload opaque |

**تصمیم مالک Q-004 (گزینه A، 2026-11-16):** محدوده Core سبک تصویب شد؛ قابلیت‌های جانبی بالا از نسخه اول کنار گذاشته شده و در Backlog نسخه‌های بعدی نگهداری می‌شوند. Backpressure و Admission همچنان در Core اجباری هستند.

---

## 6. پرسش‌ها و تصمیم‌های باز

### Q-001 — مدل اعتبارسنجی Bind

**تصمیم قطعی مالک در 2026-11-16: گزینه A (APPROVED).**

- Wire Protocol مربوط به Pinex 1.5.0 بدون تغییر باقی می‌ماند؛ هیچ Password یا Token به Bind اضافه نمی‌شود.
- Go Server باید Bind Validator Callback داشته باشد که بر اساس حداقل System ID و Remote Address واقعی اتصال و در صورت نیاز Bind Type، Bind را قبول یا رد کند.
- پاسخ موفق با `rok=0x00` و ردشده با `rfail=0xFF` در Header استاندارد Pinex ارسال شود.
- Duplicate System ID نباید اتصال قبلی را حذف یا جایگزین کند.
- بررسی ID و IP به معنای احراز هویت رمزنگاری‌شده نیست؛ برای شبکه‌های غیرقابل اعتماد، کنترل امنیت شبکه جداگانه ضروری است.

**وضعیت:** CLOSED — Option A approved؛ هنوز هیچ پیاده‌سازی مجاز نیست. جزئیات در [DECISION_LOG.md](DECISION_LOG.md).

### Q-005 — رفتار پیش‌فرض نبود Bind Validator

**تصمیم قطعی مالک در 2026-11-16: گزینه B (APPROVED).**

- اگر Go Server هیچ Bind Validator Callbackای دریافت نکرده باشد، درخواست Bind معتبر و دارای `system_id` غیرتکراری را به‌صورت پیش‌فرض می‌پذیرد؛ مطابق رفتار Pinex C++.
- نبود Validator مجوز عبور از اعتبارسنجی پروتکل، State Machine، محدودیت منابع یا بررسی Duplicate System ID نیست.
- اگر Validator تعریف شده باشد، تصمیم Allow/Reject آن اعمال می‌شود.
- Duplicate System ID همواره رد می‌شود و Session قبلی باید حفظ شود.
- رد و پذیرش از همان `bind_resp` و Status استاندارد Pinex استفاده می‌کند؛ هیچ تغییری در Wire Protocol مجاز نیست.
- این Default احراز هویت امنیتی نیست و در شبکه‌های نامطمئن باید Validator و سیاست امنیت شبکه اضافه شود.

**وضعیت:** CLOSED — Option B approved؛ پیاده‌سازی هنوز مجاز نیست. جزئیات در [DECISION_LOG.md](DECISION_LOG.md).

**تست پذیرش آینده:** بدون Validator و ID یکتا => Bind موفق؛ بدون Validator و ID تکراری => Bind ردشده؛ با Validator => تابع تصمیم‌گیر؛ Bind نامعتبر => همچنان ردشده.

### Q-002 — Enquire Link هنگام بی‌ترافیکی

**تصمیم قطعی مالک در 2026-11-16: APPROVED.**

- Enquire Link خودکار نباید با یک Ticker مستقل و در حین تبادل عادی داده ارسال شود.
- روی Client و Server، یک Idle Interval قابل پیکربندی داشته باشیم: مقدار مثبت یعنی پس از این مدت بدون Frame ارسالی **و** دریافتی، Enquire Link Request فرستاده شود؛ مقدار **0 یعنی هیچ ارسال خودکاری انجام نشود**.
- ثبت Activity فقط پس از ارسال/دریافت واقعی Frame باشد؛ شروع سنجش Idle پس از Bind موفق.
- پاسخ به Enquire Link ورودی همیشه فعال باشد، حتی وقتی Idle Interval برابر 0 است.
- حداکثر یک Enquire Link ارسالی و در انتظار پاسخ برای هر Session مجاز است.
- قالب Frame، Command ID و Sequence همان Pinex 1.5.0 باقی می‌مانند.

**وضعیت:** CLOSED — سیاست ارسال مورد تأیید است؛ هنوز پیاده‌سازی مجاز نیست.

**جزئیات موکول‌شده به طراحی Config و Failure Policy:** پیش‌فرض عددی Idle Interval، مهلت پاسخ Enquire Link و نحوه برخورد با عدم دریافت پاسخ. هیچ مقداری برای این موارد از طرف مالک فرض نشود.

**تست پذیرش آینده:** ترافیک مداوم => عدم ارسال؛ Idle با Interval مثبت => ارسال؛ 0 => عدم ارسال خودکار اما پاسخ‌گویی فعال؛ عدم ارسال چند Probe هم‌زمان؛ تست با Pinex C++.

### Q-003 — Thread Safety و Callback Ordering

**تصمیم قطعی مالک در 2026-11-16: APPROVED.**

1. Public APIهای Client، Server و Session از چند Goroutine به‌صورت هم‌زمان قابل فراخوانی باشند.
2. Callbackهای هر Session به ترتیب دریافت پیام و بدون هم‌پوشانی اجرا شوند.
3. Callback کند یا مسدود در یک Session نباید Callbackها و پردازش سایر Sessionها را متوقف کند.
4. ارسال هم‌زمان از چند Goroutine در یک Session باید بدون Interleave بایت‌ها و خرابی Frame ممکن باشد.
5. Disconnect و Shutdown باید Callbackها و عملیات Pending را کنترل‌شده و بدون تکرار اعلان نهایی خاتمه دهند.

جزئیات Signature و Capacity صف‌ها، Deadline تعطیلی و سیاست Backpressure هنوز در GO-PINEX-002/004/007/010 نیازمند طراحی هستند و خودکار تأیید نشده‌اند.

**وضعیت:** CLOSED — تأیید شده، بدون مجوز کدنویسی. [جزئیات تصمیم](DECISION_LOG.md).

### NAM-001 — انتخاب نام کتابخانه Go

**تصمیم قطعی مالک در 2026-11-16: `neta-go` (NETA Go).**

- نام کتابخانه بومی Go برای TCP Messaging برابر `neta-go` است.
- مالک مسیر مخزن GitHub را به `majiddarvishan/neta-go` تغییر داده و دسترسی `main` تأیید شده است. مسیر Module/Package مستقل از تغییر مخزن است و هنوز در API-003 تعیین نشده است.
- Package پیشنهادی آینده `neta` است؛ نام Package و مسیر Go Module هنوز تعیین نشده‌اند. تغییر بیشتر مخزن یا تغییر نام C++ Pinex مجوز اجرا ندارد.
- سازگاری Wire Protocol با Pinex C++ نسخه 1.5.0 باید حفظ شود.
- بررسی تعارض نام/علامت تجاری پیش از انتشار عمومی در Backlog قرار گیرد.

**وضعیت:** CLOSED — نام انتخاب شد و مالک متعاقباً مخزن GitHub را Rename کرد؛ بدون مجوز کدنویسی یا تغییر Module/Package. [Decision Log](DECISION_LOG.md).

### Q-004 — تعیین محدوده نسخه اول neta-go

**وضعیت:** CLOSED — گزینه A در 2026-11-16 به تأیید قطعی مالک رسید؛ فقط محدوده طراحی تأیید شده و هنوز مجوز پیاده‌سازی صادر نشده است.

**تصمیم تأییدشده A — Core سبک:**
- حفظ TCP Client/Server، Codec، Bind/Unbind، Session، Stream Request/Response، Sequence، Timeout و Reconnect.
- حفظ Backpressure، صف محدود FIFO، محدودیت منابع، Shutdown کنترل‌شده، API هم‌زمان امن، Callbackهای ترتیبی و Enquire Link فقط در زمان Idle طبق Q-002.
- انتقال Circuit Breaker، Rate Limiter تجاری، Priority Queue و Prometheus exporter به Backlog آینده.
- عدم پیاده‌سازی Deduplication خودکار بر اساس Sequence؛ Sequence برای Correlation است.
- Protobuf/SS7 Handlerها و Client Group جزو نسخه اول نیستند؛ Payload باینری opaque باقی می‌ماند.
- Metrics Hook ساده و بدون وابستگی خاص صرفاً در مرحله طراحی API بررسی شود و اکنون تعهد قطعی ایجاد نکند.
- Control PDUها در شرایط Backpressure نباید دچار Starvation شوند.

**گزینه B رد شد:** افزودن قابلیت‌های جانبی به Core نسخه اول مصوب نیست.

**معیار خروج:** تصمیم محدوده بسته شد. قابلیت‌های کنارگذاشته‌شده در Backlog مستقل باقی بمانند و هیچ‌کدام بدون تصمیم جدید وارد نسخه اول نشوند. [شرح فنی در Decision Log](DECISION_LOG.md).


### Q-006 — مقدار پیش‌فرض Enquire Link Idle Interval

**وضعیت:** CLOSED — مالک گزینه **B** را در 2026-11-16 تأیید کرد.

- پیش‌فرض `EnquireLinkIdleInterval = 30s` برای Client و Server.
- ارسال خودکار فقط بعد از ۳۰ ثانیه بی‌ترافیکی Session انجام می‌شود؛ هر Frame ارسال یا دریافت‌شده مدت Idle را از نو آغاز می‌کند.
- این تنظیم قابل تغییر است و مقدار صریح `0` ارسال خودکار را خاموش می‌کند؛ پاسخ به Enquire Link دریافتی حتی با `0` فعال می‌ماند.
- حداکثر یک Enquire Link محلی در انتظار پاسخ وجود دارد.
- این تصمیم **Q-007** (مهلت انتظار پاسخ، شکست و Close) را تعیین نکرده است.
- تطابق Wire با C++ Pinex حفظ می‌شود، اگرچه C++ در حالت پیش‌فرض Probe خودکار آغاز نمی‌کند.

**مرجع:** [Public API و Config](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md) و [Decision Log](DECISION_LOG.md).

### Q-007 — Deadline پاسخ Enquire Link

**وضعیت:** CLOSED — تأیید صریح مالک در 2026-11-16.

- Timeout پاسخ Enquire Link برای Client و Server قابل پیکربندی و مقدار پیش‌فرض آن **`10s`** است؛ مستقل از Idle Interval.
- شمارش Deadline پس از **ارسال کامل** Enquire Link Request روی TCP آغاز شود، نه با قرار گرفتن آن در صف.
- فقط `enquire_link_resp` دارای **Sequence دقیقاً متناظر** درخواست Pending، پاسخ معتبر محسوب شود. ترافیک عادی و سایر Frameها این Pending Probe را تأیید یا حذف نمی‌کنند.
- در صورت انقضای Deadline بدون پاسخ متناظر، Session **بسته** شود و Close فقط یک‌بار اعلان شود؛ Requestهای Pending طبق قرارداد Close خاتمه یابند.
- Client فقط اگر Auto Reconnect روشن باشد برای اتصال مجدد اقدام کند؛ Server اتصال معکوس برقرار نمی‌کند.
- حداکثر یک Enquire Link محلی در انتظار پاسخ باشد؛ Enquire Link ورودی همیشه پاسخ داده شود، حتی با Idle Interval صفر.
- تست هم‌زمانی پاسخ و Timeout، خطای Write، ارسال ترافیک نامرتبط و عدم دوباره‌کاری Close در GO-PINEX-012/013 لازم است.
- این تصمیم هیچ تغییری در Wire Protocol Pinex C++ ایجاد نمی‌کند.

**جزئیات در [Decision Log](DECISION_LOG.md). هنوز مجوز کدنویسی صادر نشده است.

### Q-008 — سیاست Backpressure و Default ظرفیت خروجی

**وضعیت:** CLOSED — مالک در 2026-11-16 گزینه **A** را صریحاً تأیید کرد.

- صف خروجی **FIFO محدود به‌ازای هر Session** و پیش‌فرض **4 MiB = 4,194,304 بایت** است؛ قابل پیکربندی.
- فقط Frameهای **Application Stream** با اندازه کامل Serializeشده شامل Header ده‌بایتی Pinex در این سهمیه حساب شوند. این عدد Max Frame Size یا بودجه حافظه کل سرور نیست.
- هنگام پر بودن صف، Send فوراً با خطای اختصاصی **Queue Full** شکست می‌خورد؛ نه انتظار نامحدود، نه Drop پنهانی، نه Partial Enqueue.
- Request ردشده نباید Pending Response Timer یا Sequence ثبت‌شده و ارسال‌نشده باقی بگذارد.
- مسیر/سهمیه Control PDUها مستقل و **محدود** بماند تا Enquire Link و Unbind از صف Application اشباع‌شده متوقف نشوند؛ بدون Interleave فریم و بدون نقض ترتیب الزامی Unbind، همراه با Deadline ده‌ثانیه‌ای آن.
- Race Test، تست FIFO و Accounting با چند Goroutine، خطای Queue Full، آزادسازی ظرفیت، Peer کند، Control under pressure و تست Cross-Language الزامی‌اند.

**مرز تصمیم:** عدد صف Callback ورودی، مقدار سهمیه Control PDU، بودجه کل سرور و تعداد Sessionهای مجاز هنوز تصویب نشده‌اند و تحت **Q-009** بررسی خواهند شد.

**مرجع:** [Decision Log](DECISION_LOG.md)، [Public API/Config Design](NETA_GO_PUBLIC_API_CONFIG_DESIGN_2026-11-16.md) و [Resource Admission Design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md). هیچ کدنویسی مجاز نیست.

### Q-009 — Incoming Callback Backpressure

**وضعیت:** CLOSED — مالک در 2026-11-16 گزینه **B** را صریحاً تأیید کرد.

- اگر Frame جدید Application در صف محدود Callback ورودی همان Session جا نشود، **فقط همان Session فوراً بسته شود**، با دلیل خطای Resource Limit / Inbound Queue Overflow.
- Sessionهای دیگر، Registry و Callbackهای آن‌ها نباید متوقف یا تخریب شوند.
- هیچ Drop پنهانی، انتظار نامحدود، افزایش بی‌حد صف یا بازگشت به سیاست Pause/Resume برای Overflow وجود نداشته باشد.
- Callbackها برای هر Session طبق Q-003 مرتب و بدون هم‌پوشانی‌اند؛ از یک Callback مسدود در کد کاربر نمی‌توان به‌زور خارج شد، پس بستن Transport نباید برای همیشه منتظر آن بماند. دقیق بودن اعلان Close هنوز جزو جزئیات طراحی API است.
- با بسته‌شدن Session، Pendingها، Timerها و سهمیه‌ها باید آزاد شوند و Close دوباره گزارش نشود؛ Client فقط طبق Auto Reconnect فعال مجاز به تلاش مجدد است.
- **عدد ظرفیت صف ورودی و نحوه برخورد با سرریز بودجه سراسری در Q-010 باز هستند** و با انتخاب B تعیین نشده‌اند.

**مرجع:** [Decision Log](DECISION_LOG.md) و [طراحی منابع](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md). بدون مجوز کدنویسی.

### Q-010 — اعداد سقف منابع سراسری و صف ورودی

**وضعیت:** CLOSED — گزینه A با شرط **قابل پیکربندی بودن همه مقادیر** در 2026-11-16 به تأیید مالک رسید.

| تنظیم | پیش‌فرض تأییدشده | Scope |
|---|---:|---|
| `MaxQueuedIncomingBytes` | **4 MiB** | هر Session، صف Callback ورودی |
| `MaxAggregateQueuedFrameBytes` | **256 MiB** | جمع صف‌های Application سرور |
| `MaxBoundSessions` | **1024** | Sessionهای Bindشده سرور |
| `MaxPendingPreBindSessions` | **128** | اتصال‌های Pre-Bind هر Listener |
| `PreBindHandshakeTimeout` | **30s** | Timeout مطلق از پذیرش اتصال |
| `MaxQueuedControlBytes` | **4 KiB** | ظرفیت Control PDU هر Session |

- **تمام تنظیمات فوق قابل تغییرند** و پیش‌فرض‌ها نباید به مقادیر غیرقابل تغییر در کد تبدیل شوند. همچنین سقف خروجی هر Session از Q-008 برابر 4 MiB و آن هم قابل پیکربندی است.
- در صورت رد ارسال خروجی به علت پر شدن سهمیه Session یا Global، **Queue Full و باز ماندن Session**؛ هیچ Frame ناقص یا Pending بی‌صاحب تولید نشود.
- اگر Frame ورودی به علت ظرفیت Callback همان Session یا ظرفیت Global قابل پذیرش نباشد، **فقط همان Session** بسته شود.
- در سقف تعداد Sessionهای Bindشده، Bind جدید با وضعیت سازگار Pinex رد شود و اتصال‌های قبلی حذف نشوند.
- در سقف Pre-Bind، اتصال‌های اضافه قبل از تخصیص Session بسته شوند؛ Deadline مطلق 30s باشد.
- ظرفیت Control محدود و جداگانه است؛ در اشباع غیرقابل‌حل، همان Session طبق قواعد Close بسته شود، بدون نقض ترتیب Frameها یا Unbind Deadline.
- تخصیص حافظه نباید به‌اندازه تمام سقف‌ها برای هر Session از ابتدا صورت گیرد. 256 MiB بودجه صف‌های Application است، نه سقف RSS کل Process.

**باقی‌مانده طراحی API:** نحوه بیان مقدار تنظیم‌نشده/صفر، Validation تنظیمات، ترتیب Callback هنگام Close و Accounting فریم در حال پردازش. [Decision Log](DECISION_LOG.md) و [Resource Design](NETA_GO_RESOURCE_ADMISSION_DESIGN_2026-11-16.md).

**کدنویسی همچنان مجاز نیست.**

### API-004 — تفکیک پیش‌فرض و صفر در تنظیمات Go

**وضعیت:** CLOSED — مالک در 2026-11-16 گزینه A، Functional Options، را تأیید کرد. تصمیم مربوط به روش Config بسته شد؛ نام دقیق متدها و Typeهای صادرشده هنوز در API-001 باز است.

**گزینه A (تأییدشده):** Functional Options با پیش‌فرض‌های کامل داخلی؛ هر تنظیمِ ارائه‌نشده مقدار تأییدشده را بگیرد و Override صریح مستقل اعمال شود. بنابراین نبود Idle Interval یعنی `30s` و تنظیم صریح `0` یعنی غیرفعال شدن Probe خروجی. Instance یک Effective Config اعتبارسنجی‌شده و Immutable نگه دارد.

**گزینه B (انتخاب نشد):** Config Struct با Optional/Pointer Fields به‌عنوان روش اصلی Public API. Internal Config Struct همچنان قابل استفاده است.

**مشترک:** پیش‌فرض‌های Q-008 و Q-010 قابل Override مستقل‌اند؛ صفر/منفی برای سقف منابع الزامی معتبر تلقی نشود و بی‌صدا به Unbounded یا Default تبدیل نگردد؛ خطا قبل از Dial/Listen برگردد؛ تغییر Data منبع پس از ساخت، Instance موجود را تغییر ندهد. Hot Reload هنوز تصویب نشده است.

**مرجع:** [API-004 Configuration Contract](NETA_GO_API_004_CONFIGURATION_CONTRACT_2026-11-16.md). تصمیم شیوه Config بسته است؛ امضای نهایی متدها و Callbackها همچنان در API-001 باز هستند و کدنویسی مجاز نیست.

---

### API-001a — Request/Response completion style

**وضعیت:** CLOSED — مالک گزینه **A** را در 2026-11-16 صریحاً تأیید کرد. **Performance نیز به‌عنوان اولویت اصلی نسخه اول اعلام شد.**

- **گزینه A (تأییدشده):** `SendRequest` بدون انتظار برای Response، نتیجه Admission را فوری برگرداند (مثلاً Accepted Sequence یا Queue Full) و نتیجه نهایی Request فقط از یک مسیر Callback اعلام شود؛ `SendResponse` با Sequence اصلی جواب بدهد.
- **گزینه B (انتخاب نشد):** `Request(ctx, payload)` به‌صورت synchronous/Context-first مسیر اصلی API نسخه اول نیست. در صورت نیاز، Helper آینده با تصمیم مستقل بررسی می‌شود.
- در هر دو گزینه: درخواست/پاسخ Wire مشابه Pinex C++ 1.5.0، Payload خام `[]byte` و عدم Deduplication خودکار، همان حقوق Bind Type و سیاست‌های Resource Admission.
- جزییات Callback/Close/Done، Ownership بایت‌ها و Timer/Reconnect در API-001b/API-002 همچنان Draft هستند.

**مرجع:** [Public API Design](NETA_GO_PUBLIC_API_001_DESIGN_2026-11-16.md) و [Performance Contract](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md). API-001a بسته شد؛ API-001b/PERF-002/003 همچنان بازند. بدون مجوز کدنویسی.

### PERF-001 — Performance-first acceptance methodology

**وضعیت:** OWNER REQUIREMENT ثبت شد — Performance برای نسخه اول بسیار مهم است. عدد هدف/بودجه نهایی هنوز OPEN است؛ هیچ Benchmark اجرا نشده است.

- Benchmark تکرارپذیر برای throughput، p50/p95/p99 latency، allocations/op، bytes/op، تعداد Copy Payload، GC، CPU و memory.
- سناریوهای multi-Session، چند Producer هم‌زمان، اندازه‌های مختلف Payload، Queue Full، Callback کند، Bind/Reconnect/Close، Go↔C++ Pinex.
- Race/Soak/Interop Correctness مستقل از Benchmark Performance؛ بهبود سرعت نباید Frame/Sequence، Goroutine Safety، FIFO، Callback Ordering یا Resource Bounds را نقض کند.
- Benchmark workload، environment و آستانه‌های Acceptance پس از اندازه‌گیری واقعی مشخص شوند؛ **هیچ عدد کارایی تضمین نشده است**.

### PERF-002 — Payload Ownership / Copy Cost

**وضعیت:** CLOSED — گزینه A با تأیید مالک در 2026-11-16. ارسال Async یک Snapshot مستقل از Payload پیش از بازگشت موفق می‌سازد؛ دریافت مستقیماً در بافر اختصاصی قابل نگهداری پس از Callback انجام می‌شود و کتابخانه آن را recycle نمی‌کند. کارایی هدف: یک کپی در مسیر ارسال و بدون کپی اضافی در مسیر دریافت. Pooling داخلی (PERF-004) و ادعای Zero-Copy نهایی نشده‌اند. مرجع [Decision Log](DECISION_LOG.md).

### PERF-003 — Bounded Pending/Completion

**وضعیت:** MODEL APPROVED — مالک گزینه A را در 2026-11-16 تأیید کرد: سقف شمارشی قابل تنظیم برای Pending Request و Completionهای صف‌شده/درحال اجرا به‌ازای Session و مجموع Server؛ هنگام اشباع، Request جدید بدون قطع Session فوراً رد شود و Pending/Frame معلق نماند. سقف‌های عددی، Error name، وضعیت Callback هنگام Forced Close و پیشنهاد سقف شمارش رویدادهای ورودی هنوز OPEN هستند. Goroutine به ازای Request یا صف نامحدود ممنوع است.

**مرجع:** [NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md](NETA_GO_PERFORMANCE_CONTRACT_2026-11-16.md). تا مجوز صریح، هیچ کدی نوشته یا Benchmark اجرا نشود.

---

## 7. ترتیب اجرا

ترتیب وابستگی پیشنهادی:

1. GO-PINEX-001 و ثبت تصمیم‌های بسته‌شده Q-001 و Q-005
2. GO-PINEX-002
3. GO-PINEX-003
4. GO-PINEX-004 و GO-PINEX-005
5. GO-PINEX-006 و GO-PINEX-007
6. GO-PINEX-008 و GO-PINEX-009
7. GO-PINEX-010 و GO-PINEX-011
8. GO-PINEX-012
9. GO-PINEX-013
10. GO-PINEX-014

پیاده‌سازی Client و Server پس از آماده شدن Codec و Session مشترک می‌تواند موازی پیش برود.

---

## 8. محدودیت مجوز فعلی

- هیچ فایل Application Source تغییر نکرده است.
- هیچ تست یا Build اجرا نشده است.
- هیچ API یا Wire Behavior هنوز پیاده‌سازی نشده است.
- این سند صرفاً برنامه و خروجی تحلیل است.
- شروع پیاده‌سازی نیازمند دستور صریح بعدی مالک است.


### PERF-002 approval / PERF-003 next handoff (2026-11-16)

**PERF-002 CLOSED — OWNER APPROVED A**: independent outgoing snapshot on successful async admission; owned inbound payload can outlive callback return. No implementation was authorized. The detailed contract and next PERF-003 count-bound pending/completion design are in [NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md](NETA_GO_PERF_002_003_API_001B_DESIGN_2026-11-16.md). **PERF-003 count-based Option A MODEL APPROVED; numeric / lifecycle details and API-001b remain OPEN**. Proposed counts are illustrative, not approved defaults. No code/test/benchmark execution.


### PERF-003 owner selection and next owner decision (2026-11-16)

**PERF-003 Option A MODEL APPROVED:** per-Session and aggregate Server count-based completion/pending limits, configurable Functional Options, reject new Requests promptly when count exhausted while keeping Session alive. Counts are in addition to Q-008/010 byte queue limits. See [Decision Log](DECISION_LOG.md).

**OPEN:** numeric completion limits; whether to add a separate inbound callback event-count cap; terminal callbacks / credit release on forced Close (**API-001b**); timeout start (**API-002**). Illustrative 1024/65536 **NOT APPROVED**. No code, benchmark, tests or operational changes.


### Q-003-R1 — Owner-approved callback concurrency revision (2026-11-16)

**CLOSED AS REVISED:** Same-Session Application callback dispatch supports a configurable positive `MaxConcurrentApplicationCallbacks` semantic; `1` executes one handler at a time, >1 enables bounded concurrent handlers. FIFO queue/dequeue order remains, but callback completion and response-time order may change under concurrency. Exact public option identifier and default remain OPEN. No unbounded goroutines.

**Required design work (not implementation):** decouple internal Protocol/Writer/Timeout/Pending/Close engine from any user callback; protect waiting queue Q-009/Q-010 bounds; independently budget N active callback payloads and prevent infinite zombie callback accumulation across forced close/reconnect; consider proposed global active callback cap. The per-request completion/OnClose *user callback* lane is still API-001b OPEN and must not be silently finalized by choosing parallel application callbacks.

**Existing closed contracts unchanged:** PERF-002 A owned payload; PERF-003 A bounded pending/completion count model, numeric defaults OPEN; API-001a A nonblocking SendRequest; API-004 A Functional Options; Q-008/009/010 byte and session limits. No source, tests or benchmarks authorized.


### API-001b-N1 — independent user notification lane (2026-11-16)

**OWNER APPROVED STRUCTURE:** `OnClose` and per-Request Completion user callbacks must run in a bounded Notification dispatch path **independent** of Application `OnRequest` workers and internal Protocol/Timeout/Pending/Close work. Q-003-R1 keeps Application handler concurrency configurable (1 = serial). Neither Callback domain may block internal close and settlement. Preserve bounded PERF-003 completion admission; no goroutine per event.

**OPEN:** Notification worker count/default, sublane/sharing and ordering policy, forced-close cancellation vs deferred delivery, `Done` and terminal credit release; exact count defaults and API identifiers. No code, test, benchmark, or operational changes. See [DECISION_LOG.md](DECISION_LOG.md) § API-001b-N1.
