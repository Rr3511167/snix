# snix — راهنمای کامل از صفر تا صد

<div dir="rtl">

این راهنمای تفصیلی و بلند است. اگر اولین بار است که به ابزارهای دور زدن
سانسور دست می‌زنید، از همین جا شروع کنید. اگر کار را بلد هستید، نسخهٔ
کوتاه‌تر در [README](README.md) و نسخهٔ انگلیسیِ این راهنما در
[GUIDE.md](GUIDE.md) آمده.

هر چیزی که لازم دارید تا از **هیچ چیز روی کامپیوترتان** به **یک سامانهٔ
دور زدنِ کامل، سخت‌شده و قابل استفادهٔ روزمره** برسید در همین یک فایل است.

## فهرست

۱. [این راهنما چه چیزی را پوشش می‌دهد](#۱-این-راهنما-چه-چیزی-را-پوشش-میدهد)
۲. [مدلِ ذهنی: اجزا چه هستند؟](#۲-مدلِ-ذهنی-اجزا-چه-هستند)
۳. [تصمیم‌ها](#۳-تصمیمها)
۴. [بخشِ A — تهیهٔ سرورِ پروکسی](#۴-بخشِ-a--تهیهٔ-سرورِ-پروکسی)
   - [A1. Cloudflare Worker (رایگان و آسان‌ترین)](#a1-cloudflare-worker-رایگان-و-آسانترین)
   - [A2. سرورِ VPS (پولی، انعطاف‌پذیر)](#a2-سرورِ-vps-پولی-انعطافپذیر)
   - [A3. سروری که کسی به شما داده](#a3-سروری-که-کسی-به-شما-داده)
۵. [بخشِ B — اختیاری: دامنهٔ اختصاصی](#۵-بخشِ-b--اختیاری-دامنهٔ-اختصاصی)
۶. [بخشِ C — نصب و پیکربندیِ کلاینتِ پروکسی](#۶-بخشِ-c--نصب-و-پیکربندیِ-کلاینتِ-پروکسی)
۷. [بخشِ D — نصبِ snix](#۷-بخشِ-d--نصبِ-snix)
۸. [بخشِ E — دستیارِ راه‌اندازیِ اول](#۸-بخشِ-e--دستیارِ-راهاندازیِ-اول)
۹. [بخشِ F — شروعِ استفاده](#۹-بخشِ-f--شروعِ-استفاده)
۱۰. [بخشِ G — سخت‌سازی در برابرِ DPIِ قوی‌تر](#۱۰-بخشِ-g--سختسازی-در-برابرِ-dpiِ-قویتر)
۱۱. [بخشِ H — عملیاتِ روزمره](#۱۱-بخشِ-h--عملیاتِ-روزمره)
۱۲. [عیب‌یابی بر اساسِ علامت](#۱۲-عیبیابی-بر-اساسِ-علامت)
۱۳. [پیوستِ A — واژه‌نامه](#۱۳-پیوستِ-a--واژهنامه)
۱۴. [پیوستِ B — بهداشتِ امنیتی](#۱۴-پیوستِ-b--بهداشتِ-امنیتی)
۱۵. [پیوستِ C — حذفِ تمیز](#۱۵-پیوستِ-c--حذفِ-تمیز)

---

## ۱. این راهنما چه چیزی را پوشش می‌دهد

پس از این راهنما خواهید داشت:

- یک سرورِ پروکسی که در جایی بیرون از شبکهٔ سانسور‌شده اجرا می‌شود.
- یک کلاینتِ پروکسی (Xray یا sing-box) نصب و پیکربندی‌شده.
- snix که نصب و پیکربندی شده و با ضد-اثرانگشت روشن اجرا می‌شود.
- یک اتصالِ HTTPSِ کاری از طریقِ هر دو، که با سایتِ آزمایشی تأیید شده.
- دانشِ نگهداری، به‌روزرسانی و عیب‌یابیِ سامانه‌تان.

**زمانِ کل: ۱۵ تا ۴۵ دقیقه** بسته به اینکه کدام مسیر را انتخاب کنید.

**پلتفرم‌هایِ پوشش‌داده‌شده:** لینوکس (هر توزیعِ بزرگ) و ویندوزِ ۱۰/۱۱.
اندروید در برنامه است ولی هنوز ارسال نشده. macOSِ کلاینت (اسکنر و CLI) کار
می‌کند ولی موتورِ دور زدن هنوز در حالِ توسعه است.

**قبل از شروع چه چیزی نیاز دارید:**

- یک کامپیوتر برای اجرایِ snix (لینوکس یا ویندوز).
- یک گوشی یا دستگاهِ دیگر با دیتایِ موبایل، یا دسترسی به شبکهٔ باز (مثلاً
  wifiِ مهمان، هات‌اسپاتِ دوست) — این را برایِ ثبتِ اولیه در Cloudflare
  نیاز دارید، چون تنها قسمتِ دشوار وقتی است که پشتِ DPI هستید.
- حدودِ ۱۵ دقیقه توجهِ متمرکز. دستیارها طوری نوشته شده‌اند که هیچ گامی
  بداهه نباشد.

**چه چیزی نیاز ندارید:**

- VPN یا VPSِ پولی (مگر اینکه بخواهید — مسیرِ A2).
- دانشِ برنامه‌نویسی.
- دامنهٔ اختصاصی (اختیاری است — بخشِ B).
- تجربهٔ لینوکس فراتر از اجرای `sudo` و چسباندنِ دستور.

---

## ۲. مدلِ ذهنی: اجزا چه هستند؟

قبل از شروع، بدانید چطور قطعات با هم کار می‌کنند.

```
[دستگاهِ شما، طرفِ سانسور]                      [اینترنتِ باز]

  مرورگر / اپ                                   سرورِ پروکسیِ شما
     │                                          (Cloudflare Worker
     │                                           یا VPS)
     ▼                                              ▲
  کلاینتِ پروکسی                                    │
  (Xray / sing-box)                                 │
     │                                              │
     │  outbound → 127.0.0.1:40443                  │
     ▼                                              │
  snix روی 127.0.0.1:40443 ─── اینترنت + DPI ───────┘
  (به هر اتصالی که خارج می‌رود
   ترفندِ SNI-spoof را اضافه می‌کند)
```

چهار قطعهٔ در حرکت. سه تا روی دستگاهِ شما:

۱. **سرورِ پروکسی** — روی اینترنت زندگی می‌کند. روی کامپیوترِ خودتان
   نصب نمی‌کنید؛ یا اجاره می‌کنید یا روی زیرساختِ دیگری (Cloudflare)
   مستقر می‌کنید. چیزی مثلِ VLESS / VMess / Trojan اجرا می‌کند.
۲. **کلاینتِ پروکسی** — روی دستگاهِ شما. Xray / sing-box / NekoBox —
   برنامهٔ محلی که پروتکلِ پروکسی را با سرور صحبت می‌کند.
۳. **snix** — روی دستگاهِ شما. بینِ کلاینتِ پروکسی و اینترنت می‌نشیند،
   تا وقتی کلاینت می‌خواهد به سرورِ پروکسی برسد، ترفندِ SNI-spoof در
   دست‌دادنِ TLS اعمال شود.
۴. **اپ‌هایِ شما** (مرورگر و …) — به کلاینتِ پروکسی وصل می‌شوند.

**چرا snix به تنهایی VPN نیست؟** چون هیچ پروتکلِ پروکسی‌ای صحبت نمی‌کند.
فقط DPI را گول می‌زند. باز هم به یک پروتکلِ پروکسی (که سرور + کلاینت
فراهم می‌کنند) نیاز دارید تا واقعاً داده به بیرون و داخل جابه‌جا شود.

---

## ۳. تصمیم‌ها

سه تصمیمِ بزرگ.

### تصمیمِ ۱ — سرورِ رایگان یا پولی؟

|                | **رایگان (Cloudflare Worker)**              | **پولی (VPS)**                      |
|---|---|---|
| هزینهٔ اولیه     | ۰ تومان                                   | معمولاً ۳ تا ۱۰ دلار در ماه             |
| زمانِ راه‌اندازی | ۵ دقیقه در مرورگر                          | ۱۵ تا ۳۰ دقیقه، یک‌بار                  |
| خوب برایِ       | مرور، پیام‌رسان، ویدیوی سبک                | ویدیویِ سنگین، torrent، چند دستگاه     |
| سرعت            | معمولاً ۱۵ تا ۵۰ مگابیت/ثانیه              | هر چه VPS بدهد                         |
| سقفِ پهنای‌باند | ۱۰۰هزار درخواستِ HTTP در روز (برای یک نفر کافی) | معمولاً نامحدود                   |
| مسدودیِ سخت    | خیلی سخت (IP مشترک با میلیون‌ها سایتِ CF)    | متوسط (هر سرور یک IP)                  |
| مهارتِ لازم     | هیچ                                       | چسباندنِ دستور در SSH                  |

**توصیه: با Cloudflare Worker شروع کنید (مسیرِ A1).** بعداً می‌توانید
به VPS ارتقا دهید. خودِ مراحلِ snix یکسان است.

### تصمیمِ ۲ — دامنه؟

آیا یک URLِ قشنگ مثلِ `proxy.yourname.com` می‌خواهید یا
`snix-ab12cd.yourname.workers.dev` هم اوکی است؟

- **نه**: بخشِ B را رد کنید. همه چیز بدونِ دامنه هم کار می‌کند.
- **بله**: بخشِ B شما را هدایت می‌کند. ۱۰ دقیقهٔ بیشتر + سالی ۱۰ دلار.

اکثرِ کاربران: **تا وقتی چیزی کار کند دامنه را رد کنید**. بعداً در ۵
دقیقه اضافه می‌شود.

### تصمیمِ ۳ — کدام کلاینتِ پروکسی؟

snix با هر چیزی که VLESS / VMess / Trojan / Shadowsocks صحبت کند کار
می‌کند.

| کلاینت     | پلتفرم‌ها           | خوب برایِ                                 |
|---|---|---|
| **Xray**   | لینوکس، ویندوز      | منعطف‌ترین؛ توصیهٔ پیش‌فرضِ ما           |
| sing-box   | لینوکس، ویندوز      | جدیدتر، پیکربندیِ تمیزتر                  |
| v2rayN     | فقط ویندوز          | GUI دارد — اگر ترمینال دوست ندارید        |
| NekoBox    | فقط اندروید         | موبایل؛ هنوز پشتیبانی نمی‌کنیم            |

**توصیه: Xray.** قالبِ Cloudflare Workerِ ما برایِ VLESS طراحی شده که
Xray به صورتِ بومی پشتیبانی می‌کند.

---

## ۴. بخشِ A — تهیهٔ سرورِ پروکسی

**یکی** از A1 یا A2 یا A3 را انتخاب کنید.

### A1. Cloudflare Worker (رایگان و آسان‌ترین)

یک قطعهٔ کدِ کوچک را به پلتفرمِ رایگانِ «Workers» در Cloudflare مستقر
می‌کنید. نقشِ سرورِ پروکسی را ایفا می‌کند. بدونِ سرور، بدونِ هزینه،
Cloudflare TLS و محافظت در برابرِ DDoS را رایگان انجام می‌دهد.

#### A1.1 یک حسابِ Cloudflare بسازید

برایِ این گام ممکن است به دستگاهی روی **شبکهٔ باز** نیاز داشته باشید،
چون بعضی ISPها Cloudflare را تا حدی بلاک می‌کنند. گزینه‌ها:

- دیتایِ موبایل
- wifiِ دوست یا هات‌اسپات
- یک VPNِ موجود (غیر از snix)

اگر هیچ شبکهٔ جایگزینی ندارید، به مسیرِ A2 (VPS) بروید که به Cloudflare
نیاز ندارد.

۱. <https://dash.cloudflare.com/sign-up> را در مرورگر باز کنید.
۲. با ایمیل و رمز ثبتِ نام کنید. ایمیل را تأیید کنید.
۳. فعلاً تمام. **نیازی به افزودنِ دامنه یا کارتِ پرداخت نیست.** سطحِ
   رایگان واقعاً رایگان است.

#### A1.2 یک UUID برایِ احرازِ هویت تولید کنید

Workerِ شما فقط ترافیکی که UUIDِ مخفی را بداند می‌پذیرد. الان تولید
کنید و امن نگه دارید.

یکی از این روش‌ها:

- <https://www.uuidgenerator.net/version4> را باز کنید و UUIDِ نمایش‌داده‌شده را کپی کنید.
- در لینوکس: `cat /proc/sys/kernel/random/uuid`.
- در PowerShellِ ویندوز: `[guid]::NewGuid().ToString()`.
- snix هم در دستیار یکی تولید می‌کند — اگر ترجیح می‌دهید، این گام را رد
  کنید و از UUIDِ دستیار استفاده کنید.

نمونه (UUIDِ شما متفاوت خواهد بود):
```
f47ac10b-58cc-4372-a567-0e02b2c3d479
```

**این را در جایی ذخیره کنید.** لحظاتی بعد باید در Cloudflare بگذارید و
بعد در کلاینتِ پروکسی. مثلِ رمز با آن رفتار کنید — هرکس این UUID را
داشته باشد می‌تواند از Workerِ شما استفاده کند.

#### A1.3 Worker را مستقر کنید

۱. در <https://dash.cloudflare.com/> وارد شوید.
۲. در نوارِ کناریِ چپ، **Workers & Pages** را کلیک کنید.
۳. **Create application → Create Worker** را کلیک کنید.
۴. نامش را مثلاً `snix` بگذارید (بعداً قابلِ تغییر است). **Deploy** را
   بزنید. Cloudflare شما را به یک Workerِ پیش‌فرضِ «hello world» می‌برد.
۵. در بالایِ صفحه **Edit code** را کلیک کنید.
۶. در repoیِ snix، فایلِ [`cfworker/worker.js`](cfworker/worker.js) را
   باز کنید. تمامِ محتوا را کپی کنید.
۷. در ویرایشگرِ Cloudflare، همه چیز را پاک کنید و محتوایِ `worker.js` را
   جای‌گذاری کنید.
۸. **Save and deploy** را بزنید.
۹. حالا UUID را به عنوانِ متغیرِ محیطی اضافه کنید:
   - **← Back to service** (بالا، چپ).
   - تبِ **Settings**.
   - **Variables and Secrets**.
   - **+ Add**.
   - نام: `UUID`، مقدار: UUIDِ شما از گامِ A1.2، نوع: **Plaintext**.
   - **Save and deploy**.
۱۰. بالایِ صفحهٔ Worker یک URL با پسوندِ `.workers.dev` می‌بینید. چیزی
    شبیهِ `snix.yourname.workers.dev`. کپی کنید.

حالا یک سرورِ پروکسیِ کاری دارید. تأیید کنید با بازکردنِ URL در مرورگر —
باید صفحهٔ «snix worker» را ببینید.

#### A1.4 مشخصاتِ خود را یادداشت کنید

اکنون برایِ تکمیلِ بقیهٔ مراحل به این سه چیز نیاز دارید. بنویسید (روی
کامپیوترِ اصلیِ خود نگذارید اگر تحتِ نظارت است):

- **Worker host**: `snix.yourname.workers.dev`
- **UUID**: همان که در A1.2 گرفتید
- **Port**: `443` (همیشه، برایِ Workers)
- **Protocol**: VLESS روی WebSocket با TLS، path: `/?ed=2048`

به [بخشِ B](#۵-بخشِ-b--اختیاری-دامنهٔ-اختصاصی) (اختیاری) یا
[بخشِ C](#۶-بخشِ-c--نصب-و-پیکربندیِ-کلاینتِ-پروکسی) بروید.

---

### A2. سرورِ VPS (پولی، انعطاف‌پذیر)

اگر از قبل یک VPS لینوکسی با IPِ عمومی دارید یا می‌خواهید بخرید، این
مسیر سرورِ منعطف‌تری بدونِ محدودیت‌هایِ Worker می‌دهد.

#### A2.1 یک VPS بگیرید

ارائه‌دهندگانِ ارزانِ VPS لینوکسی:
- **RackNerd** — گاهی ۱۰ تا ۱۵ دلار در سال. آمریکا / اروپا / آسیا.
- **BuyVM** — ۳.۵ دلار در ماه، شبکهٔ خوب.
- **Vultr** — ۵ دلار در ماه، جهانی.
- **Hetzner** — ۴ یورو در ماه، اروپا.

هنگامِ سفارش، انتخاب کنید:
- OS: **Ubuntu 22.04 LTS** (این راهنما همین را فرض می‌کند).
- معماری: **amd64**.
- محل: جایی که از ISPِ شما مسدود نباشد. پیش‌فرضِ خوب: آلمان یا هلند.

چیزهایی که می‌گیرید:
- آدرسِ IP (بنویسید).
- رمزِ SSH (یا کلیدِ SSH که آپلود کرده‌اید).

#### A2.2 به VPS از طریقِ SSH متصل شوید

از لینوکس / macOS / PowerShellِ ویندوز:

```bash
ssh root@YOUR.VPS.IP
```

رمز را وارد کنید. prompt شبیهِ `root@vps:~#` می‌بینید.

**فوراً رمز را به یک رمزِ قوی تغییر دهید**:
```bash
passwd
```
دو بار رمزِ جدید را بزنید. فراموش نکنید.

#### A2.3 Xray را به عنوانِ سرورِ VLESS نصب کنید

Xray را روی VPS نصب می‌کنیم تا ترافیکِ پروکسی را روی پورتِ ۴۴۳ بپذیرد.

```bash
# 1. Xray را از طریقِ تک‌خطیِ رسمی نصب کنید.
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install

# 2. یک UUID تولید کنید.
xray uuid
# (UUIDِ چاپ‌شده را کپی کنید؛ پایین می‌چسبانید)

# 3. پیکربندیِ Xray را باز کنید.
nano /usr/local/etc/xray/config.json
```

محتوای فایل را با این جایگزین کنید (UUIDِ خود را بچسبانید):

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "port": 443,
      "protocol": "vless",
      "settings": {
        "clients": [
          { "id": "PASTE-YOUR-UUID-HERE", "flow": "" }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "certificates": [
            {
              "certificateFile": "/etc/xray/fullchain.pem",
              "keyFile":         "/etc/xray/privkey.pem"
            }
          ]
        }
      }
    }
  ],
  "outbounds": [ { "protocol": "freedom" } ]
}
```

ذخیره: Ctrl+O، Enter، Ctrl+X.

به گواهیِ TLS نیاز دارید. دو گزینه:

**گزینهٔ A (توصیه‌شده)** — دامنه‌ای که مالکِ آن هستید:
[بخشِ B](#۵-بخشِ-b--اختیاری-دامنهٔ-اختصاصی) را ببینید برایِ خریدِ دامنه
و اتصالِ آن. سپس روی VPS:
```bash
apt update && apt install -y certbot
certbot certonly --standalone -d proxy.yourname.com
mkdir -p /etc/xray
cp /etc/letsencrypt/live/proxy.yourname.com/fullchain.pem  /etc/xray/
cp /etc/letsencrypt/live/proxy.yourname.com/privkey.pem    /etc/xray/
chown -R nobody:nogroup /etc/xray
chmod 600 /etc/xray/privkey.pem
```

**گزینهٔ B** — گواهیِ self-signed (زشت‌تر ولی کار می‌کند):
```bash
mkdir -p /etc/xray
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout /etc/xray/privkey.pem \
  -out    /etc/xray/fullchain.pem \
  -subj "/CN=your-vps-ip" -days 3650
```
کلاینتِ شما بعداً باید TLSِ «ناامن» را اجازه دهد (بعداً می‌بینیم).

Xray را استارت کنید:
```bash
systemctl restart xray
systemctl enable  xray
systemctl status  xray       # باید بگوید "active (running)"
```

اگر فایروال فعال است، پورتِ ۴۴۳ را باز کنید:
```bash
ufw allow 443/tcp
```

یادداشت کنید:
- **Server host**: IPِ VPSِ شما (یا دامنه اگر گزینهٔ A را رفتید).
- **UUID**: همان که تولید کردید.
- **Port**: ۴۴۳.
- **Protocol**: VLESS روی TCP با TLS.

به [بخشِ C](#۶-بخشِ-c--نصب-و-پیکربندیِ-کلاینتِ-پروکسی) بروید.

---

### A3. سروری که کسی به شما داده

اگر دوست یا سرویسِ پولی از قبل یک پیکربندیِ پروکسی به شما داده، از
راه‌اندازیِ سرور رد شوید. باید به شما گفته باشند:

- هاستِ سرور (دامنه یا IP)
- پورت (معمولاً ۴۴۳)
- پروتکل (VLESS / VMess / Trojan / …)
- UUID یا رمز
- ترنسپورت (TCP / WebSocket / gRPC)
- تنظیماتِ TLS (SNI، اینکه تأییدِ گواهی را رد کنیم یا نه)

بنویسید و به [بخشِ C](#۶-بخشِ-c--نصب-و-پیکربندیِ-کلاینتِ-پروکسی) بروید.

---

## ۵. بخشِ B — اختیاری: دامنهٔ اختصاصی

اگر `*.workers.dev` یا صرفاً IPِ VPS راضی‌تان می‌کند، این بخش را رد کنید.
اگر بعداً URLِ قشنگ‌تر خواستید، برگردید.

### B1. دامنه بخرید

ثبت‌کنندگانِ ارزان (همه A/CNAME پشتیبانی می‌کنند):

- **Cloudflare Registrar** — به قیمتِ هزینه، حدودِ ۹ دلار در سال برای `.com`.
- **Porkbun** — ۹ تا ۱۲ دلار در سال، UIِ تمیز.
- **Namecheap** — شناخته‌شده، کمی گران‌تر.

هر دامنه‌ای. کوتاه لازم نیست. `my-proxy-2026.com` هم خوب است.

### B2. دامنه را به Workerِ Cloudflare وصل کنید

(اگر مسیرِ VPS را می‌روید، به [B3](#b3-دامنه-را-به-vps-وصل-کنید) بروید.)

رویکردِ مدرنِ Cloudflare DNS + TLS + مسیردهی را یک‌جا انجام می‌دهد. نه
Workers Routes، نه AAAAِ ترفندی.

۱. اگر دامنه را از **Cloudflare Registrar** خریدید، از قبل روی Cloudflare
   DNS است. به گامِ ۴ بروید.
۲. وگرنه، در داشبوردِ Cloudflare، **Add a site** را کلیک کنید، دامنه
   را بچسبانید، **Free plan** را انتخاب کنید و ادامه دهید.
۳. Cloudflare دو nameserver می‌دهد (`aaron.ns.cloudflare.com` و …).
   به ثبت‌کنندهٔ خود بروید، nameserverها را به این دو تغییر دهید. ۵ تا ۶۰
   دقیقه منتظرِ انتشار.
۴. در Cloudflare، **Workers & Pages → Workerِ snix** را باز کنید.
۵. تبِ **Settings** سپس **Triggers** در چپ.
۶. زیرِ **Custom Domains**، **Add Custom Domain** را کلیک کنید.
۷. `proxy.yourname.com` را وارد کنید و **Add Custom Domain** را بزنید.
۸. Cloudflare به صورتِ خودکار رکوردِ DNS را می‌سازد و گواهیِ TLS را فراهم
   می‌کند. ۳۰ تا ۶۰ ثانیه طول می‌کشد.

تأیید:
```bash
curl -sI https://proxy.yourname.com/
```
باید `200 OK` و header هایِ صفحهٔ «snix worker» را ببینید.

از این پس هرجا `xxx.workers.dev` می‌بینید، `proxy.yourname.com` را
به‌جایش بگذارید (در پیکربندیِ snix، در پیکربندیِ Xray، در prompt هایِ
دستیار).

### B3. دامنه را به VPS وصل کنید

(اگر مسیرِ Worker را می‌روید، این را رد کنید.)

۱. در Cloudflare DNS (یا DNSِ ثبت‌کنندهٔ خود)، یک **A record** اضافه کنید:
   - نام: `proxy`
   - آدرسِ IPv4: IPِ VPSِ شما
   - Proxy status: **DNS only** (ابرِ خاکستری). چون خودِ VPS TLS را
     ترمینیت می‌کند.
۲. ذخیره. چند دقیقه منتظر.
۳. `curl -k https://proxy.yourname.com` باید به VPS برسد.
۴. روی VPS، `certbot` را اجرا کنید تا گواهیِ واقعی بگیرید (A2.3 گزینهٔ A).

از این پس به‌جایِ IPِ VPS از `proxy.yourname.com` استفاده کنید.

---

## ۶. بخشِ C — نصب و پیکربندیِ کلاینتِ پروکسی

**یک** کلاینت لازم دارید (Xray در این راهنما). روی دستگاهِ محلیِ خود نصب
کنید، نه VPS.

**دو مرحله:**
۱. **نصبِ** باینریِ Xray (C1 / C2 / C3 پایین).
۲. **پیکربندیِ** آن با مشخصاتِ سرور تا Xray بداند چطور با Worker / VPS
   حرف بزند (C4 پایین — قالب‌هایِ آمادهٔ چسباندن دارد).

بعداً در [بخشِ E](#۸-بخشِ-e--دستیارِ-راهاندازیِ-اول)، دستیارِ snix فقط
فیلدهایِ `address` و `port` را در این پیکربندی بازنویسی می‌کند تا به
listenerِ محلیِ snix اشاره کنند. بقیه (UUID، TLS، WebSocket path) همان
طور که اینجا می‌گذارید باقی می‌ماند.

### C1. نصب روی لینوکس

```bash
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
```

پس از نصب:
- باینری: `/usr/local/bin/xray`
- پیکربندی: `/usr/local/etc/xray/config.json`
- سرویسِ systemd: `xray.service`

### C2. نصب روی ویندوز

دو گزینه:

- **v2rayN** (GUI، آسان‌تر): از
  <https://github.com/2dust/v2rayN/releases> دانلود کنید. tray app است.
  پیکربندی در `%APPDATA%\v2rayN\` می‌نشیند.
- **Xray core** (CLI، سبک‌تر): از
  <https://github.com/XTLS/Xray-core/releases> دانلود کنید، مثلاً در
  `C:\Program Files\xray\` استخراج کنید و
  `xray.exe run -c config.json` اجرا کنید.

### C3. نصب روی macOS

```bash
brew install xray
```
پیکربندی در `~/.config/xray/config.json` می‌رود.

### C4. پیکربندیِ کلاینتِ Xray را بنویسید

قالبی را که با نحوهٔ راه‌اندازیِ سرور در بخشِ A می‌خواند انتخاب کنید.

هر دو قالب:
- یک inboundِ SOCKS5 محلی روی `127.0.0.1:1080` باز می‌کنند (آنچه مرورگر
  با آن صحبت می‌کند).
- تمامِ ترافیک را به عنوانِ VLESS به سرورتان می‌فرستند.
- در بخشِ E خودکار patch می‌شوند تا `address`/`port` به listenerِ محلیِ
  snix اشاره کند نه به سرورِ واقعی.

**قالبِ ۱ — Cloudflare Worker (مسیرِ A1)**

به عنوانِ `/usr/local/etc/xray/config.json` در لینوکس یا معادلش در OSِ
دیگر ذخیره کنید:

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 1080,
      "protocol": "socks",
      "settings": { "auth": "noauth", "udp": true, "ip": "127.0.0.1" }
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "snix.yourname.workers.dev",
            "port": 443,
            "users": [
              { "id": "PASTE-YOUR-UUID-HERE", "encryption": "none" }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls",
        "tlsSettings": {
          "serverName": "snix.yourname.workers.dev",
          "allowInsecure": false
        },
        "wsSettings": {
          "path": "/?ed=2048",
          "headers": { "Host": "snix.yourname.workers.dev" }
        }
      }
    }
  ]
}
```

جایگزین کنید:
- `snix.yourname.workers.dev` را با hostِ واقعیِ Workerِ خود (یا دامنهٔ
  سفارشیِ بخشِ B، در هر سه جایی که قالب اشاره می‌کند).
- `PASTE-YOUR-UUID-HERE` را با UUIDِ A1.4.

**قالبِ ۲ — VPSِ شما (مسیرِ A2)**

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "listen": "127.0.0.1",
      "port": 1080,
      "protocol": "socks",
      "settings": { "auth": "noauth", "udp": true, "ip": "127.0.0.1" }
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "settings": {
        "vnext": [
          {
            "address": "proxy.yourname.com",
            "port": 443,
            "users": [
              { "id": "PASTE-YOUR-UUID-HERE", "encryption": "none" }
            ]
          }
        ]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
          "serverName": "proxy.yourname.com",
          "allowInsecure": false
        }
      }
    }
  ]
}
```

جایگزین کنید:
- `proxy.yourname.com` را با دامنهٔ VPS (یا IP اگر self-signed را در A2.3
  استفاده کردید — در آن حالت `"allowInsecure": true` هم بگذارید).
- `PASTE-YOUR-UUID-HERE` را با UUIDِ تولیدشده روی VPS.

### C5. Xray را استارت و تأیید کنید

لینوکس:
```bash
sudo systemctl restart xray
sudo systemctl enable  xray
sudo systemctl status  xray     # باید "active (running)" بگوید
ss -tlnp | grep 1080            # Xray باید روی SOCKS5 گوش دهد
```

ویندوز (v2rayN): **Servers → Add server → VLESS Import** و URLِ vless://
را بچسبانید، یا محتوایِ قالب را در
`%APPDATA%\v2rayN\config\config.json` بگذارید و **Restart service**
را کلیک کنید.

ویندوز (Xray-core CLI):
```cmd
"C:\Program Files\xray\xray.exe" run -c "C:\Program Files\xray\config.json"
```

### C6. آزمونِ دود (بدونِ snix)

این آزمون تأیید می‌کند سرور قابلِ دسترسی است و Xray می‌تواند حرف بزند.
هنوز snix نصب نشده، پس DPI هنوز سرِ راه است — اگر ISPِ شما SNIِ
cloudflare.com را مستقیم بلاک می‌کند این ناکام می‌شود، ولی بعد از
وارد شدنِ snix همین آزمون موفق می‌شود.

```bash
curl --socks5 127.0.0.1:1080 -sI https://www.cloudflare.com/ | head -3
```

اگر `HTTP/2 200` دیدید، Xray سالم است. اگر timeout یا «connection reset»
گرفتید، DPI فعال است — ادامهٔ راهنما را بروید. snix این را رفع می‌کند.

اگر خطای دیگری دیدید (شکستِ TLS، عدمِ تطابقِ UUID، و …) Xray را قبل از
ادامه درست کنید:
- لینوکس: `sudo journalctl -u xray -n 50` خطاهایِ Xray را نشان می‌دهد.
- اشتباهاتِ رایج: UUIDِ تایپی، SNIِ اشتباه، فراموش کردنِ `/?ed=2048`
  برایِ Worker.

---

## ۷. بخشِ D — نصبِ snix

روی همان دستگاهی که کلاینتِ پروکسی نصب کردید snix را هم نصب کنید.

### D1. لینوکس (تک‌خطی)

```bash
curl -fsSL https://get.snix.sh | sudo sh
```

امضا را تأیید می‌کند، `/usr/local/bin/snix` می‌گذارد، یک سرویسِ systemd
(پیش‌فرض غیرفعال) و auto-completionِ bash/zsh اضافه می‌کند.

### D2. لینوکس (دستی)

```bash
curl -LO https://github.com/SamNet-dev/snix/releases/latest/download/snix-linux-amd64.tar.gz
tar -xzf snix-linux-amd64.tar.gz
sudo install -m 0755 snix /usr/local/bin/snix
```

(به‌جایِ `amd64` از `arm64` یا `armv7` برایِ Raspberry Pi / arm
استفاده کنید.)

### D3. ویندوز

۱. `snix-setup.exe` را از
   <https://github.com/SamNet-dev/snix/releases> دانلود کنید.
۲. راست‌کلیک → **Run as Administrator**.
۳. مراحلِ نصب‌کننده را دنبال کنید.
۴. نصب‌کننده:
   - `snix.exe` + `WinDivert.dll` + `WinDivert64.sys` را در
     `C:\Program Files\snix\` می‌گذارد.
   - میانبرهایِ Start Menu برایِ **snix (TUI)** و **snix (wizard)**
     ایجاد می‌کند.
   - اختیاری: سرویسِ ویندوز نصب و استارت می‌کند.

### D4. ساختن از سورس (هر پلتفرمی)

```bash
git clone https://github.com/SamNet-dev/snix.git
cd snix
go build -o snix ./cmd/snix
```

Go نسخهٔ ۱.۲۳+ نیاز دارید. در ویندوز هم `WinDivert.dll` +
`WinDivert64.sys` را کنارِ exe می‌گذارید — از
<https://github.com/basil00/Divert/releases/tag/v2.2.2> (`x64`).

### D5. تأیید

```bash
snix --version      # "snix version X.Y.Z" را چاپ می‌کند
snix --help         # همهٔ دستورها را نشان می‌دهد
```

---

## ۸. بخشِ E — دستیارِ راه‌اندازیِ اول

اینجا همه چیز کنارِ هم می‌آید. دستیار حدودِ ۶۰ ثانیه طول می‌کشد.

### E1. دستیار را اجرا کنید

لینوکس:
```bash
sudo snix init --wizard
```

ویندوز (بالارفته):
```cmd
snix.exe init --wizard
```

می‌بینید:

```
snix first-run wizard
─────────────────────

Five short steps. Safe to Ctrl-C at any time; nothing is written
to disk until the end.
```

### E2. گامِ ۱ — آیا سرورِ پروکسی دارید؟

**`y`** بزنید و مشخصاتِ بخشِ A / B را بچسبانید. دستیار می‌پرسد:

```
  Host (domain or IP):   snix.yourname.workers.dev     <- یا proxy.yourname.com / IPِ VPS
  Port [443]:            443
```

### E3. گامِ ۳ — اسکنِ شبکه

```
Step 3/5 — Scanning your network for working SNIs and CDN IPs…
  ip probes:  (20 candidates)
  sni probes: (55 candidates against 1.1.1.1)
  (takes up to ~10 seconds)
  ✓ 47/55 SNI candidates OK, 18/20 IPs reachable
```

دستیار یاد گرفته ISPِ شما کدام SNIها را بلاک نمی‌کند و کدام IPهایِ CDN
قابلِ دسترسی‌اند. این‌ها خودکار در profileِ شما می‌روند.

### E4. گامِ ۴ — کلیدهایِ ضد-اثرانگشت

به هر سه **`y`** بگویید. DPI را بسیار سخت‌تر می‌کنند.

```
  Randomize inject timing?   (Y/n) y
  Randomize ClientHello size? (Y/n) y
  Rotate bypass strategies per flow? (Y/n) y
```

### E5. گامِ ۵ — ادغام با کلاینتِ پروکسی

```
Step 5/5 — Proxy-client integration
  Detected xray at /usr/local/bin/xray
  Update xray config to route through snix? (y/N) y
    done (backup: /usr/local/etc/xray/config.json.bak)
```

**مهم**: دستیار پیکربندیِ Xray را بازنویسی می‌کند تا outboundش به
`127.0.0.1:40443` (snix) برود نه مستقیم به سرورِ شما. snix ارسال به
سرورِ واقعی را انجام می‌دهد. پشتیبانِ پیکربندیِ اصلی همیشه اول ذخیره
می‌شود.

اگر گفت «No supported proxy client detected»، بعد از نصبِ Xray (بخشِ C)
به این گام برگردید.

### E6. چه چیزی نوشت

خروجیِ نهایی:
```
Config written to /root/.config/snix/config.yaml
Next:
  sudo snix start           # foreground
  sudo systemctl enable --now snix   # managed service (linux)
```

اگر کنجکاوید پیکربندی را ببینید:
```bash
cat /root/.config/snix/config.yaml
```

یک profileِ کامل می‌بینید با سرورِ شما، ۱۰ SNIِ برتر، ۴ IPِ پشتیبان، و
همهٔ کلیدهایِ ضد-اثرانگشت روشن.

---

## ۹. بخشِ F — شروعِ استفاده

### F1. snix را استارت کنید

لینوکس (foreground، برایِ آزمایش):
```bash
sudo snix start
```

می‌بینید:
```
snix: starting profile "default" listen=127.0.0.1:40443 connect=snix.yourname.workers.dev:443 strategy=wrong_seq
snix: local=10.0.0.5  remote=104.21.2.17:443
snix: iptables rules installed, NFQUEUE active
snix: bypass engine running
snix: listening on 127.0.0.1:40443
```

آن ترمینال را باز بگذارید.

لینوکس (سرویسِ مدیریت‌شده، برایِ استفادهٔ روزمره):
```bash
sudo systemctl enable --now snix
journalctl -u snix -f      # در ترمینالِ دیگر، لاگ‌ها را دنبال کنید
```

ویندوز (GUI): Start Menu → **snix (TUI)** (UAC خودکار).

### F2. Xray را استارت کنید

لینوکس:
```bash
systemctl restart xray
systemctl enable  xray
```

ویندوز: v2rayN را از tray باز کنید، دکمهٔ سبزِ start را بزنید.

### F3. مرورگر را اشاره دهید

در تنظیماتِ پروکسیِ مرورگر:
- نوع: **SOCKS5** (یا HTTP بسته به پیکربندیِ Xray).
- هاست: `127.0.0.1`
- پورت: `1080` (پیش‌فرضِ SOCKSِ Xray؛ در بخشِ `inbounds`ِ پیکربندی ببینید).
- بدونِ احرازِ هویت.

یا اکستنشنِ **SwitchyOmega** (کروم / فایرفاکس) نصب کنید و پروفایلی به
`127.0.0.1:1080` بسازید.

### F4. آزمایش کنید

با پروکسیِ فعال در مرورگر، به این سایت‌ها بروید:
- <https://www.youtube.com/>
- <https://twitter.com/>
- <https://chatgpt.com/>

هر سه باید لود شوند. اگر شدند — تبریک، دور زدنِ DPIِ سخت‌شده دارید.

با curl هم آزمایش کنید:
```bash
curl --socks5 127.0.0.1:1080 https://www.youtube.com/ -I
```
باید `HTTP/2 200` برگرداند.

لاگ‌هایِ snix را ببینید:
```bash
journalctl -u snix | grep injected
```

برایِ هر اتصالِ خروجی یک خط:
```
snix: injected wrong_seq sni=auth.vercel.com size=843 seq=0xab12... id_delta=42
snix: injected wrong_checksum sni=cdn.segment.io size=1102 seq=0xcd34... id_delta=18
```

SNIِ مختلف، اندازهٔ مختلف، استراتژیِ مختلف در هر بار. **یعنی
ضد-اثرانگشت کار می‌کند.**

---

## ۱۰. بخشِ G — سخت‌سازی در برابرِ DPIِ قوی‌تر

اگر DPIِ ISPِ شما خشن‌تر شد، کلیدهایِ بیشتری را بالا ببرید. تغییرات
فوراً ذخیره می‌شوند؛ در TUI نیازی به restart نیست.

### G1. TUI را باز کنید

```bash
sudo snix tui
```

کلیدِ **5** برای Settings.

### G2. تنظیماتِ پیشنهادیِ تولید

- **Randomize timing**: ON (min 500µs، max 5ms).
- **Randomize padding**: ON (max extra pad 600 bytes).
- **Strategy rotation**: ON.
- **IP ID delta range**: 64.
- **SNI selection**: random (نه round-robin).

### G3. به‌صورتِ دوره‌ای اسکن کنید

هر چند هفته، یا هر وقت چیزی از کار افتاد:
```bash
snix scan all
```

`sni_pool`ِ به‌روزشده را در پیکربندی بچسبانید، یا در TUI تبِ Scan را
استفاده کنید که می‌تواند مستقیماً در profileِ شما ذخیره کند.

### G4. profileهایِ چندگانه برایِ شبکه‌هایِ مختلف

می‌توانید یک profile برایِ هر شبکهٔ مورد استفاده داشته باشید. در TUI تبِ
Profiles، یک profileِ موجود را کپی کنید، `connect.host`ش را به سرورِ
متفاوتی تغییر دهید، و با Enter بینشان سوییچ کنید.

یا `~/.config/snix/config.yaml` را مستقیم ویرایش کنید — یک ورودیِ دیگر
زیرِ `profiles:` اضافه کنید.

---

## ۱۱. بخشِ H — عملیاتِ روزمره

### H1. به‌روزرسانیِ snix

```bash
snix update
```

آخرین نسخهٔ امضاشده را می‌گیرد، SHA-256 را تأیید می‌کند، به صورتِ
atomic باینری را جایگزین می‌کند. روی لینوکس و ویندوز کار می‌کند (در
ویندوز TUI یا installer باید بسته باشد).

### H2. ببینید چه خبر است

```bash
journalctl -u snix -f                # لاگِ سرویسِ لینوکس
sudo snix tui  →  تبِ Run            # لاگِ زنده در TUI
```

### H3. سوییچِ profile

```bash
snix profile list
snix profile switch workerB
sudo systemctl restart snix
```

### H4. به‌صورتِ دوره‌ای اسکن کنید

```bash
snix scan all
```

برایِ لاگِ هفتگیِ خودکار، از زیرفرمان‌هایی که JSON پشتیبانی می‌کنند
استفاده کنید (`scan all` تعاملی است):
```cron
0 3 * * 1 /usr/local/bin/snix scan sni --target 1.1.1.1 --json >> /var/log/snix-sni.jsonl
5 3 * * 1 /usr/local/bin/snix scan ip --json >> /var/log/snix-ip.jsonl
```

### H5. پشتیبانِ پیکربندی

```bash
cp ~/.config/snix/config.yaml ~/.config/snix/config.yaml.$(date +%F)
```

پیکربندی تنها state قابلِ پشتیبان‌گیری است. اگر گم شد، `snix init --wizard`
را دوباره اجرا کنید.

### H6. انتقال به دستگاهِ جدید

۱. snix را نصب کنید.
۲. `config.yaml` را از دستگاهِ قبلی به مسیرِ معادل در جدید کپی کنید
   (لینوکس: `~/.config/snix/`، ویندوز: `%APPDATA%\snix\`).
۳. مثلِ همیشه استارت کنید.

---

## ۱۲. عیب‌یابی بر اساسِ علامت

### ۱۲.۱ «No config loaded» در TUI / تبِ Home

یعنی snix نمی‌تواند `config.yaml` را پیدا کند. اجرا کنید:
```bash
snix status   # مسیرِ موردِ انتظار را نشان می‌دهد
snix init --wizard
```

### ۱۲.۲ ویندوز: `ERROR_ACCESS_DENIED (run as Administrator)`

TUIِ خود admin نیاز ندارد، ولی `snix start` دارد. TUI را از promptِ
بالارفته اجرا کنید یا موتور را از طریقِ سرویسِ ویندوز اجرا کنید.

### ۱۲.۳ ویندوز: `WinDivert.dll not loadable`

`snix.exe`، `WinDivert.dll` و `WinDivert64.sys` باید در یک پوشه باشند،
یا DLL روی `%PATH%`. نصب‌کننده این را خودکار انجام می‌دهد.

### ۱۲.۴ `snix scan` همه چیز را "reset" یا "timeout" می‌گوید

ISPِ شما احتمالاً TLSِ خام به `1.1.1.1` را مختل می‌کند. امتحان:
```bash
snix scan sni --target 104.21.2.17      # هر IPِ Cloudflare
```
اگر آن هم ناکام ماند، ISP بسیار خشن است. پشتهٔ کاملِ سخت‌سازی (بخشِ G)
به‌علاوهٔ احتمالاً VPS روی پورتِ غیرعادی لازم است.

### ۱۲.۵ `snix start` کار می‌کند ولی مرورگر به هیچ جا نمی‌رسد

زنجیره را بررسی کنید: مرورگر → Xray → snix → اینترنت.

گامِ ۱: آیا Xray اجرا است و گوش می‌دهد؟ `ss -tlnp | grep 1080` (یا پورتِ
SOCKS از پیکربندیِ Xray).

گامِ ۲: آیا outboundِ Xray به `127.0.0.1:40443` اشاره می‌کند؟
```bash
grep -A3 outbounds /usr/local/etc/xray/config.json
```
اگر نه، `snix init --wizard --force` را اجرا کنید (پیکربندیِ Xray را
patch می‌کند).

گامِ ۳: آیا snix وقتی مرور می‌کنید خط‌هایِ injection لاگ می‌کند؟
```bash
journalctl -u snix | grep injected
```
بدونِ injection = Xray به `127.0.0.1:40443` نمی‌رسد. خطایِ پیکربندیِ Xray.

گامِ ۴: آیا سرورِ واقعی کلاً قابلِ دسترسی است؟ از شبکهٔ دیگر:
```bash
curl -I https://snix.yourname.workers.dev/
```
اگر 502 یا timeout → مشکلِ سرور؛ Worker را دوباره deploy / VPS را restart.

### ۱۲.۶ اتصال ۳۰ ثانیه کار می‌کند، بعد قطع می‌شود

DPIِ مبتنی بر flow که الگوهایِ غیرعادی را دیرتر می‌گیرد. همه چیز در
بخشِ G را فعال کنید. به `sni_pool` ورودی‌هایِ بیشتری (حداقل ۱۰) اضافه
کنید.

### ۱۲.۷ CPU بالا

در این نسخه طبیعی است؛ zero-copy relay کار نسخهٔ v0.8 است.

### ۱۲.۸ قطع شدم از سرور، حالا اتصالِ مجدد ناکام است

۳۰ ثانیه صبر کنید (TCPِ `TIME_WAIT`)، بعد دوباره امتحان:
```bash
# لینوکس:
sudo systemctl restart snix xray
```

### ۱۲.۹ دستیار نتوانست Xray را پیدا کند

Xray روی `$PATH` نبود یا پیکربندی در جایِ غیرِاستاندارد. `~/.config/snix/config.yaml`
را دستی ویرایش کنید و طبقِ دستورالعملِ پایانیِ دستیار، Xray را به
`127.0.0.1:40443` اشاره دهید.

---

## ۱۳. پیوستِ A — واژه‌نامه

- **DPI** — Deep Packet Inspection. سخت‌افزارِ سانسور که درونِ بسته‌ها را
  نگاه می‌کند تا تصمیم بگیرد چه چیزی را بلاک کند.
- **SNI** — Server Name Indication. یک فیلدِ plain-text در اولین بستهٔ
  TLS که دامنهٔ موردِ درخواست را می‌گوید. محرکِ اصلیِ DPI.
- **ClientHello** — اولین بستهٔ TLS حاویِ SNI.
- **wrong_seq** — استراتژیِ پیش‌فرضِ snix. ClientHelloِ جعلی با شمارهٔ
  توالیِ TCPِ خارج از بازه.
- **wrong_checksum** — استراتژیِ جایگزین. ClientHelloِ جعلی با checksumِ
  خراب و شمارهٔ توالیِ خارج از بازه.
- **VLESS** — پروتکلِ پروکسیِ پنهان‌کارِ Xray. قالبِ Worker ما طرفِ
  سرورِ VLESS روی WebSocket را پیاده می‌کند.
- **VMess / Trojan** — پروتکل‌هایِ پنهان‌کارِ دیگر. snix با هر کدام کار
  می‌کند.
- **Cloudflare Worker** — قطعهٔ JSِ کوچکی که رویِ شبکهٔ جهانیِ Cloudflare
  رایگان اجرا می‌شود. پوششِ خوب: ترافیکِ شما شبیهِ cloudflare.com است.
- **NFQUEUE** — مکانیزمِ کرنلِ لینوکس که به snix اجازهٔ رهگیریِ بسته‌ها را
  می‌دهد.
- **WinDivert** — معادلِ ویندوز. درایورِ کرنل.
- **TUI** — رابطِ متنی. آنچه `snix tui` نشان می‌دهد.
- **Profile** — یک بستهٔ نام‌دارِ تنظیماتِ snix (listen، سرور، SNI pool،
  کلیدهایِ randomization). می‌توانید چند تا داشته باشید.

## ۱۴. پیوستِ B — بهداشتِ امنیتی

- **UUIDتان را هیچ‌وقت در معرضِ عموم نگذارید.** هرکس داشته باشد می‌تواند
  از Worker / VPSتان استفاده کند. اگر نشت کرد، عوض کنید.
- **`config.yaml` را در مخزنِ عمومی commit نکنید.** آدرسِ سرورتان در آن
  plain-text است.
- **هرگز build هایِ snix از افرادِ ناشناس را اجرا نکنید.** فقط از
  release هایِ امضاشدهٔ این repo استفاده کنید.
- **امضاهایِ release را تأیید کنید** اگر جای دیگر گرفتید:
  ```bash
  minisign -Vm snix-linux-amd64.tar.gz \
    -P "$(cat snix-pubkey.txt)"
  ```
- **رمزِ قوی برایِ حسابِ Cloudflare + 2FA.**
- snix خود هیچ تله‌متری ندارد. `snix update` تنها اتصالِ خروجی است که
  خودش ایجاد می‌کند، و فقط وقتی اجرا کنید.

## ۱۵. پیوستِ C — حذفِ تمیز

### لینوکس

```bash
sudo systemctl disable --now snix 2>/dev/null
sudo rm -f /usr/local/bin/snix
sudo rm -f /etc/systemd/system/snix.service
sudo systemctl daemon-reload
# اختیاری: حذفِ پیکربندی و لاگ‌ها
rm -rf ~/.config/snix
```

### ویندوز

Control Panel → Programs → snix → Uninstall. این سرویسِ ویندوز را هم
حذف می‌کند اگر نصب شده باشد. درایورِ WinDivert تا restart می‌ماند
(بی‌ضرر).

### Cloudflare Worker

در داشبوردِ Cloudflare → Workers → Workerِ snix → **Delete**.

### VPS

VM را در ارائه‌دهندهٔ خود cancel / destroy کنید. روی کامپیوترِ خودتان
چیزی فراتر از خودِ snix برایِ پاک کردن نیست.

---

رسیدید. اگر چیزی در این راهنما نیست، یک issue باز کنید — اضافه می‌کنیم.

</div>
