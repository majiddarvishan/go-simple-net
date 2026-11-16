# برنامه تسک‌های پیاده‌سازی Go Client/Server سازگار با Pinex

**تاریخ:** 2026-11-16  
**وضعیت:** ANALYSIS RECORDED — IMPLEMENTATION NOT AUTHORIZED  
**مخزن مقصد:** majiddarvishan/go-simple-net  
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

این موارد در صورت تأیید مالک می‌توانند به Backlog نسخه‌های بعدی اضافه شوند.

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
- مخزن فعلی `majiddarvishan/go-simple-net` و ماژول Go فعلاً بدون تغییر باقی می‌مانند.
- Package پیشنهادی آینده `neta` است؛ اما نام Package، مسیر Module، Rename مخزن یا تغییر نام C++ Pinex مجوز اجرا ندارند.
- سازگاری Wire Protocol با Pinex C++ نسخه 1.5.0 باید حفظ شود.
- بررسی تعارض نام/علامت تجاری پیش از انتشار عمومی در Backlog قرار گیرد.

**وضعیت:** CLOSED — نام انتخاب شد؛ بدون مجوز تغییر نام عملیاتی یا کدنویسی. [Decision Log](DECISION_LOG.md).

### Q-004 — انتقال قابلیت‌های جانبی نمونه

پیشنهاد: ابتدا فقط Core Protocol و قابلیت‌های اصلی Pinex پیاده‌سازی شوند؛ Circuit Breaker، Rate Limiter، Deduplication، Priority و Metrics به Backlog مستقل منتقل شوند.

**وضعیت:** پیشنهاد ثبت شده؛ نیازمند تأیید.

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
