# Pinex 1.5.0 — Wire Protocol Compatibility Contract برای Go

**تاریخ بررسی:** 2026-11-17  
**مرجع:** majiddarvishan/pinex — branch develop — version 1.5.0  
**وضعیت:** SOURCE-VERIFIED + GO OWNER APPROVED — PROTOCOL-001 OPTION A CLOSED؛ سقف ثابت Maximum Frame برابر 1 MiB total؛ golden frames و cross-language verification هنوز اجرا نشده  
**پیاده‌سازی:** NOT STARTED

---

## 1. منبع حقیقت

مرجع Wire Protocol این پروژه، نسخه 1.5.0 شاخه develop مخزن Pinex است، نه Header/Payload نمونه Go موجود در ss7.tgz.

مراجع اصلی:

- [Pinex architecture.md](https://github.com/majiddarvishan/pinex/blob/develop/docs/architecture.md)
- [Pinex migration.md](https://github.com/majiddarvishan/pinex/blob/develop/docs/migration.md)
- [Pinex lifecycle.md](https://github.com/majiddarvishan/pinex/blob/develop/docs/lifecycle.md)
- [command_id.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/common/command_id.hpp)
- [command_status.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/common/command_status.hpp)
- [bind_type.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/common/bind_type.hpp)
- [bind_request.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/pdu/bind_request.hpp)
- [bind_response.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/pdu/bind_response.hpp)
- [stream_request.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/pdu/stream_request.hpp)
- [stream_response.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/pdu/stream_response.hpp)
- [session.hpp](https://github.com/majiddarvishan/pinex/blob/develop/include/pa/pinex/net/session.hpp)

Version 1.5.0 در CMakeLists.txt شاخه develop ثبت شده است. Snapshot بررسی‌شده در 2026-11-17 برابر `f7bd1eb779baff0d10e624f20522df345e6ee747` است. پیش از هر تست Cross-Language یا شروع پیاده‌سازی، HEAD شاخه باید دوباره بررسی و در صورت تغییر، اختلاف‌ها re-audit شوند.

**حداکثر Frame — SOURCE VERIFIED:** در `include/pa/pinex/net/session.hpp`، بافر دریافت از نوع `detail::flat_buffer<uint8_t, 1024 * 1024>` است و Parser هر `command_length` بزرگ‌تر از `capacity()` را رد می‌کند. `command_length` طول کل Frame و شامل Header ده‌بایتی است. بنابراین Maximum Frame Size برابر **1,048,576 bytes total** و Maximum Body برابر **1,048,566 bytes** است. این سقف در مرحله Pre-Bind و پس از Bind یکسان است. `flat_buffer::capacity()` سقف ثابت پروتکل است، نه اندازه فعلی allocation.

Pinex فعلی در مسیر send_impl() سقف متناظر را پیش از ارسال enforce نمی‌کند؛ با این حال گیرنده Pinex چنین Frameی را نمی‌پذیرد. طبق تصمیم مالک PROTOCOL-001 Option A، neta-go باید سقف ثابت 1 MiB را برای ورودی و خروجی اعمال کند. Frame خروجی بزرگ‌تر پیش از Sequence/Pending/Queue admission رد می‌شود؛ Frame ورودی بزرگ‌تر پس از اعتبارسنجی Header و پیش از Body allocation باعث بستن همان Session/Pre-Bind transport می‌شود. این سقف قابل افزایش یا تنظیم عمومی نیست و مستقل از WORKER-009C است. Golden Frame و Socket-level cross-language verification هنوز اجرا نشده‌اند.

---

## 2. Frame Header

تمام پیام‌ها با Header ثابت 10 بایتی شروع می‌شوند.

| Offset | Size | Field | Encoding |
|---:|---:|---|---|
| 0 | 4 | command_length | Unsigned 32-bit Big Endian |
| 4 | 1 | command_id | Unsigned 8-bit |
| 5 | 1 | command_status | Unsigned 8-bit |
| 6 | 4 | sequence_number | Unsigned 32-bit Big Endian |
| 10 | Variable | Body | PDU-specific |

قواعد:

1. command_length طول کل Frame است و 10 بایت Header را نیز شامل می‌شود.
2. TCP یک Byte Stream است؛ هر Read ممکن است بخشی از Header، بخشی از Body یا چند Frame را برگرداند.
3. Decoder باید Header و Body کامل را مستقل از مرز Read دریافت کند.
4. command_id با بیت بالایی 0x80 نوع Response را مشخص می‌کند.
5. command_status در Header است، نه در ابتدای Body.
6. sequence_number در Header و Big Endian است.
7. سقف دقیق Frame برابر **1 MiB total (1,048,576 bytes شامل Header ده‌بایتی)** است؛ Maximum Body برابر **1,048,566 bytes**. Source این مقدار را در Pinex 1.5.0 تأیید می‌کند. neta-go باید سقف را در هر دو جهت اعمال کند. Golden Frame و Cross-Language test هنوز اجرا نشده‌اند.

---

## 3. Command ID

| Name | Value | Kind | Body |
|---|---:|---|---|
| bind_req | 0x01 | Request | Bind Request |
| bind_resp | 0x81 | Response | Bind Response |
| stream_req | 0x02 | Request | Raw Message Body |
| stream_resp | 0x82 | Response | Raw Message Body |
| unbind_req | 0x03 | Request | Empty |
| unbind_resp | 0x83 | Response | Empty |
| enquire_link_req | 0x04 | Request | Empty |
| enquire_link_resp | 0x84 | Response | Empty |

هر Command ناشناخته یا PDU نامعتبر باید مطابق رفتار Pinex باعث خاتمه کنترل‌شده Session شود؛ نباید به Process Panic منجر شود.

---

## 4. Command Status

| Name | Value | Meaning |
|---|---:|---|
| rok | 0x00 | موفق |
| rfail | 0xFF | رد یا ناموفق |

- Status در بایت شماره 5 Header قرار دارد.
- Bind Response موفق باید status برابر rok داشته باشد.
- Bind Response ردشده باید status برابر rfail داشته باشد.
- Pinex در مسیر Response، Status را به Response Handler می‌دهد.
- Pinex برای Stream Response در API سطح بالا Status را در Callback عمومی نادیده می‌گیرد؛ Go می‌تواند آن را در API خود در دسترس قرار دهد، بدون تغییر Wire Protocol.
- Requestهای عادی Pinex با status برابر rok ارسال می‌شوند؛ Receiver فعلی Pinex status مربوط به Request را برای Dispatch اعمال نمی‌کند.

---

## 5. Bind Request و Bind Response

### Bind Type

Bind Type یک بایت است:

| Name | Value | Semantics |
|---|---:|---|
| uni_direction | 0 | requester-only |
| bi_direction | 1 | درخواست و پاسخ دوطرفه |

### Body مربوط به Bind Request

| ترتیب | Size | Field | Encoding |
|---:|---:|---|---|
| 1 | 1 | bind_type | u8 |
| 2 | Variable | system_id | C-Octet String |

### Body مربوط به Bind Response

| ترتیب | Size | Field | Encoding |
|---:|---:|---|---|
| 1 | 1 | bind_type | u8 |
| 2 | Variable | system_id | C-Octet String |

C-Octet String:
- با یک Null Terminator پایان می‌یابد.
- Codec فعلی Pinex از c_octet_str<20> استفاده می‌کند.
- بنابراین طول System ID باید کمتر از 20 بایت باشد؛ حداکثر 19 بایت داده، به‌علاوه Null Terminator.
- طول بر حسب Byte است، نه تعداد Character.
- Bind Response همان Bind Type درخواست‌شده و System ID سرور را حمل می‌کند.

### رفتار Bind

- هر TCP Connection ابتدا وارد Bind Handshake می‌شود.
- تا قبل از Bind موفق، Stream PDU معتبر نیست.
- Bind Response باید اولین Frame خروجی سرور در اتصال تازه باشد.
- Server می‌تواند Bind را با callback برنامه قبول یا رد کند.
- در Pinex، Bind ردشده با rfail پاسخ داده می‌شود و سپس Session وارد مسیر Unbind/Close می‌شود.
- System ID تکراری در messaging_server فعلی رد می‌شود.
- uni_direction یعنی Client درخواست می‌فرستد اما درخواست ورودی از Peer را به Request Handler تحویل نمی‌دهد؛ این محدودیت در سمت Client که Bind Type را اعلام کرده اعمال می‌شود.

---

## 6. Stream Request و Stream Response

Body هر دو پیام برابر تمام بایت‌های باقی‌مانده Frame است.

- هیچ Length Prefix اضافه‌ای در Body وجود ندارد.
- هیچ Protobuf Envelope اجباری وجود ندارد.
- Payload می‌تواند متن، باینری یا خالی باشد.
- Go باید Payload را به‌صورت []byte نگهداری کند تا صفرهای داخلی و داده غیر UTF-8 تغییر نکنند.
- std::string در API C++ می‌تواند بایت‌های دلخواه را حمل کند؛ بنابراین []byte در Go معادل مناسب آن است.

Correlation:
- فرستنده Request یک Sequence Number اختصاص می‌دهد.
- Receiver همان Sequence Number را در Response بازمی‌گرداند.
- Response باید با Pending Request متناظر تطبیق داده شود.
- Sequence Number یک شناسه Correlation در محدوده Session است؛ قرارداد Deduplication پیام نیست.

---

## 7. Sequence Number

رفتار فعلی Pinex:
- Sequence Number در هر Session نگهداری می‌شود.
- مقدار بعدی به‌صورت افزایشی تخصیص می‌یابد.
- مقدار صفر رزرو شده و پس از Wraparound نیز استفاده نمی‌شود.
- Bind Request و Control Request نیز Sequence دارند.
- Response همان Sequence Request متناظر را بازمی‌گرداند.

پیشنهاد Go:
- شمارنده افزایشی uint32 در هر Session
- عدم استفاده از صفر
- ثبت Pending پیش از قابل مشاهده شدن Request در Writer
- برخورد Sequence یا Wraparound نباید Pending قبلی را overwrite کند.

---

## 8. Enquire Link

- Enquire Link Request و Response فقط Header ده‌بایتی دارند.
- Enquire Link Response باید همان Sequence Number درخواست را برگرداند.
- Pinex C++ فعلی به Enquire Link دریافتی پاسخ می‌دهد، از TCP Keepalive سیستم‌عامل استفاده می‌کند و Enquire Link دوره‌ای را به‌صورت پیش‌فرض ارسال نمی‌کند.
- **تصمیم قطعی Q-002 مالک (2026-11-16):** Go باید Enquire Link خودکار را فقط هنگام **Idle** بودن Session ارسال کند؛ نه با دوره ثابت، هنگام عبور ترافیک عادی.
- در Client و Server یک Idle Interval قابل پیکربندی تعریف می‌شود. مقدار مثبت => پس از عدم ارسال **و** دریافت Frame در مدت تنظیم‌شده Enquire Link Request ارسال شود. مقدار **0 => ارسال خودکار غیرفعال**.
- زمان آخرین Activity با ارسال/دریافت واقعی Frame به‌روز می‌شود و محاسبه پس از Bind موفق آغاز می‌شود؛ ارسال صرفاً در صفِ Writer به‌تنهایی Activity محسوب نمی‌شود.
- پاسخ‌گویی به Enquire Link دریافتی حتی با Interval صفر همیشه برقرار است.
- بیش از یک Enquire Link محلیِ در انتظار پاسخ در یک Session ارسال نشود.
- این تنظیم فقط منطق زمان‌بندی Go است و هیچ Command ID، Byte Layout یا Sequence Rule پروتکل Pinex را تغییر نمی‌دهد.
- مقدار پیش‌فرض عددی Interval و Deadline پاسخ Probe در این تصمیم تعیین نشده‌اند؛ باید در طراحی Public Config/Failure Policy مستقل تعیین شوند.

---

## 9. Unbind و Close

- آغازکننده Unbind، یک unbind_req با Sequence جدید می‌فرستد.
- طرف مقابل unbind_resp را با همان Sequence برمی‌گرداند.
- طرف دریافت‌کننده باید Response را واقعاً روی TCP بنویسد و سپس Socket را ببندد.
- طرف آغازکننده پس از دریافت unbind_resp اتصال را می‌بندد.
- Pinex یک Deadline پروتکلی 10 ثانیه‌ای برای Unbind دارد.
- Unbind نباید باعث شود Session تا ابد در حالت Closing بماند.
- Close Callback باید دقیقاً یک بار اجرا شود.
- Stop صریح Client باید Retry بعدی را غیرفعال کند.
- Server Stop باید Listener را متوقف کند و تمام Sessionهای Bound را Unbind کند.

---

## 10. Transport، Limits و Admission

مبنای فعلی Pinex:
- TCP خام، بدون TLS در خود Wire Protocol
- Header ثابت 10 بایتی
- Maximum Frame Size برابر 1 MiB total، شامل Header ده‌بایتی (Maximum Body برابر 1,048,566 bytes)
- TCP_NODELAY در Session
- TCP Keepalive با inactivity timeout قابل تنظیم
- Pre-Bind Handshake Timeout پیش‌فرض 30 ثانیه
- حداکثر Pending Pre-Bind Session پیش‌فرض 128
- Optional per-IP pending quota با مقدار پیش‌فرض صفر
- Backpressure بر اساس اندازه Send Buffer
- Control PDUها در زمان توقف Dispatch داده نیز باید قابل پردازش بمانند تا Disconnect/Unbind گم نشود.

TLS فقط در صورتی قابل استفاده است که هر دو سمت TCP را با TLS بپیچند؛ این قابلیت بخشی از Wire Protocol فعلی Pinex نیست و در نسخه اول هدف نیست.

---

## 11. اختلاف‌های تأییدشده با نمونه Go در ss7.tgz

### اختلاف A — Bind Type در Wire

نمونه Go:
- Bind Request Body فقط System ID و Null Terminator است.
- Bind Response Body فقط Server ID و Null Terminator است.

Pinex 1.5.0:
- یک بایت Bind Type قبل از System ID در هر دو Bind Request و Bind Response قرار دارد.

نتیجه: Bind نمونه Go با Pinex 1.5.0 سازگار نیست.

### اختلاف B — Command Status در Client Go

نمونه Go:
- ConnectionReader بایت Status از Header را استخراج نمی‌کند و به incomingMessage نمی‌سپارد.
- ClientProtocolHandler.handleBindResponse اولین بایت Body را Status فرض می‌کند.

Pinex 1.5.0:
- Status در Header، offset شماره 5 است.
- Body پاسخ Bind از Bind Type شروع می‌شود.

نتیجه: Client نمونه Go پاسخ Bind از Pinex فعلی را درست تفسیر نخواهد کرد.

### اختلاف C — نام Commandها

نمونه Go از نام submit_req/submit_resp استفاده می‌کند؛ Pinex نام stream_req/stream_resp دارد. مقادیر Wire هر دو یکسان‌اند: 0x02 و 0x82. نام داخلی نباید باعث تفاوت در Wire شود.

### اختلاف D — Payload

نمونه Go در برنامه SS7، Protobuf را داخل Payload می‌گذارد. Pinex فقط بایت‌های خام Message Body را حمل می‌کند. این دو از نظر Transport سازگارند، به شرط آنکه Go Library Payload را بدون تغییر منتقل کند.

### اختلاف E — Sequence

نمونه Go از عدد تصادفی استفاده می‌کند. Pinex از شمارنده افزایشی per-session استفاده می‌کند و صفر را کنار می‌گذارد. Go بهتر است رفتار Pinex را تقلید کند تا مدیریت Pending و Wraparound قابل پیش‌بینی باشد.

### اختلاف F — Deduplication

Deduplication بر اساس Sequence در نمونه Go بخشی از Pinex Wire Contract نیست. Sequence برای Correlation است. Deduplication خودکار در نسخه اول Go نباید باعث حذف Response یا Request معتبر شود.

---

## 12. قواعد ممنوعیت تغییر ناسازگار

برای حفظ سازگاری با C++ Pinex 1.5.0:

- Header نباید تغییر کند.
- Command ID جدید یا تغییر مقدار Commandهای فعلی بدون تصمیم جداگانه مجاز نیست.
- Bind Type نباید حذف شود.
- Status نباید به Payload منتقل شود.
- Stream Body نباید Length Prefix یا Envelope اضافه دریافت کند.
- Sequence Number نباید در Payload تکرار شود.
- هیچ Magic، Version Byte یا Checksum جدیدی به Frame اضافه نشود.
- Authentication field جدید به Bind اضافه نشود؛ چنین تغییری نیازمند نسخه پروتکل جدید و تغییر هماهنگ C++ است.

---

## 13. تست‌های ضروری سازگاری

| Test ID | مسیر | سناریو |
|---|---|---|
| PC-01 | Go Client → C++ Server | Bind bi_direction |
| PC-02 | C++ Client → Go Server | Bind bi_direction |
| PC-03 | Go Client → C++ Server | Bind uni_direction |
| PC-04 | C++ Client → Go Server | Bind rejection / rfail |
| PC-05 | هر دو جهت | Stream Request و Response |
| PC-06 | هر دو جهت | Payload باینری شامل 0x00 |
| PC-07 | هر دو جهت | Payload خالی |
| PC-08 | هر دو جهت | Sequence correlation |
| PC-09 | هر دو جهت | Enquire Link |
| PC-10 | هر دو جهت | Graceful Unbind |
| PC-11 | هر دو جهت | Fragmented Header/Body |
| PC-12 | هر دو جهت | چند Frame در یک TCP Read |
| PC-13 | هر دو جهت | Invalid command_length |
| PC-14 | هر دو جهت | Unknown Command ID |
| PC-15 | هر دو جهت | Max-size Frame |

تست‌های Codec و Golden Frame به‌تنهایی کافی نیستند؛ باید Socket واقعی بین دو پیاده‌سازی اجرا شود.

---

## 14. تصمیم‌های ثبت‌شده و موارد باز

1. **Q-001 — CLOSED / APPROVED OPTION A:** مالک Bind Validator Callback بر اساس System ID و Remote Address را بدون هیچ تغییر در Wire Protocol Pinex 1.5.0 تأیید کرد (2026-11-16). رمز عبور یا Token جدید روی Wire تعریف نمی‌شود. این روش معادل احراز هویت رمزنگاری‌شده نیست.
2. **Q-005 — CLOSED / APPROVED OPTION B (2026-11-16):** اگر Validator تعریف نشده باشد، Go Server یک Bind معتبر با `system_id` یکتا را مانند Pinex C++ می‌پذیرد. با وجود Validator، تصمیم Allow/Reject آن اعمال می‌شود. بررسی فرمت پروتکل، وضعیت Session، محدودیت منابع و Duplicate System ID در هر دو حالت پابرجاست. این Default معادل احراز هویت نیست.
3. **Q-002 — CLOSED / APPROVED (2026-11-16):** ارسال خودکار Enquire Link فقط پس از بی‌ترافیکی واقعی Session به مدت Idle Interval قابل پیکربندی انجام می‌شود؛ `0` ارسال خودکار را غیرفعال می‌کند. پاسخ به درخواست ورودی همواره برقرار است. **Q-006: پیش‌فرض Idle برابر 30s** و **Q-007: پیش‌فرض Timeout پاسخ برابر 10s، قابل پیکربندی** در 2026-11-16 تأیید شدند؛ بدون پاسخ متناظر Session بسته می‌شود.
4. **Q-003-R1 — CLOSED / APPROVED (2026-11-16):** APIهای عمومی Client/Server/Session باید Goroutine-safe باشند؛ Application callbackهای هر Session دارای parallelism محدود و قابل تنظیم هستند (مقدار 1 رفتار serial می‌دهد)؛ Sessionها مستقل می‌مانند و Concurrent Send باید Frame-safe باشد. جزئیات lifecycle در API-002 تکمیل می‌شوند.
5. **Q-004 — CLOSED / APPROVED OPTION A (2026-11-16):** نسخه اول Core سبک است؛ Circuit Breaker، Rate Limiter تجاری، Priority Queue، Prometheus exporter و وابستگی‌های کاربردی به نسخه‌های بعد منتقل می‌شوند؛ Deduplication خودکار Sequence در Core وجود ندارد. محدودیت منابع و Backpressure همچنان الزامی است.

جزئیات در [DECISION_LOG.md](DECISION_LOG.md) و فایل PINEX_GO_PORT_TASKS_2026-11-16.md ثبت شده‌اند.


**Q-006 — CLOSED / APPROVED OPTION B (2026-11-16):** پیش‌فرض Idle Interval برای neta-go Client و Server برابر `30s` است. ارسال Request فقط پس از بی‌ترافیکی واقعی انجام می‌شود، نه با Ticker ثابت در طول تبادل پیام. مقدار `0` فقط ارسال خودکار را خاموش می‌کند. Q-007 نیز تأیید شده است (Timeout پیش‌فرض `10s`، پاسخ متناظر با Sequence و در نبود آن Close Session)؛ این تنظیم هیچ Byte یا Command ID جدیدی به Wire Protocol اضافه نمی‌کند.


**Q-007 — CLOSED / APPROVED (2026-11-16):** مهلت پاسخ Enquire Link به‌صورت مستقل قابل پیکربندی است و پیش‌فرض `10s` دارد. Deadline فقط بعد از ارسال کامل درخواست فعال می‌شود. تنها `enquire_link_resp` با Sequence متناظر آن را تکمیل می‌کند؛ عبور Stream یا هر Frame نامرتبط جای پاسخ را نمی‌گیرد. با Timeout پاسخ، Session بسته می‌شود و فقط Client دارای Auto Reconnect مجاز به تلاش مجدد است. در هر Session فقط یک Probe محلی Pending باشد و پاسخ‌گویی به Probe ورودی حتی هنگام Idle Interval صفر ادامه پیدا کند. قالب Wire ثابت می‌ماند.


## 15. Pinned source baseline و وضعیت Maximum Frame Size

- Pinex `develop` HEAD بررسی‌شده: `f7bd1eb779baff0d10e624f20522df345e6ee747` (CMake version 1.5.0).
- Maximum accepted inbound Frame: **1,048,576 bytes total**, Header included; maximum Body: **1,048,566 bytes**.
- Go compatibility rule: apply the same maximum to inbound validation and outbound admission.
- هنوز هیچ Golden Frame test یا Socket-level Go↔C++ test اجرا نشده است؛ GO-PINEX-001 تا آماده شدن Golden Frameهای قابل‌استفاده و مجوز تست، کامل بسته نمی‌شود.

## 16. PROTOCOL-001 — Go Maximum Frame Size adoption (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- Fixed maximum total Frame: **1,048,576 bytes**, including the 10-byte Header.
- Maximum Body: **1,048,566 bytes**.
- The same limit applies before Bind and after Bind, inbound and outbound.
- Inbound: validate the complete Header and total length before Body allocation; a larger Frame closes only the offending Session/Pre-Bind transport under Protocol Error handling.
- Outbound: reject a Frame larger than the limit before Sequence allocation, Pending Request admission or queue reservation; expose ErrFrameTooLarge.
- No configurable option may raise the limit. Queue/retained-byte limits remain separate.
- This does not change Wire bytes, command IDs, status values, Sequence semantics or Bind/Stream/Unbind/Enquire Link encoding.

**Owner decision:** Option A approved. Source baseline remains Pinex C++ 1.5.0 develop HEAD f7bd1eb779baff0d10e624f20522df345e6ee747. Golden-frame vectors and real bidirectional Go↔C++ socket confirmation remain pending until implementation/test authorization.


## Phase 2 Authorization — Wire Protocol (2026-11-17)

The owner explicitly approved beginning Phase 2 by replying «ادامه بده» after being asked whether Phase 2 (Wire Protocol) was authorized. This approval is scoped to protocol implementation only.

**Authorized scope:** pure-Go `internal/wire` package; Pinex 1.5.0 10-byte frame header and Big Endian fields; frame encode/decode and size validation; Bind Request/Response with bind_type and C-octet system_id; Stream Request/Response raw body; Unbind and Enquire Link control PDUs; sequence fields/semantics; unit and golden-vector tests; documentation and CI.

**Compatibility reference:** Pinex C++ develop HEAD `f7bd1eb779baff0d10e624f20522df345e6ee747`, version 1.5.0. Re-pin/re-audit if the develop branch moves before interoperability testing.

**Hard protocol maximum:** 1,048,576 bytes total including the 10-byte header; maximum Body 1,048,566 bytes. The limit is fixed, not configurable, and applies inbound and outbound.

**Not authorized:** TCP transport/read-write loops, Client/Server runtime integration, lifecycle/shutdown/reconnect, worker scheduler, resource-admission runtime, cross-language socket test harness requiring C++ execution, benchmarks, release or deployment. Those belong to later phases and require separate owner approval.

No frozen architecture decision is reopened. Phase 1 remains complete with CI PASS. Phase 2 implementation must not import root package `neta` from `internal/wire`; the package uses only the Go standard library.
