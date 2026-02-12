<div dir="rtl">

# 📡 GRPCClient - Wrapper حرفه‌ای gRPC برای Real-time و Streaming

این ماژول یک **gRPC wrapper پیشرفته برای Go** ارائه می‌دهد که برای **chat apps، messaging و real-time streaming** بهینه شده است.
ویژگی‌ها:

- ✅ **Short RPC Pool:** برای callهای کوتاه (ارسال پیام، دریافت کاربران، fetch data)
- ✅ **Dedicated Streaming Connection:** connection طولانی با auto-reconnect برای streaming real-time
- ⚡ **Performance بالا و overhead کم**
- 🔄 **Retry خودکار با exponential backoff** برای callهای کوتاه
- ⏱️ **Context-aware و timeout قابل تنظیم**
- 🏗️ **Thread-safe و scalable**

---

## 🛠️ نصب

```bash
go get github.com/Skryldev/grpcclient
```
## 🔑 وارد کردن به فایل
```
import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/askari/gpm/grpc-demo/proto/userpb"
	"github.com/Skryldev/grpcclient
	"google.golang.org/grpc"
)
```
---

## ⚙️ پیکربندی

## ⚙️ پیکربندی WrapperClient (Config)

| فیلد | نوع | واحد | مقدار پیش‌فرض | توضیح کامل |
|------|-----|------|----------------|------------|
| `DialTimeout` | `time.Duration` | ثانیه | 5s | **تایم‌اوت اتصال اولیه به سرور gRPC**. اگر سرور پاسخ ندهد یا شبکه کند باشد، پس از این مدت اتصال قطع می‌شود. مهم برای کنترل latency شروع client و جلوگیری از hanging. |
| `CallTimeout` | `time.Duration` | ثانیه | 3s | **تایم‌اوت هر Short RPC call**. هر call کوتاه که با `ShortCall` اجرا می‌شود، باید در این زمان کامل شود، در غیر این صورت منقضی می‌شود. |
| `MaxRetries` | `int` | دفعات | 3 | **تعداد تلاش مجدد برای Short RPC در خطاهای موقت**. خطاهای قابل retry شامل `Unavailable`, `DeadlineExceeded`, `ResourceExhausted` هستند. افزایش مقدار باعث reliability بیشتر ولی latency بالقوه بیشتر می‌شود. |
| `BackoffFactor` | `float64` | ضریب | 2.0 | **ضریب exponential backoff بین retryها**. مثال: اگر backoff اولیه 100ms باشد، تلاش دوم 200ms، تلاش سوم 400ms و … خواهد بود. |
| `PoolSize` | `int` | تعداد connection | 5 | **تعداد connectionهای Short RPC در pool**. هر call کوتاه از یکی از connectionهای pool استفاده می‌کند. افزایش poolSize باعث concurrency بیشتر و کاهش wait time می‌شود اما مصرف منابع را افزایش می‌دهد. |
| `StreamRetry` | `int` | دفعات | 5 | **تعداد تلاش reconnect برای Streaming connection**. اگر stream طولانی قطع شود، client حداکثر این تعداد reconnect تلاش می‌کند. |
| `StreamBackoff` | `time.Duration` | میلی‌ثانیه | 200ms | **مدت زمان انتظار بین تلاش‌های reconnect برای stream**. کنترل می‌کند reconnect سریع باشد یا با فاصله برای کاهش فشار روی سرور و شبکه. |

### 🔹 نکات حرفه‌ای

- **DialTimeout vs CallTimeout:**
  - DialTimeout فقط برای ایجاد connection اولیه است
  - CallTimeout برای هر RPC کوتاه استفاده می‌شود

- **MaxRetries و BackoffFactor:**
  - افزایش retries → reliability بیشتر، latency بیشتر
  - BackoffFactor > 1 → فاصله retry بصورت exponential افزایش می‌یابد

- **PoolSize:**
  - برای high-concurrency مقدار بالاتر مناسب است
  - برای سیستم‌های با منابع محدود مقدار متوسط کافی است

- **StreamRetry و StreamBackoff:**
  - برای long-lived stream (چت یا real-time) مهم است
  - بهبود reliability بدون از دست دادن state
  - باید متناسب با شبکه و latency تنظیم شود


---

## 🚀 استفاده پایه

### 1️⃣ ایجاد client

<div dir="ltr">

```
cfg := grpcclient.Config{
	DialTimeout:   5*time.Second,
	CallTimeout:   2*time.Second,
	MaxRetries:    3,
	BackoffFactor: 2.0,
	PoolSize:      5,
	StreamRetry:   5,
	StreamBackoff: 200*time.Millisecond,
}

client, err := grpcclient.NewWrapperClient("localhost:50051", cfg)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

<div dir="rtl">

### 2️⃣ اجرای یک RPC call (callهای کوتاه) (روش 1)

<div dir="ltr">

```
resp, err := client.ShortCall(context.Background(), func(ctx context.Context, conn *grpc.ClientConn) (interface{}, error) {
	userClient := pb.NewUserServiceClient(conn)
	return userClient.GetUser(ctx, &pb.GetUserRequest{Id: 1})
})

if err != nil {
	log.Fatalf("ShortCall failed: %v", err)
}
fmt.Printf("User: %+v\n", resp)
```
<div dir="rtl">

#### 🔹 Short RPC از connection pool استفاده می‌کند و retry خودکار دارد.

### 🌐 اجرای چند call همزمان (Concurrency) (روش 2)

<div dir="ltr">

```
for i := 1; i <= 10; i++ {
	go func(id int) {
		resp, err := client.ShortCall(context.Background(), func(ctx context.Context, conn *grpc.ClientConn) (interface{}, error) {
			userClient := pb.NewUserServiceClient(conn)
			return userClient.GetUser(ctx, &pb.GetUserRequest{Id: int64(id)})
		})
		if err != nil {
			log.Printf("Call %d failed: %v", id, err)
			return
		}
		fmt.Printf("User %d: %+v\n", id, resp)
	}(i)
}
time.Sleep(3 * time.Second)
```
<div dir="rtl">

### ⚡ Streaming (real-time / live chat) (روش 3)

<div dir="ltr">

```
err = client.StreamCall(func(ctx context.Context, conn *grpc.ClientConn) error {
	chatClient := pb.NewChatServiceClient(conn)
	stream, err := chatClient.Chat(ctx)
	if err != nil {
		return err
	}

	// ارسال پیام
	if err := stream.Send(&pb.ChatMessage{UserId: 1, Text: "سلام!"}); err != nil {
		return err
	}

	// دریافت پیام‌ها
	msg, err := stream.Recv()
	if err != nil {
		return err
	}
	fmt.Printf("Received: %v\n", msg)
	return nil
})

if err != nil {
	log.Fatalf("Streaming failed: %v", err)
}
```
<div dir="rtl">

#### 🔹 Streaming از یک connection اختصاصی استفاده می‌کند و auto-reconnect دارد. Retry خودکار برای stream غیرفعال است تا state حفظ شود.
---
## 🔄 Retry و Exponential Backoff
- هر RPC call که با خطای موقت مواجه شود (مانند `Unavailable`, `DeadlineExceeded`) به صورت خودکار **retry می‌شود**
- فاصله بین retry‌ها به صورت **exponential backoff** افزایش می‌یابد
- می‌توانید با `Config.MaxRetries` و `Config.BackoffFactor` آن را شخصی‌سازی کنید.

## 📝 نکات حرفه‌ای
1. **Concurrency-safe**: می‌توانید wrapper را بین goroutineها به اشتراک بگذارید
2. **Retry و backoff فقط برای Short RPC**: هstream بدون retry برای حفظ state
3. **Performance**: هoverhead حداقلی، latency برای callهای موفق ≈ gRPC مستقیم
4. **Scaling**: هPoolSize و StreamRetry قابل تنظیم برای پروژه‌های بزرگ

## 💡 پیشنهادات
- برای callهای کوتاه و frequent از **ShortCall** استفاده کنید
- برای real-time stream طولانی، **StreamCall** استفاده کنید
- برای latency حساس، backoff و retry قابل تنظیم هستند
- می‌توان wrapper را با logging و metrics ترکیب کرد

---
## 📦 خلاصه
### grpcclient یک راه حل حرفه‌ای و انعطاف‌پذیر برای استفاده از gRPC در Go است، با ویژگی‌های زیر:
- Generic و reusable
- Multi-call و thread-safe
- Retry هوشمند و backoff
- Connection pool واقعی
- Production-ready و مناسب concurrency-heavy
---
**✅ این ماژول مناسب پروژه‌های real-world و مکانیزم‌های همزمانی بالا است و به شما اجازه می‌دهد بدون نگرانی از مدیریت connection یا retry، روی منطق برنامه تمرکز کنید.**
