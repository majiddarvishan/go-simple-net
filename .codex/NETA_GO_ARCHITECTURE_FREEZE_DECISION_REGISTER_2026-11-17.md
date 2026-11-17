# neta-go V1 — Consolidated Architecture Freeze Decision Register

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Status:** CLOSED / OWNER APPROVED — OPTION A FOR ALL EIGHT ITEMS (2026-11-17).  
**Implementation:** NOT AUTHORIZED.

این سند تمام تصمیم‌های معماری باقی‌مانده برای Freeze را یکجا جمع می‌کند. تصمیم‌های قبلاً تأییدشده عمداً باز نشده‌اند. پاسخ همه موارد را می‌توان یکجا و با شناسه‌های انتهای سند اعلام کرد.

## 0. موارد بسته‌شده‌ای که نباید دوباره باز شوند

- Q-001 تا Q-010 و قراردادهای قبلی Wire/Idle/Queue.
- API-001a: callback-first و admission غیرمسدودکننده.
- PERF-002: Snapshot خروجی و مالکیت مستقل Buffer ورودی.
- PERF-003: Request credit و completion admission.
- API-001b policy: Forced Close A، N-ORD2، DONE-A و C-ORD-B.
- WORKER-001 تا WORKER-009C: سه Worker class مستقل، fairness، owner-scoped limits، تمام defaults، hard maxima، validation و resource accounting.
- API-002-01: RequestTimeout برابر 30s از Local Admission.
- API-002-02: FrameAssemblyTimeout برابر 30s مطلق از اولین بایت Header در Session متصل.
- API-002-03: ConnectTimeout برابر 30s مطلق برای هر Dial، شامل DNS و TCP؛ deadline زودتر Caller مقدم است.

هیچ‌یک از این موارد با پاسخ‌های این سند تغییر نمی‌کنند.

## 1. API-002-04 — Bind Response Timeout

**پیشنهاد A (توصیه‌شده):**
- Default برابر 30s.
- شروع بلافاصله پس از موفقیت TCP Connect و قبل از ارسال Bind Request.
- کل مرحله Bind را پوشش دهد: گرفتن مالکیت Write، ارسال کامل Bind و دریافت Bind Response متناظر.
- Caller Context cancellation همیشه اعمال شود؛ deadline زودتر Caller مقدم باشد.
- در Timeout، Socket بسته شود و Session متصل/Bound منتشر نشود.
- این deadline مستقل از ConnectTimeout و PreBindHandshakeTimeout باشد.

**B:** مهلت 30s فقط پس از Write کامل Bind Request آغاز شود؛ Writer stall را پوشش نمی‌دهد.

**C:** timeout داخلی نداشته باشیم و فقط Caller Context محدودیت ایجاد کند.

**توصیه:** A؛ تمام مرحله Bind سقف زمانی مستقل دارد و یک Writer معطل نمی‌تواند Client را برای همیشه در مرحله اتصال نگه دارد.

## 2. API-002-05 — Graceful Unbind Timeout

**پیشنهاد A (توصیه‌شده):**
- Default برابر 10s، از لحظه ورود Session به Unbinding.
- یک deadline مطلق برای کل عملیات Graceful Unbind باشد: توقف Admission جدید، Drain پیام‌های Application پذیرفته‌شده در FIFO، Write کردن Unbind و انتظار برای Response متناظر.
- Progress در Write یا دریافت Frame deadline را تمدید نکند.
- با انقضا، Forced Close طبق FC-A انجام شود؛ Pendingها یک‌بار settle و callbackهای شروع‌نشده طبق FC-A لغو شوند؛ callbackهای در حال اجرا متوقف نمی‌شوند.
- Caller Context زودتر می‌تواند عملیات را به Forced Close ببرد.

**B:** مهلت 10s فقط پس از Write کامل Unbind آغاز شود؛ Drain و Write خارج از deadline بمانند.

**C:** timeout داخلی نداشته باشیم و فقط Caller Context محدودیت ایجاد کند.

**توصیه:** A؛ Graceful Shutdown باید سقف زمانی کل داشته باشد، نه فقط سقف انتظار برای Response.

## 3. API-002-06 — Generic Writer Stall Timeout

**پیشنهاد A (توصیه‌شده):**
- Default برابر 30s برای هر Frame از لحظه‌ای که Writer مالکیت آن را می‌گیرد.
- مهلت مطلق است و با Partial Write یا پیشرفت تعداد بایت Reset نمی‌شود.
- برای Application و Control PDUها یکسان است.
- اگر Write کامل نشود، فقط Session مربوطه Forced Close می‌شود؛ بعد از Partial Write ادامه Stream قابل اعتماد نیست.
- Queue wait در این deadline نیست؛ RequestTimeout و UnbindTimeout چرخه‌های سطح بالاتر را محدود می‌کنند.
- Write timeout مستقل از RequestTimeout است، چون SendNoWait و بعضی Control/Response writeها Request timer ندارند.

**B:** Idle-write timeout برابر 10s و با هر پیشرفت موفق Write تمدید شود.

**C:** timeout عمومی نداشته باشیم؛ به Request/Unbind/Caller deadlines اکتفا کنیم.

**توصیه:** A؛ از گیرکردن دائمی Writer و نگه‌داشتن منابع Session جلوگیری می‌کند.

## 4. API-002-07 — Client Reconnect Policy

در تمام گزینه‌ها:
- AutoReconnect به‌صورت پیش‌فرض false است و فقط با Functional Option صریح فعال می‌شود.
- فقط Client reconnect می‌کند؛ Server-side Session خودکار reconnect نمی‌کند.
- هر تلاش موفق یک Session generation تازه می‌سازد؛ sequence و pendingهای نسل قبلی به نسل جدید منتقل نمی‌شوند.
- هیچ Request پذیرفته‌شده‌ای Replay یا Resend نمی‌شود. هر Request نسل قبلی دقیقاً یک بار با نتیجه terminal همان نسل settle می‌شود؛ ارسال مجدد تصمیم برنامه مصرف‌کننده است.
- Stop صریح تمام تلاش‌های بعدی را برای همیشه غیرفعال می‌کند.

**A (توصیه‌شده — Production-safe):**
- Dial failure/timeout و Transport failure غیرمنتظره retryable باشند.
- Bind rejection با Response صریح ناموفق، به‌صورت پیش‌فرض terminal/non-retryable باشد؛ علت رد از Wire به‌اندازه کافی دقیق نیست و retry دائمی برای رد دائمی می‌تواند ایجاد شود.
- Backoff نمایی محدود با Full Jitter: base=1s، سقف=30s؛ پس از 30s اتصال Bound پایدار، backoff به base برگردد.
- یک retry loop و timer محدود برای هر Client؛ بدون goroutine-per-attempt و بدون timerهای انباشته.

**B (Pinex-compatible):**
- AutoReconnect همچنان پیش‌فرض false.
- retry با delay ثابت 5s.
- Bind rejection نیز retry شود؛ احتمال retry تکراری برای رد دائمی پذیرفته می‌شود.

**توصیه:** A؛ در fleet بزرگ از reconnect storm هم‌زمان جلوگیری می‌کند و خطای Bind ردشده را بی‌جهت تکرار نمی‌کند. B فقط اگر تطابق رفتاری دقیق‌تر با Pinex C++ اولویت بالاتری داشته باشد.

## 5. API-002-08 — Stop / Close Lifecycle

**A (توصیه‌شده — Graceful then Forced):**
- Session.Unbind(ctx) عملیات Graceful است: Admission جدید بسته شود، FIFO پذیرفته‌شده Drain شود، سپس Unbind ارسال و Response متناظر انتظار کشیده شود.
- Session.Close() عملیات Forced و idempotent است: Admission/Dispatch/Write جدید را فوراً متوقف کند، transport را ببندد و terminal settlement را آغاز کند. منتظر callbackهای کاربر نمی‌ماند.
- Session.Done() فقط پس از internal teardown و انتشار terminal cause immutable بسته می‌شود؛ join برای callbackهای کاربر نیست.
- Client.Stop(ctx) terminal و idempotent است: reconnect را برای همیشه خاموش کند، Dial/Bind جاری را لغو کند و Session متصل را Graceful Unbind کند.
- Server.Stop(ctx) ابتدا Listener را ببندد، Pre-Bind Sessionها را ببندد و Graceful Unbind همه Sessionهای Bound را هم‌زمان آغاز کند؛ یک Peer کند نباید بقیه را serialize کند.
- اگر Context پیش از تکمیل Graceful shutdown تمام شد، موارد باقی‌مانده Forced Close شوند؛ Stop فقط تا انتشار Done داخلی منتظر بماند و خطای Context را برگرداند.
- Stop/Done هرگز منتظر بازگشت Application/Completion/OnClose callbackها نمی‌مانند. Callbackهای در حال اجرا تا بازگشت واقعی، worker permit و Request/retained-byte credit خود را نگه می‌دارند.
- Stop terminal و غیرقابل Restart است؛ تکرار آن Unbind یا Notification جدید ایجاد نمی‌کند.

**B:** Stop همیشه Forced Close باشد؛ بدون Drain و Unbind. ساده‌تر و سریع‌تر است، اما ممکن است پیام‌های پذیرفته‌شده به Peer نرسند.

**توصیه:** A؛ Graceful path با deadline محدود و fallback قطعی به FC-A، بدون نقض DONE-A و عدم Join کردن callbackهای کاربر.

## 6. API-001 — Final Public API Shape

سیاست‌های API-001a و API-001b بسته هستند. این تصمیم فقط شکل و نام‌های Public API را نهایی می‌کند.

**A (توصیه‌شده — Minimal Go-native API):**

### Construction and startup
- NewClient(opts ...ClientOption) (*Client, error)
- NewServer(opts ...ServerOption) (*Server, error)
- Validation قبل از Socket/Goroutine انجام شود.
- Client.Connect(ctx) تا Bound شدن اولیه یا خطای Dial/Bind برگردد. Context فقط عملیات اتصال اولیه را محدود می‌کند؛ پس از موفقیت، cancel شدن آن Session را متوقف نمی‌کند.
- Server.Start() پس از Listen و آماده‌شدن Accept loop برگردد؛ Server.Addr() آدرس واقعی Listener را بدهد.
- Handlerها و Bind Validator با Functional Options قبل از Start/Connect ثبت شوند؛ Setter زنده برای تغییر Handlerها ارائه نشود.

### Session operations
- SendRequest(payload, completion) (sequence, error): نتیجه Local Admission را فوراً برگرداند؛ Completion دقیقاً یک‌بار نتیجه نهایی را دریافت کند.
- SendNoWait(payload) (sequence, error).
- SendResponse(sequence, payload, status) error.
- Unbind(ctx) error برای Graceful.
- Close() برای Forced و بدون انتظار برای teardown.
- Done() <-chan struct{} برای انتظار تا پایان teardown داخلی.
- Err() error برای terminal cause immutable پس از Done.
- State(), SystemID(), RemoteAddr() برای مشاهده وضعیت و هویت Peer.
- Client.CurrentSession() برای Session generation فعلی؛ اگر Bound نیست، نتیجه nil/false.

### Callback shape
- Client: OnBound و OnClose.
- Server: OnBound، OnRequest و OnClose؛ BindValidator فقط روی Server.
- Completion فقط کنار همان SendRequest ثبت شود؛ Global OnResponse دوم اضافه نشود.
- Application/Completion/OnClose laneها و سیاست concurrency همان تصمیم‌های قبلی را حفظ کنند.

### Errors and options
- Errorها typed و با errors.Is/errors.As قابل بررسی باشند؛ حداقل خانواده‌های NotBound, Closing, Closed, QueueFull, Capacity, FrameTooLarge, RequestTimeout, ConnectTimeout, BindTimeout, WriteTimeout, UnbindTimeout, BindRejected و ProtocolError.
- Terminal cause از طریق Err() پس از Done immutable باشد.
- برای هر setting یک Typed Functional Option مستقل؛ هیچ map عمومی، mutable Config یا hot reload وجود نداشته باشد.
- نام‌گذاری Optionها از الگوی With... استفاده کند و Client-only / Server-only domainها جدا باشند. Per-Session limits/concurrency در Session configuration/inheritance قرار گیرند.
- تنها Public package در V1 همان root package باشد؛ هیچ Codec/PDU/Worker/Resource Ledger/Transport داخلی export نشود.

**B:** Public subpackageهای جدا برای client/server/session/protocol، setterهای Handler بعد از Start و Global Response Handler. انعطاف بیشتری دارد اما سطح وابستگی، چرخه عمر و هماهنگی را پیچیده‌تر می‌کند.

**توصیه:** A؛ کوچک، قابل پیش‌بینی، thread-safe و مناسب Library سبک و performance-first.

## 7. API-003 — Module and Package Architecture

**A (توصیه‌شده):**
- Module path: github.com/majiddarvishan/neta-go
- Root package identifier: neta
- تنها Public package در V1 همان root package باشد.
- فایل‌های exported Client/Server/Session/Options/Errors در root package باشند.
- Codec و Frame/PDU definitions در internal/wire قرار گیرند؛ pure Go و بدون import از root package.
- Reader/Writer, state machine, Request tracker, admission ledgers و worker schedulers در فایل‌های داخلی unexported بمانند. Packageهای داخلی بیشتر فقط در صورت نیاز اثبات‌شده ایجاد شوند.
- هیچ dependency اجباری Protobuf یا C++ وجود نداشته باشد؛ ابتدا standard library.

**B:** Facade در root و چند internal package مستقل برای core/worker/admission/protocol/transport از ابتدا. جداسازی بیشتر، ولی interface/adaptor و dependency graph پیچیده‌تر.

**توصیه:** A؛ یک import ساده برای کاربر، مرز داخلی امن و کمترین پیچیدگی اولیه.

## 8. PROTOCOL-001 — Maximum Frame Size Adoption

**Source-verified fact:** Pinex C++ 1.5.0 در HEAD برابر f7bd1eb779baff0d10e624f20522df345e6ee747 حداکثر Frame ورودی را 1,048,576 bytes total شامل Header ده‌بایتی می‌پذیرد. حداکثر Body برابر 1,048,566 bytes است. این حد Pre-Bind و Bound یکسان است.

**A (توصیه‌شده):** neta-go همین سقف ثابت 1 MiB total را هم برای inbound validation و هم outbound admission enforce کند. سقف عمومی قابل افزایش یا configurable نباشد. Frame ورودی بزرگ‌تر پس از Header validation و قبل از Body allocation باعث Protocol Close همان Session شود. Frame خروجی بزرگ‌تر قبل از Sequence/Pending/Queue admission با FrameTooLarge رد شود. Queue و retained-byte limits مستقل و unchanged باقی بمانند.

**B:** سقف با Functional Option قابل کاهش باشد، ولی هیچ‌گاه از 1 MiB بالاتر نرود؛ انعطاف برای deployment محدود، در برابر configuration بیشتر.

**C:** سقف بالاتر از 1 MiB مجاز باشد؛ با Pinex C++ 1.5.0 سازگار نیست و توصیه نمی‌شود.

**توصیه:** A؛ سقف Wire یک قرارداد ثابت است، نه ظرفیت Resource Admission.

## 9. کارهایی که Owner Decision نیستند و بعداً انجام می‌شوند

- Golden-frame vectors و Go↔C++ socket-level confirmation پس از Implementation Authorization.
- Re-check کردن Pinex develop HEAD پیش از اولین implementation/interoperability test.
- Numeric performance pass/fail targets پس از baseline تکرارپذیر؛ در حال حاضر عدد SLA حدس زده نمی‌شود.
- PERF-004 pooling/allocation policy بر اساس benchmark واقعی؛ پیشاپیش zero-copy یا pooling وعده داده نمی‌شود.
- هیچ‌کدام از موارد بالا مجوز اجرای کد، تست، benchmark یا تغییر dependency ایجاد نمی‌کند.

## پاسخ یکجای مالک

برای بستن تمام موارد باقی‌مانده، گزینه‌ها را یکجا اعلام کن:

API-002-04=A  
API-002-05=A  
API-002-06=A  
API-002-07=A  
API-002-08=A  
API-001=A  
API-003=A  
PROTOCOL-001=A

هر گزینه را می‌توانی جداگانه تغییر دهی یا کنار شناسه اصلاح موردنظرت را بنویسی. پس از تأیید، تصمیم‌ها را در Decision Log و اسناد مرجع ثبت می‌کنم، سپس Architecture Freeze و Implementation Roadmap را نهایی می‌کنم. تا آن زمان Implementation و Test همچنان NOT AUTHORIZED هستند.

## 10. Final authority

Owner response «گزینه‌های A» closes all eight items: API-002-04, API-002-05, API-002-06, API-002-07, API-002-08, API-001, API-003 and PROTOCOL-001. Every item is CLOSED / OWNER APPROVED / OPTION A. Alternatives B/C were not selected. This register is no longer awaiting owner input; all decisions are synchronized in DECISION_LOG.md and corresponding design documents. Architecture Freeze and the implementation roadmap are recorded in NETA_GO_ARCHITECTURE_FREEZE_AND_IMPLEMENTATION_ROADMAP_2026-11-17.md. Implementation remains NOT AUTHORIZED.
