# بازبینی فنی Server به زبان Go

**تاریخ:** 2026-11-16  
**نقش بازبینی:** Senior Go / Networking Engineer  
**وضعیت:** STATIC REVIEW COMPLETE — IMPLEMENTATION NOT STARTED  
**دامنه:** مسیر Server و اجزای مشترک Connection/Protocol در آرشیو ارسالی

---

## 1. خلاصه اجرایی

کد از نظر تفکیک مسئولیت‌ها شروع قابل قبولی دارد: خواندن و نوشتن شبکه از هم جدا شده‌اند، برای هر اتصال صف‌های محدود تعریف شده، نوشتن از یک Goroutine انجام می‌شود، خواندن با io.ReadFull صورت می‌گیرد و برای بخشی از وضعیت‌ها از atomic و mutex استفاده شده است.

با این حال، در وضعیت فعلی چند نقص بنیادی وجود دارد که پیش از استفاده عملیاتی باید تعیین تکلیف شوند. مهم‌ترین آن‌ها عبارت‌اند از:

1. چرخه عمر Server و Shutdown قابل اتکا نیست؛ Accept از بیرون قابل آزادسازی نیست و WaitGroup اتصال‌ها را دنبال نمی‌کند.
2. احراز هویت واقعی در مسیر Bind وجود ندارد و دسترسی به فرمان‌ها به State اتصال محدود نشده است.
3. ورودی Protobuf می‌تواند با فیلدهای nested خالی باعث panic شود.
4. مدیریت Connection در بعضی مسیرهای بستن، Registry را پاک نمی‌کند.
5. پردازش هر پیام با ساخت Goroutine جدید انجام می‌شود؛ محدودیت Semaphore بعد از ساخت Goroutine اعمال می‌شود و ترتیب پردازش حفظ نمی‌شود.
6. تغییر شناسه در زمان Bind با Goroutineهای فعال هم‌زمان است و چند Data Race محتمل وجود دارد.
7. مدیریت درخواست‌های خروجی در شکست ارسال و برخورد Sequence Number، تضمین exactly-once برای Callback ندارد.
8. Load shedding در چند لایه پیام را بدون پاسخ پروتکلی حذف می‌کند.

**جمع‌بندی:** این کد برای یک نمونه اولیه یا محیط کنترل‌شده قابل بررسی است، اما با شواهد موجود نباید آن را از نظر امنیت، Shutdown، کنترل بار و صحت پردازش پیام production-ready تلقی کرد.

شدت امنیتی بعضی موارد به این بستگی دارد که Listener در چه شبکه‌ای در دسترس است. اگر اتصال از شبکه غیرقابل اعتماد ممکن باشد، نبود احراز هویت و اعتبارسنجی ورودی ریسک بسیار جدی‌تری خواهد داشت.

---

## 2. مبنای بررسی و محدودیت‌ها

### 2.1 منبع بررسی

- فایل ورودی: ss7.tgz
- SHA-256:
  25b3e0cdfed10c170cf1a4aa02945bfe765bfa1378ef979ed5546883dadd8aa3
- محتوای آرشیو: 24 فایل Go شامل کد package و فایل‌های generated Protobuf.
- فایل‌های اصلی مسیر Server در پوشه ss7/ قرار دارند.

### 2.2 وضعیت مخزن در شروع بررسی

- Repository: majiddarvishan/go-simple-net
- Branch: main
- HEAD مشاهده‌شده قبل از ثبت گزارش:
  d7f1b25e1d8ebb1899f922f4694a59bfa3241902
- این commit فقط فایل LICENSE را اضافه کرده بود؛ سورس برنامه در آن زمان در مخزن موجود نبود.

بنابراین این گزارش به snapshot آرشیو ارسالی ارجاع می‌دهد. شماره خطوط، شماره خطوط همان snapshot هستند و تا زمان ورود سورس به مخزن، نباید آن‌ها را شماره خطوط نسخه دیگری فرض کرد.

### 2.3 محدودیت اعتبارسنجی

آرشیو شامل موارد زیر نبود:

- go.mod و go.sum
- تست‌های واحد و Integration
- فایل‌های تنظیمات نمونه
- فایل‌های منبع .proto
- کد پیاده‌سازی وابستگی‌های داخلی مانند router_core/pkg/...
- قرارداد رسمی پروتکل و مستندات مصرف‌کنندگان Callback

در نتیجه Build، go test، go vet و go test -race اجرا نشده‌اند. این گزارش یک **Static Code Review** است؛ مواردی که به رفتار وابستگی خارجی یا قرارداد پروتکل وابسته‌اند صریحاً به‌عنوان نیازمند تأیید مشخص شده‌اند.

فایل‌های client.go و client_protocol_handler.go در آرشیو وجود دارند، اما تمرکز این گزارش مسیر Server است. اجزای مشترک Reader، Writer و KeepAlive از این جهت بررسی شده‌اند که Server از آن‌ها استفاده می‌کند.

---

## 3. تصویر معماری فعلی

مسیر اصلی دریافت پیام:

1. Server.Start روی TCP Listen می‌کند و Connection را Accept می‌کند.
2. NewConnection اجزای Connection را می‌سازد و Goroutineهای Reader، Writer، Processor و در صورت فعال بودن KeepAlive را اجرا می‌کند.
3. ConnectionReader فریم را از TCP می‌خواند و در processCh قرار می‌دهد.
4. processLoop پیام را به ProtocolHandler.HandleCommand می‌دهد.
5. برای submit_req و submit_resp، ProtocolHandler یک Goroutine جداگانه ایجاد می‌کند.
6. Server.receiveMessage یا Server.receiveResponse پیام را پردازش و به Handlerهای مشترک می‌سپارد.
7. نتیجه از طریق Callback مالک برنامه به بخش بالادستی تحویل می‌شود.
8. مسیر ارسال، پیام را در sendCh قرار می‌دهد و ConnectionWriter آن را به TCP می‌نویسد.

این تفکیک Reader/Writer از نظر جلوگیری از نوشتن هم‌زمان روی یک TCP stream تصمیم مناسبی است. مسئله اصلی، هماهنگی چرخه عمر، ترتیب پردازش، کنترل منابع و State اتصال بین این اجزاست.

---

## 4. یافته‌های فنی

### F-01 — احراز هویت واقعی در Bind وجود ندارد

**شدت: Critical در صورت دسترسی شبکه غیرقابل اعتماد؛ در غیر این صورت High**

**مراجع:**
- ss7/server.go:477-488
- ss7/protocol_handler.go:135-150

تابع authenticate فقط بررسی می‌کند که شناسه قبلاً در connectedClients ثبت نشده باشد. خود کد نیز صریحاً TODO مربوط به احراز هویت واقعی دارد.

در Bind، شناسه از payload دریافت می‌شود؛ در این مسیر Credential، Secret، امضای درخواست یا اعتبارسنجی هویت قابل مشاهده نیست. net.Listen("tcp", ...) نیز TLS راه‌اندازی نمی‌کند.

**اثر:**
- یک Peer که به پورت دسترسی دارد می‌تواند شناسه دلخواه را ادعا کند.
- تشخیص هویت واقعی طرف مقابل از این کد ممکن نیست.
- اگر این Listener از شبکه عمومی یا شبکه‌ای با اعتماد محدود قابل دسترسی باشد، جعل هویت Client امکان‌پذیر است.

**پیشنهاد:**
- ابتدا Trust Boundary و مدل احراز هویت باید مشخص شود.
- شناسه Client نباید به‌تنهایی Credential تلقی شود.
- ثبت شناسه و رزرو آن باید پس از احراز هویت موفق و به‌شکل اتمیک انجام شود.
- تصمیم TLS یا حفاظت شبکه باید صریحاً در قرارداد استقرار ثبت شود؛ نبود TLS را نباید با احراز هویت اشتباه گرفت.

### F-02 — فرمان‌ها پیش از Bind موفق نیز پردازش می‌شوند

**شدت: High**

**مرجع:** ss7/protocol_handler.go:85-133

HandleCommand برای submit_req، submit_resp، enquire_link_req، enquire_link_resp و unbind_req پیش از بررسی State اتصال وارد Handler مربوطه می‌شود.

در نتیجه Stateهای closed و binding مانع پردازش فرمان‌های عادی نیستند. فقط مسیر Bind خودش Transition انجام می‌دهد.

**اثر:**
- Client می‌تواند قبل از Bind موفق پیام Submit بفرستد.
- پیام با ph.id خالی یا شناسه‌ای که هنوز احراز هویت نشده وارد مسیر پردازش می‌شود.
- مرز امنیتی و قرارداد State Machine عملاً enforce نشده است.

**پیشنهاد:** برای هر Command، State مجاز باید مشخص شود. در حالت معمول، فقط Bind در State اولیه پذیرفته شود و فرمان‌های عملیاتی فقط در State برابر bound پردازش شوند.

### F-03 — Protobuf ناقص می‌تواند کل Process را Panic کند

**شدت: Critical برای ورودی شبکه‌ای قابل کنترل توسط Peer**

**مراجع:**
- ss7/handlers.go:39-162
- ss7/server.go:612-654
- ss7/protobuf/peykasa.ir/smsc/proto/SmsSubmit.pb.go:23-47
- ss7/protobuf/peykasa.ir/smsc/proto/SmsDeliver.pb.go:23-44
- ss7/protobuf/peykasa.ir/smsc/proto/SRIForSm.pb.go:23-33
- ss7/protobuf/peykasa.ir/smsc/proto/StatusReport.pb.go:138-160

proto.Unmarshal موفقیت خود را به معنی کامل بودن تمام nested messageها تضمین نمی‌کند. فیلدهایی مانند Body و SourceAddr از نوع Pointer هستند.

نمونه Dereferenceهای بدون بررسی Nil:

- MO: mo.SourceAddr.AddrValue و mo.Body.DataCoding
- SRI: sri.ServiceCenterAddr.AddrValue و sri.DestinationAddr.AddrValue
- Status Report: mdr.SmrpOriginatingAddr.AddrValue، mdr.RecipientAddress.AddrValue و mdr.Body
- MT: mt.SourceAddr، mt.DestinationSccpAddr، mt.SmrpDestinationAddr و mt.Body

یک Protobuf خالی یا ناقص می‌تواند Unmarshal شود ولی Pointerهای لازم را ایجاد نکند. این مسیرها Recovery محلی ندارند. Panic در یک Goroutine نیز Process را متوقف می‌کند، مگر آنکه در همان Goroutine Recovery مناسبی وجود داشته باشد.

**پیشنهاد:**
- پس از Unmarshal، تمام فیلدهای الزامی و حدود مقادیر بررسی شوند.
- اعتبارسنجی باید قبل از ساخت MessageInfo انجام شود.
- تست Fuzz برای payloadهای خالی، ناقص، بزرگ و دارای nested fieldهای حذف‌شده اضافه شود.
- Recovery سراسری جایگزین Validation نیست؛ فقط می‌تواند آخرین لایه دفاعی برای جلوگیری از Crash ناخواسته باشد.

### F-04 — Shutdown، Listener را از Accept خارج نمی‌کند

**شدت: Critical**

**مراجع:** ss7/server.go:409-475

Listener در متغیر محلی Start نگهداری می‌شود و در ساختار Server ثبت نشده است. حلقه قبل از Accept، Context را بررسی می‌کند؛ اما وقتی داخل Accept مسدود شد، srv.cancel() آن را بیدار نمی‌کند.

Stop نیز Listener را نمی‌بندد.

**اثر:**
- اجرای Stop تضمین نمی‌کند Start برگردد.
- سرور ممکن است پس از درخواست Shutdown همچنان در Accept منتظر بماند.
- کنترل چرخه عمر Listener از API عمومی Server ممکن نیست.

**پیشنهاد:** Listener باید بخشی از Lifecycle سرور باشد. Shutdown باید با بستن Listener یا یک سازوکار معادل، Accept را آزاد کند و سپس منتظر خروج حلقه Accept بماند.

### F-05 — WaitGroup، Goroutineهای واقعی را دنبال نمی‌کند

**شدت: Critical**

**مرجع:** ss7/server.go:438-445

پس از NewConnection، یک WaitGroup Add انجام می‌شود و سپس یک Goroutine خالی ساخته می‌شود که بلافاصله Done می‌کند.

Goroutineهای واقعی Connection در NewConnection و اجزای آن ایجاد می‌شوند و در WaitGroup سرور ثبت نشده‌اند.

همچنین Add بعد از شروع Connection انجام شده است؛ در صورت هم‌زمانی Stop و Accept، امکان Race در ترتیب Add و Wait وجود دارد.

**اثر:**
- srv.wg.Wait() تضمین نمی‌کند Reader، Writer، Processor یا KeepAlive تمام شده باشند.
- Stop می‌تواند پیش از پایان کار واقعی Connection برگردد.
- این WaitGroup در شکل فعلی تقریباً هیچ تضمین Lifecycle مفیدی ایجاد نمی‌کند.

**پیشنهاد:** مالکیت Goroutineها باید روشن باشد. یا هر Connection یک Supervisor و WaitGroup مستقل داشته باشد و Server روی تمام Connectionها Join کند، یا ساختار مدیریت Goroutineها به شکل دیگری با Add-before-Wait و مالکیت مشخص طراحی شود.

### F-06 — Connectionهای Bind نشده در Shutdown مدیریت نمی‌شوند

**شدت: High**

**مراجع:**
- ss7/connection.go:58-145
- ss7/server.go:454-464

Connection بلافاصله پس از Accept ساخته می‌شود، اما فقط پس از Bind موفق در connectedClients ثبت می‌شود.

Context هر Connection نیز از context.Background() ساخته شده و فرزند Context سرور نیست.

Stop فقط Connectionهای موجود در connectedClients را جمع‌آوری و Close می‌کند.

**اثر:** Connectionهایی که هنوز Bind نشده‌اند، در Registry سرور نیستند و از مسیر Stop بسته نمی‌شوند. در نتیجه سرور Registry کاملی از Socketهای پذیرفته‌شده ندارد.

**پیشنهاد:** تمام Socketهای پذیرفته‌شده باید از لحظه Accept تا پایان کامل Lifecycle در یک Registry یا Supervisor ثبت شوند؛ وضعیت Bound فقط یک Attribute از Connection باشد، نه شرط ثبت آن در Registry.

### F-07 — بستن Server-Initiated اتصال، Registry را پاک نمی‌کند

**شدت: High**

**مراجع:**
- ss7/connection.go:152-165
- ss7/connection.go:263-272
- ss7/connection.go:291-305
- ss7/connection_keepalive.go:120-131
- ss7/server.go:518-544

Connection.Close در صورت Bound بودن ابتدا State را به closing تغییر می‌دهد. سپس Writer با بسته‌شدن Context، Socket را می‌بندد و Reader در نهایت onReaderClose را اجرا می‌کند.

اما onReaderClose فقط وقتی State == bound باشد removeConnection را صدا می‌زند. اگر Close از سمت Server یا KeepAlive شروع شده باشد، State قبلاً closing شده و حذف از Registry انجام نمی‌شود.

KeepAlive نیز مستقیماً c.Close را به‌عنوان Callback بسته‌شدن دریافت می‌کند.

**اثر:**
- Connection بسته‌شده ممکن است در connectedClients باقی بماند.
- Round-robin می‌تواند آن را دوباره انتخاب کند.
- ارسال به آن Connection خطا می‌دهد.
- Gauge تعداد اتصال‌ها و Registry از واقعیت فاصله می‌گیرند.

در Stop نیز Mapها Reset می‌شوند ولی Gauge مربوط به تعداد اتصال‌ها صریحاً صفر نمی‌شود.

**پیشنهاد:** حذف Connection باید مستقل از State جاری و با هویت خود Pointer انجام شود؛ به‌طوری‌که فقط همان Connection ثبت‌شده حذف شود. Cleanup باید Idempotent باشد و تمام مسیرهای بسته‌شدن را پوشش دهد.

### F-08 — بررسی و ثبت Client ID اتمیک نیست

**شدت: High**

**مراجع:** ss7/server.go:477-488 و 507-540

authenticate وجود ID را بررسی می‌کند و Lock را آزاد می‌کند. ثبت واقعی ID بعداً در addConnection انجام می‌شود.

دو Connection می‌توانند تقریباً هم‌زمان یک ID را بررسی کنند و هر دو نتیجه موفق بگیرند. سپس هر دو در addConnection همان کلید را ثبت می‌کنند.

addConnection نیز قبل از Append کردن ID به Slice، وجود قبلی آن را بررسی نمی‌کند.

**اثر:**
- کلیدهای تکراری در connectedClientsKey
- ناسازگاری Index Map با Slice
- افزایش نادرست Gauge
- حذف Connection جدید توسط Callback بسته‌شدن Connection قدیمی، چون removeConnection فقط ID را بررسی می‌کند و Pointer را مقایسه نمی‌کند.

**پیشنهاد:** عملیات Reserve/Authenticate/Register باید یک عملیات اتمیک باشد. هنگام Remove نیز شرط connectedClients[id] == conn باید برقرار باشد تا Connection قدیمی نتواند جایگزین جدید را حذف کند.

### F-09 — ساخت Goroutine برای هر پیام، محدودیت واقعی ایجاد نمی‌کند

**شدت: High**

**مراجع:**
- ss7/protocol_handler.go:106-109
- ss7/protocol_handler.go:185-194
- ss7/server.go:568-610

processLoop پیام‌ها را دریافت می‌کند اما برای هر submit_req و submit_resp یک Goroutine جدید می‌سازد.

محدودیت processSem داخل receiveMessage و receiveResponse اعمال می‌شود؛ یعنی پس از ساخته‌شدن Goroutine.

**اثر:**
- تعداد Goroutineهای منتظر یا در حال اجرا می‌تواند به‌سرعت افزایش یابد.
- Semaphore از تعداد Goroutineهای ساخته‌شده جلوگیری نمی‌کند.
- ترتیب پردازش پیام‌های یک Connection تضمین نمی‌شود؛ Callback پیام دوم ممکن است قبل از پیام اول اجرا شود.
- Callbackهای مالک برنامه از چند Goroutine به‌صورت هم‌زمان فراخوانی می‌شوند.

**پیشنهاد:** پردازش باید از طریق یک Queue محدود و Worker Pool با ظرفیت مشخص انجام شود. اگر ترتیب پیام‌های هر Client مهم است، باید ترتیب در سطح همان Client حفظ شود. قرارداد Thread Safety مربوط به Callback نیز باید مستند باشد.

### F-10 — Data Race در زمان Bind

**شدت: High**

**مراجع:**
- ss7/protocol_handler.go:135-183
- ss7/connection.go:135-145
- ss7/connection.go:291-305
- ss7/connection_state.go:20-30, 69

Reader، Writer، Processor و KeepAlive پیش از Bind موفق شروع شده‌اند.

پس از Bind، OnBound شناسه را در چند فیلد String تغییر می‌دهد:
- c.circuitBreaker.id
- c.stateManager.id
- c.writer.id
- c.reader.id
- c.keepAlive.id

این فیلدها در Goroutineهای دیگر برای Log یا عملیات داخلی خوانده می‌شوند. بعضی از آن‌ها بدون استفاده از همان Lock نوشته می‌شوند. ProtocolHandler.id نیز در Bind تغییر می‌کند و Getter آن Synchronization ندارد.

**اثر:** Race Detector می‌تواند روی این فیلدها هشدار بدهد و در اجرای واقعی، رفتار هم‌زمانی قابل اتکا نیست.

**پیشنهاد:** شناسه باید پیش از شروع Goroutineهای وابسته تعیین شود، یا از یک سازوکار immutable/atomic یا Lock مشترک استفاده شود. ترجیح من این است که Connection قبل از Bind نیز یک شناسه داخلی ثابت داشته باشد و Client ID فقط پس از احراز هویت به‌صورت یک مقدار immutable منتشر شود.

### F-11 — مدیریت Sequence و Pending Request تضمین Callback یکتا ندارد

**شدت: High**

**مراجع:** ss7/protocol_handler.go:185-216

در SendRequest:

1. Sequence با rand.Uint32() ساخته می‌شود.
2. درخواست در userData با SetTemporary ثبت می‌شود.
3. سپس ارسال به Sender واگذار می‌شود.

مشکلات:

- برای Sequence تصادفی، برخورد با Sequence درخواست معلق دیگر بررسی نمی‌شود.
- در صورت خطای Sender، رکورد Pending حذف نمی‌شود.
- در Server، خطای conn.sendRequest باعث Callback فوری خطا می‌شود؛ رکورد Pending باقی‌مانده می‌تواند بعداً Timeout شود و Callback دوم ایجاد کند.
- در handleSubmitResponse، رکورد قبل از Decode پاسخ حذف می‌شود. اگر Decode ناموفق باشد، دیگر Timeout جایگزینی برای آن درخواست اجرا نمی‌شود.

**پیشنهاد:**
- Sequence باید با مدیریت برخورد و بررسی کلیدهای Pending تولید شود.
- اگر ارسال پذیرفته نشد، Pending باید Rollback شود.
- حذف Pending، Timeout و دریافت Response باید یک State Transition اتمیک داشته باشند.
- برای هر Request باید قرارداد دقیقاً یک نتیجه نهایی تعریف شود.
- نوع Response نیز باید با نوع Request اصلی سازگار باشد.

رفتار داخلی temap.TimedMap در این آرشیو موجود نیست؛ بنابراین جزئیات هم‌زمانی خود آن نیازمند بررسی وابستگی است.

### F-12 — خطای ساخت پیام، نتیجه نهایی را به Callback اعلام نمی‌کند

**شدت: Medium تا High**

**مراجع:** ss7/server.go:684-746

در SendSRI و SendDeliver، اگر BuildSRIRequest یا BuildMTRequest خطا بدهد، Counter افزایش می‌یابد و تابع برمی‌گردد؛ اما Error به MessageInfo تنظیم و Callback نتیجه فراخوانی نمی‌شود.

در نتیجه Caller ممکن است هیچ نتیجه‌ای دریافت نکند.

**پیشنهاد:** تمام مسیرهای Failure باید به یک نتیجه نهایی مشخص برسند؛ از جمله Build Failure، نبود Connection، رد شدن در Queue و خطای نوشتن شبکه. Callback نباید در بعضی مسیرها فراموش شود یا دوبار فراخوانی شود.

### F-13 — چند مسیر Load Shedding پیام را بدون نتیجه پروتکلی حذف می‌کنند

**شدت: High**

**مراجع:**
- ss7/connection_reader.go:92-98
- ss7/protocol_handler.go:87-99
- ss7/server.go:581-610
- ss7/connection.go:190-226

در شرایط مختلف پیام Drop می‌شود:
- پر شدن processCh
- پر شدن ظرفیت processSem
- عبور از Rate Limit
- تشخیص Duplicate
- پر شدن صف ارسال یا Timeout در پذیرش پیام

در بسیاری از این مسیرها فقط Log یا Metric ثبت می‌شود و هیچ نتیجه پروتکلی برای فرستنده ارسال نمی‌شود.

**اثر:** از دید Client، درخواست ممکن است صرفاً Timeout شود. همچنین بخشی از Dropها در Metric مربوط به droppedMessages ثبت نمی‌شوند.

**پیشنهاد:** برای هر نوع Drop باید سیاست مشخص باشد: Backpressure، Reject صریح، Disconnect یا Drop کنترل‌شده. این تصمیم باید با قرارداد پروتکل سازگار باشد. شمارنده‌های Drop نیز باید همه مسیرها را پوشش دهند.

### F-14 — Deduplication فقط بر اساس Sequence Number است

**شدت: Medium تا High**

**مراجع:** ss7/deduplicator.go:9-55 و ss7/protocol_handler.go:94-99

Deduplicator فقط Sequence Number را ذخیره می‌کند و Command ID در کلید لحاظ نمی‌شود. این بررسی برای تمام Commandها، از جمله Bind و Enquire Link و Responseها انجام می‌شود.

پنجره تشخیص Duplicate برابر 60 ثانیه است، در حالی که Cleanup Retention برابر 5 دقیقه تعریف شده است.

**اثر:** استفاده مجدد از یک Sequence Number در Command متفاوت می‌تواند به‌اشتباه Duplicate تلقی شود. پیام معتبر در نتیجه Drop می‌شود.

**پیشنهاد:** ابتدا باید semantics مربوط به Sequence Number در پروتکل روشن شود. سپس کلید Deduplication بر اساس هویت مناسب پیام تعریف شود؛ احتمالاً ترکیبی از Command Type و Sequence Number یا شناسه درخواست. سیاست Dedup برای Control Messageها نیز باید جداگانه تعیین شود.

### F-15 — Unbind پاسخ می‌دهد اما اتصال را نمی‌بندد

**شدت: Medium**

**مرجع:** ss7/protocol_handler.go:124-128

در دریافت unbind_req، پاسخ Unbind ارسال می‌شود و سپس Error برگردانده می‌شود.

processLoop فقط Error را Log می‌کند و ادامه می‌دهد. در این مسیر State به closing تغییر نمی‌کند و Connection.Close نیز فراخوانی نمی‌شود.

**اثر:** ارسال Unbind الزاماً به پایان Session منجر نمی‌شود و Connection می‌تواند در Registry باقی بماند.

**پیشنهاد:** رفتار دقیق پس از Unbind باید در State Machine تعریف شود: پاسخ، توقف پذیرش Command جدید، Drain محدود و سپس Close.

### F-16 — اعتبارسنجی مقادیر Configuration ناقص است

**شدت: High**

**مرجع:** ss7/server.go:233-329

برای مقادیر زیر Parse انجام می‌شود، اما محدوده مجاز بررسی نمی‌شود:
- timeout
- write_timeout
- read_timeout
- enquire_link_interval
- messages_per_second
- burst_size

نمونه مشخص: اگر enquire_link_interval صفر یا منفی باشد و KeepAlive فعال باشد، time.NewTicker می‌تواند Panic کند.

همچنین Timeout اصلی به uint32 تبدیل می‌شود؛ مقدار منفی می‌تواند به یک مقدار بسیار بزرگ تبدیل شود.

Callbackهای checkLicenseCallback، validationCallbackFunc و callback نیز Nil بودنشان در Constructor بررسی نمی‌شود.

**پیشنهاد:** Constructor باید مقادیر را Validate کند و در صورت نامعتبر بودن، Error قابل فهم برگرداند. Callbackهای الزامی نیز باید قبل از ساخت Server بررسی شوند.

### F-17 — ظرفیت حافظه در سطح کل Server محدود نشده است

**شدت: High**

**مراجع:**
- ss7/connection.go:18-21, 25-30
- ss7/connection_reader.go:59-89
- ss7/connection_writer.go:18-26
- ss7/server.go:24-32

برای هر Connection:
- صف دریافت ظرفیت 200 پیام دارد.
- صف ارسال ظرفیت 100 پیام دارد.
- حداکثر طول فریم ورودی حدود 1 MiB است.

بنابراین صرفاً ظرفیت داده صف‌های هر Connection در حالت بدبینانه می‌تواند بسیار زیاد باشد؛ حدود 200 MiB برای صف دریافت و تا حدود 100 MiB برای صف ارسال، بدون احتساب سربار و سایر حافظه‌ها.

هیچ Max Connection یا Global Memory Budget در کد مشاهده نشد.

**اثر:** تعداد زیاد Connectionها یا پیام‌های بزرگ می‌تواند فشار حافظه شدیدی ایجاد کند. محدودیت 1000 پردازش هم‌زمان جایگزین محدودیت حافظه یا تعداد Connection نیست.

**پیشنهاد:**
- محدودیت تعداد Connection
- محدودیت اندازه پیام متناسب با نیاز واقعی
- محدودیت حافظه/Byte Budget در صف‌ها، نه فقط تعداد پیام
- سیاست Backpressure
- محدودیت‌های جداگانه برای هر Client و کل Server

### F-18 — Priority و Circuit Breaker معنای واقعی مورد انتظار را ندارند

**شدت: Medium**

**مراجع:**
- ss7/connection.go:190-226
- ss7/connection_writer.go:51-93
- ss7/circuit_breaker.go:29-73

سه نکته:

1. همه پیام‌ها در یک sendCh مشترک قرار می‌گیرند. PriorityHigh فقط از بعضی Load Sheddingها عبور می‌کند و ترتیب ارسال را جلو نمی‌اندازد. بنابراین Priority واقعی وجود ندارد.
2. CircuitBreaker.RecordSuccess پس از Enqueue موفق اجرا می‌شود، نه پس از نوشتن موفق روی Socket.
3. خطاهای واقعی Writer در مسیر ثبت Failure در Circuit Breaker وارد نمی‌شوند.

**اثر:** نام Priority و Circuit Breaker ممکن است تضمینی بیش از رفتار واقعی سیستم القا کنند.

**پیشنهاد:** یا semantics این قابلیت‌ها ساده و دقیق مستند شوند، یا صف‌های اولویت‌دار و Feedback واقعی از Writer طراحی شود. اگر High Priority باید latency کمتری داشته باشد، یک FIFO مشترک کافی نیست.

### F-19 — Metricهای Queue و Connection Cleanup دقیق نیستند

**شدت: Medium**

**مراجع:** ss7/server.go:179-229, 518-544 و ss7/connection.go:214-217

- Gauge مربوط به Queue پس از Enqueue به‌روزرسانی می‌شود اما هنگام Dequeue توسط Writer به‌روزرسانی نمی‌شود؛ در نتیجه مقدار آن الزاماً Current Queue Depth نیست.
- هنگام Remove، فقط Label مربوط به queueDepth حذف می‌شود.
- Labelهای دارای Client ID در Histogram و Counterهای دیگر Cleanup نمی‌شوند.
- اگر Client ID آزادانه انتخاب شود، Connection Churn می‌تواند Cardinality متریک‌ها را افزایش دهد.
- Gauge تعداد اتصال‌ها نیز در مسیرهای Server-Initiated Close ممکن است کاهش پیدا نکند.

**پیشنهاد:** Lifecycle متریک‌ها باید با Lifecycle Connection هماهنگ شود. برای Client ID نیز باید Cardinality Budget یا شناسه‌های کنترل‌شده در نظر گرفته شود.

### F-20 — StateManager تضمین Transition سخت‌گیرانه نمی‌دهد

**شدت: Medium**

**مرجع:** ss7/connection_state.go:45-71

در TransitionState، اگر Transition در جدول مجاز نباشد ولی State فعلی برابر مقدار from باشد، Error برگردانده نمی‌شود و Transition پذیرفته می‌شود.

در نتیجه جدول validTransitions در این حالت محدودیت سخت‌گیرانه‌ای ایجاد نمی‌کند.

همچنین SetState بدون اعتبارسنجی Transition عمل می‌کند.

**پیشنهاد:** State Machine باید تنها یک قاعده روشن داشته باشد: State فعلی باید برابر from و زوج Transition نیز باید در فهرست مجاز باشد. مسیرهای اضطراری و Terminal State نیز باید صریح و محدود باشند.

### F-21 — Buffer Pool می‌تواند Buffer بزرگ را نگه دارد

**شدت: Low تا Medium**

**مرجع:** ss7/buffer_pool.go:5-20

PutBuffer فقط Slice را به طول صفر Reslice می‌کند و محدودیتی روی Capacity ندارد.

اگر Buffer در اثر پیام بزرگ رشد کند، همان backing array می‌تواند وارد sync.Pool شود و تا زمان جمع‌آوری Garbage یا استفاده مجدد حافظه زیادی نگه دارد.

**پیشنهاد:** فقط Bufferهای زیر یک سقف Capacity مشخص به Pool بازگردانده شوند. برای داده‌های حساس نیز سیاست پاک‌سازی حافظه بررسی شود.

### F-22 — تبدیل عددی بدون Range Validation

**شدت: Medium**

**مراجع:** ss7/server.go:622-637 و ss7/handlers.go:114-128

مقادیر Protobuf به انواع کوچک‌تر تبدیل می‌شوند؛ از جمله:
- DataCoding به uint8
- SarMsgRefNum به uint16
- SarTotalSegments و SarSegmentSeqnum به uint8

بدون بررسی Range، مقادیر خارج از محدوده می‌توانند Truncate شوند و مقدار دیگری در MessageInfo ایجاد کنند.

**پیشنهاد:** قبل از تبدیل، محدوده معتبر هر فیلد براساس قرارداد پروتکل بررسی شود.

### F-23 — احتمال ثبت داده حساس در Log

**شدت: نیازمند بررسی Logger و سیاست داده**

**مراجع:** ss7/handlers.go در Handlerهای SRI، MT و Status Report و بخش Build پیام‌ها

در کد، کل Protobuf به logger.InfoProto داده می‌شود. این پیام‌ها می‌توانند شامل شماره‌ها، IMSI، آدرس‌ها و محتوای پیام باشند.

پیاده‌سازی Logger در آرشیو وجود ندارد؛ بنابراین نمی‌توان تأیید کرد که خروجی آن همه فیلدها را چاپ می‌کند یا Redaction انجام می‌دهد.

**پیشنهاد:** پیاده‌سازی InfoProto و سیاست Redaction باید بررسی شود. محتوای SMS و شناسه‌های مشترک نباید بدون تصمیم امنیتی وارد Log عملیاتی شوند.

---

## 5. نکات مثبت طراحی

موارد زیر ارزش حفظ کردن دارند:

1. تفکیک Reader و Writer باعث می‌شود نوشتن هم‌زمان از چند مسیر روی TCP Connection اتفاق نیفتد.
2. استفاده از io.ReadFull برای خواندن Header و Body با رفتار Stream-Based TCP سازگار است و فرض نمی‌کند هر Read یک فریم کامل برگرداند.
3. برای طول فریم ورودی سقف تعیین شده است.
4. صف‌های ارسال و دریافت ظرفیت محدود دارند؛ هرچند سقف حافظه کل Server همچنان مشخص نشده است.
5. Connection.Close از sync.Once استفاده می‌کند.
6. برای بخشی از Stateها و Counterهای هم‌زمان از atomic و Mutex استفاده شده است.
7. Write Deadline و Read Deadline در مسیر شبکه در نظر گرفته شده‌اند.
8. Writer قابلیت Batch و Flush دوره‌ای دارد.
9. کد تلاش کرده Rate Limit، Deduplication، KeepAlive و Circuit Breaker را به Componentهای جدا تقسیم کند.

هدف بازطراحی نباید حذف بی‌دلیل این تفکیک‌ها باشد؛ باید قرارداد و Lifecycle آن‌ها اصلاح شود.

---

## 6. برنامه پیشنهادی اصلاحات

ترتیب پیشنهادی من:

### مرحله A — Security و Protocol Correctness

- تعیین مدل احراز هویت و Trust Boundary
- محدود کردن Commandها براساس State اتصال
- اعتبارسنجی تمام nested fieldهای Protobuf
- تعریف رفتار Command ناشناخته و پیام نامعتبر
- تعریف رفتار Unbind و KeepAlive
- مشخص کردن سیاست Sequence Number و Deduplication

### مرحله B — Lifecycle و Concurrency

- بازطراحی Start/Stop و مالکیت Listener
- Registry تمام Connectionهای پذیرفته‌شده
- WaitGroup/Supervisor واقعی برای Goroutineها
- حذف Goroutine-per-message نامحدود
- حفظ ترتیب پردازش در صورت نیاز قرارداد
- رفع Race در Bind و انتشار Client ID
- اصلاح Reserve/Register/Remove شناسه‌ها

### مرحله C — Message Delivery Correctness

- اصلاح Pending Request Lifecycle
- Sequence Generator با Collision Handling
- Rollback Pending در Send Failure
- تضمین یک نتیجه نهایی برای هر Request
- تکمیل Callback برای Build Failure
- تعریف رفتار Load Shedding و Reject

### مرحله D — Capacity و Observability

- Connection Limit
- Global و Per-Client Byte Budget
- بازبینی Queue Size و Max Frame Size
- اصلاح Metricهای Queue Depth و Cleanup
- بازبینی Cardinality
- اصلاح Circuit Breaker و Priority
- محدود کردن Buffer Pool

---

## 7. تست‌هایی که قبل از تأیید نسخه اصلاح‌شده لازم هستند

| شناسه | سناریو | انتظار |
|---|---|---|
| T-01 | Stop در حالی که Accept مسدود است | Start خارج شود و Listener بسته شود |
| T-02 | Stop با Connection هنوز Bind نشده | تمام Socketها بسته شوند |
| T-03 | Stop با صف ارسال پر | زمان Shutdown محدود و قابل پیش‌بینی باشد |
| T-04 | قطع Connection توسط KeepAlive | Registry و تمام Gaugeها پاک شوند |
| T-05 | دو Bind هم‌زمان با یک ID | دقیقاً یک Connection پذیرفته شود |
| T-06 | Disconnect اتصال قدیمی پس از Bind اتصال جدید | اتصال جدید از Registry حذف نشود |
| T-07 | ارسال Protobuf خالی و ناقص | Process Panic نکند و ورودی Reject شود |
| T-08 | Submit پیش از Bind | فرمان رد شود |
| T-09 | Unbind | پاسخ ارسال و Session طبق قرارداد بسته شود |
| T-10 | Queue Full | رفتار Reject/Backpressure مشخص و Metric ثبت شود |
| T-11 | Send Failure بعد از ثبت Pending | Callback دقیقاً یک بار اجرا شود |
| T-12 | Sequence Collision | Pending Request قبلی از بین نرود |
| T-13 | Response با Payload نامعتبر | وضعیت Request به شکل کنترل‌شده نهایی شود |
| T-14 | پیام‌های پشت سر هم یک Client | ترتیب مورد انتظار حفظ شود |
| T-15 | Configuration با Interval صفر یا منفی | Constructor Error بدهد، نه Panic |
| T-16 | چند هزار Connection و پیام بزرگ | حافظه و تعداد Goroutineها در سقف تعیین‌شده بمانند |
| T-17 | Bind و Log هم‌زمان | go test -race بدون Race باشد |
| T-18 | تکرار Connect/Disconnect | Registry، Metric و Goroutine Leak ایجاد نشود |

تست‌های T-01 تا T-18 پیشنهاد تست هستند و در این مرحله اجرا نشده‌اند.

---

## 8. نتیجه نهایی

**وضعیت فعلی: NOT READY FOR PRODUCTION SIGN-OFF**

این نتیجه به معنی نامناسب بودن کل معماری نیست. تفکیک Componentها و استفاده از Reader/Writer مستقل پایه مناسبی ایجاد کرده است؛ اما مشکلات Lifecycle، احراز هویت، State Machine، Validation و Concurrency باید پیش از اتکا به رفتار Server اصلاح شوند.

اولویت من شروع با بازنویسی کامل نیست. بهتر است ابتدا قراردادهای Lifecycle و Protocol را روشن کنیم، سپس اصلاحات کوچک و قابل تست را در چند مرحله انجام دهیم.

**هیچ کد برنامه‌ای در این بازبینی تغییر نکرده است.**