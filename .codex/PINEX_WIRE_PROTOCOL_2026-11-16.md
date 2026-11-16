# Pinex 1.5.0 — Wire Protocol Compatibility Contract برای Go

**تاریخ بررسی:** 2026-11-16  
**مرجع:** majiddarvishan/pinex — branch develop — version 1.5.0  
**وضعیت:** SOURCE-BASED ANALYSIS — OWNER CONFIRMATION PENDING  
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

Version 1.5.0 در CMakeLists.txt شاخه develop ثبت شده است. SHA دقیق HEAD شاخه باید قبل از شروع تست‌های Cross-Language مجدداً Pin شود.

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
7. سقف Frame باید با محدودیت 1 MiB فعلی Pinex هم‌راستا باشد؛ مقدار دقیق باید در تست Cross-Language به‌عنوان قرارداد ثابت شود.

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
- Pinex فعلی به Enquire Link دریافتی پاسخ می‌دهد.
- Pinex فعلی از TCP Keepalive سیستم‌عامل استفاده می‌کند و Enquire Link دوره‌ای را به‌صورت پیش‌فرض ارسال نمی‌کند.
- Go باید پاسخ به Enquire Link را همیشه پشتیبانی کند.
- ارسال دوره‌ای Enquire Link در Go باید Option جداگانه و در حالت پیش‌فرض خاموش باشد تا رفتار پیش‌فرض با Pinex یکسان بماند.

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
- سقف Frame هم‌راستا با 1 MiB
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

## 14. تصمیم‌های باز

1. مدل احراز هویت Bind: Wire فعلی credential ندارد. پیشنهاد، Bind Validator Callback بدون تغییر پروتکل است.
2. Enquire Link دوره‌ای: پیشنهاد، اختیاری و پیش‌فرض خاموش.
3. Thread Safety API Go: پیشنهاد، API امن برای فراخوانی هم‌زمان و Callback ترتیبی در هر Session.
4. قابلیت‌های جانبی نمونه Go: پیشنهاد، خارج از Core نسخه اول.

این موارد در فایل PINEX_GO_PORT_TASKS_2026-11-16.md نیز ثبت شده‌اند.
