**3X-UI: Một Bảng Điều Khiển Được Xây Dựng trên Xray Core**

[![](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](#)
[![Phiên Bản GO](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Tải Xuống](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](#)
[![Giấy Phép](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Lưu Ý:** Dự án này chỉ dành cho việc học tập và giao tiếp cá nhân, vui lòng không sử dụng cho mục đích bất hợp pháp, vui lòng không sử dụng trong môi trường sản xuất.

**Nếu dự án này hữu ích với bạn, bạn có thể muốn đánh giá nó**:star2:

<p align="left"><a href="#"><img width="125" src="https://github.com/MHSanaei/3x-ui/assets/115543613/7aa895dd-048a-42e7-989b-afd41a74e2e1" alt="Hình ảnh"></a></p>

- USDT (TRC20): `TXncxkvhkDWGts487Pjqq1qT9JmwRUz8CC`

## Cài Đặt & Nâng Cấp

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

## Cài Đặt Phiên Bản Tùy Chỉnh

Để cài đặt phiên bản mong muốn của bạn, thêm phiên bản vào cuối lệnh cài đặt. Ví dụ, phiên bản `v2.2.6`:

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v2.2.6
```

## Chứng Chỉ SSL

<details>
  <summary>Ấn để Xem Chứng Chỉ SSL</summary>

### Cloudflare

Kịch bản Quản lý tích hợp sẵn ứng dụng chứng chỉ SSL cho Cloudflare. Để sử dụng kịch bản này để đăng ký chứng chỉ, bạn cần những điều sau:

- Email đăng ký Cloudflare
- Cần khóa Global API trong tài khoản Cloudflare của bạn
- Tên miền đã được giải quyết đến máy chủ hiện tại thông qua cloudflare

**1:** Chạy lệnh `x-ui` trên terminal, sau đó chọn `Cloudflare SSL Certificate`.

### Certbot
```
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

***Gợi ý:*** *Certbot cũng được tích hợp trong kịch bản Quản lý. Bạn có thể chạy lệnh `x-ui`, sau đó chọn `Quản lý Chứng chỉ SSL`.*

</details>

## Cài Đặt & Nâng Cấp Thủ Công

<details>
  <summary>Ấn để xem chi tiết cài đặt thủ công</summary>

#### Cách Sử Dụng

1. Để tải phiên bản mới nhất của gói nén trực tiếp xuống máy chủ của bạn, hãy chạy lệnh sau:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;;
  i*86 | x86) XUI_ARCH="386" ;;
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;;
  armv7* | armv7) XUI_ARCH="armv7" ;;
  armv6* | armv6) XUI_ARCH="armv6" ;;
  armv5* | armv5) XUI_ARCH="armv5" ;;
  *) XUI_ARCH="amd64" ;;
esac


wget https://github.com/MHSanaei/3x-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. Khi gói nén được tải xuống, thực thi các lệnh sau để cài đặt hoặc nâng cấp x-ui:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;;
  i*86 | x86) XUI_ARCH="386" ;;
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;;
  armv7* | armv7) XUI_ARCH="armv7" ;;
  armv6* | armv6) XUI_ARCH="armv6" ;;
  armv5* | armv5) XUI_ARCH="armv5" ;;
  *) XUI_ARCH="amd64" ;;
esac

cd /root/
rm -rf x-ui/ /usr/local/x-ui/ /usr/bin/x-ui
tar zxvf x-ui-linux-${XUI_ARCH}.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

</details>

## Cài Đặt với Docker

<details>
  <summary>Ấn để xem chi tiết Docker</summary>

#### Cách Sử Dụng

1. Cài đặt Docker:

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. Sao chép Repository Dự Án:

   ```sh
   git clone https://github.com/MHSanaei/3x-ui.git
   cd

 3x-ui
   ```

3. Khởi chạy Dịch Vụ

   ```sh
   docker compose up -d
   ```

   HOẶC

   ```sh
   docker run -itd \
      -e XRAY_VMESS_AEAD_FORCED=false \
      -v $PWD/db/:/etc/x-ui/ \
      -v $PWD/cert/:/root/cert/ \
      --network=host \
      --restart=unless-stopped \
      --name 3x-ui \
      ghcr.io/mhsanaei/3x-ui:latest
   ```

cập nhật phiên bản mới nhất

   ```sh
    cd 3x-ui
    docker compose down
    docker compose pull 3x-ui
    docker compose up -d
   ```

xóa 3x-ui từ docker 

   ```sh
    docker stop 3x-ui
    docker rm 3x-ui
    cd --
    rm -r 3x-ui
   ```

</details>


## Hệ Điều Hành Được Khuyến Nghị

- Ubuntu 20.04+
- Debian 11+
- CentOS 8+
- Fedora 36+
- Arch Linux
- Manjaro
- Armbian
- AlmaLinux 9+
- Rockylinux 9+

## Kiến Trúc và Thiết Bị Được Hỗ Trợ

<details>
  <summary>Ấn để xem chi tiết kiến trúc và thiết bị được hỗ trợ</summary>

Nền tảng của chúng tôi cung cấp tính tương thích với một loạt các kiến trúc và thiết bị đa dạng, đảm bảo tính linh hoạt trên nhiều môi trường máy tính khác nhau. Dưới đây là các kiến trúc chính mà chúng tôi hỗ trợ:

- **amd64**: Kiến trúc phổ biến này là tiêu chuẩn cho máy tính cá nhân và máy chủ, phù hợp với hầu hết các hệ điều hành hiện đại.

- **x86 / i386**: Được sử dụng rộng rãi trong máy tính để bàn và laptop, kiến trúc này được hỗ trợ rộng rãi từ nhiều hệ điều hành và ứng dụng khác nhau, bao gồm nhưng không giới hạn ở Windows, macOS và các hệ điều hành Linux.

- **armv8 / arm64 / aarch64**: Được tinh chỉnh cho các thiết bị di động và nhúng hiện đại, như điện thoại thông minh và máy tính bảng, kiến trúc này được thể hiện qua các thiết bị như Raspberry Pi 4, Raspberry Pi 3, Raspberry Pi Zero 2/Zero 2 W, Orange Pi 3 LTS và nhiều hơn nữa.

- **armv7 / arm / arm32**: Được sử dụng làm kiến trúc cho các thiết bị di động và nhúng cũ hơn, nó vẫn được sử dụng rộng rãi trong các thiết bị như Orange Pi Zero LTS, Orange Pi PC Plus, Raspberry Pi 2, và nhiều thiết bị khác.

- **armv6 / arm / arm32**: Được thiết kế cho các thiết bị nhúng cũ, kiến trúc này, mặc dù không phổ biến nhưng vẫn được sử dụng. Các thiết bị như Raspberry Pi 1, Raspberry Pi Zero/Zero W, dựa vào kiến trúc này.

- **armv5 / arm / arm32**: Là một kiến trúc cũ hơn chủ yếu liên quan đến các hệ thống nhúng sớm, nó ít phổ biến hơn ngày nay nhưng vẫn có thể được tìm thấy trong các thiết bị kế thừa như các phiên bản Raspberry Pi sớm và một số điện thoại thông minh cũ.

</details>

## Ngôn Ngữ

- Tiếng Anh
- Tiếng Ba Tư
- Tiếng Trung
- Tiếng Nga
- Tiếng Việt
- Tiếng Tây Ban Nha
- Tiếng Indonesia 
- Tiếng Ukraina


## Tính Năng

- Giám Sát Trạng Thái Hệ Thống
- Tìm Kiếm Trong Tất Cả Các Đầu Và Khách Hàng
- Chủ Đề Tối/Sáng
- Hỗ Trợ Đa Người Dùng và Đa Giao Thức
- Hỗ Trợ Các Giao Thức, Bao Gồm VMess, VLESS, Trojan, Shadowsocks, Dokodemo-door, Socks, HTTP, wireguard
- Hỗ Trợ Các Giao Thức XTLS Native, Bao Gồm RPRX-Direct, Vision, REALITY
- Thống Kê Lưu Lượng, Giới Hạn Lưu Lượng, Thời Gian Hết Hạn
- Mẫu Cấu Hình Xray Có Thể Tùy Chỉnh
- Hỗ Trợ Truy Cập Bảng Bằng HTTPS (Tên Miền Tự Cung Cấp + Chứng Chỉ SSL)
- Hỗ Trợ Đăng Ký Chứng Chỉ SSL và Tự Động Gia Hạn Bằng Một Cú Nhấp Chuột
- Đối Với Các Mục Cấu Hình Nâng Cao Hơn, Vui Lòng Tham Khảo Bảng Điều Khiển
- Sửa lỗi các tuyến API (cài đặt người dùng sẽ được tạo với API)
- Hỗ trợ thay đổi cấu hình bằng các mục khác nhau được cung cấp trong bảng điều khiển.
- Hỗ trợ xuất/nhập cơ sở dữ liệu từ bảng điều khiển


## Cài Đặt Mặc Định



<details>
  <summary>Ấn để xem chi tiết cài đặt mặc định</summary>

  ### Thông Tin

- **Cổng:** 2053
- **Tên Người Dùng & Mật Khẩu:** Nó sẽ được tạo ngẫu nhiên nếu bạn bỏ qua việc chỉnh sửa.
- **Đường Dẫn Cơ Sở Dữ Liệu:**
  - /etc/x-ui/x-ui.db
- **Đường Dẫn Cấu Hình Xray:**
  - /usr/local/x-ui/bin/config.json
- **Đường Dẫn Bảng Mạng w/o Triển Khai SSL:**
  - http://ip:2053/panel
  - http://domain:2053/panel
- **Đường Dẫn Bảng Mạng w/ Triển Khai SSL:**
  - https://domain:2053/panel
 
</details>

## [Cấu Hình WARP](https://gitlab.com/fscarmen/warp)

<details>
  <summary>Ấn để xem chi tiết cấu hình WARP</summary>

#### Cách Sử Dụng

Nếu bạn muốn sử dụng định tuyến đến WARP trước phiên bản `v2.1.0` theo các bước sau:

**1.** Cài Đặt WARP ở **Chế Độ Proxy SOCKS**:

   ```sh
   bash <(curl -sSL https://raw.githubusercontent.com/hamid-gh98/x-ui-scripts/main/install_warp_proxy.sh)
   ```

**2.** Nếu bạn đã cài đặt warp rồi, bạn có thể gỡ cài đặt bằng lệnh dưới đây:

   ```sh
   warp u
   ```

**3.** Bật cấu hình bạn cần trong bảng điều khiển

   Các Tính Năng Cấu Hình:

   - Chặn Quảng Cáo
   - Định Tuyến Google + Netflix + Spotify + OpenAI (ChatGPT) đến WARP
   - Sửa Lỗi 403 của Google

</details>

## Giới Hạn IP

<details>
  <summary>Ấn để xem chi tiết giới hạn IP</summary>

#### Cách Sử Dụng

**Lưu Ý:** Giới Hạn IP sẽ không hoạt động đúng khi sử dụng Đường Hầm IP

- Đối với các phiên bản đến `v1.6.1`:

  - Giới hạn IP được tích hợp trong bảng điều khiển.

- Đối với các phiên bản `v1.7.0` và mới hơn:

  - Để giúp Giới Hạn IP hoạt động đúng, bạn cần cài đặt fail2ban và các tệp cần thiết của nó bằng cách thực hiện các bước sau:

    1. Sử dụng lệnh `x-ui` trong shell.
    2. Chọn `Quản Lý Giới Hạn IP`.
    3. Chọn các tùy chọn phù hợp dựa trên nhu cầu của bạn.
   
  - đảm bảo rằng bạn có ./access.log trên Cấu Hình Xray của mình sau v2.1.3 chúng tôi có một lựa chọn cho nó
  
  ```sh
    "log": {
      "access": "./access.log",
      "dnsLog": false,
      "loglevel": "warning"
    },
  ```

</details>

## Bot Telegram

<details>
  <summary>Ấn để xem chi tiết bot Telegram</summary>

#### Cách Sử Dụng

Bảng điều khiển web hỗ trợ lưu lượng hàng ngày, thông báo đăng nhập, sao lưu cơ sở dữ liệu, trạng thái hệ thống, thông tin khách hàng và các chức năng khác thông qua Bot Telegram. Để sử dụng bot, bạn cần thiết lập các thông số liên quan đến bot trong bảng điều khiển, bao gồm:

- Mã Token Telegram
- ID Trò Chuyện Quản Trị (các)
- Thời Gian Thông Báo (theo cú pháp cron)
- Thông Báo Ngày Hết Hạn
- Thông Báo Giới Hạn Lưu Lượng
- Sao Lưu Cơ Sở Dữ Liệu
- Thông Báo Tải Cpu


**Cú Pháp Tham Khảo:**

- `30 \* \* \* \* \*` - Thông báo vào 30 giây của mỗi điểm
- `0 \*/10 \* \* \* \*` - Thông báo vào giây đầu tiên của mỗi 10 phút
- `@hourly` - Thông báo hàng giờ
- `@daily` - Thông báo hàng ngày (00:00 sáng)
- `@weekly` - thông báo hàng tuần
- `@every 8h` - Thông báo mỗi 8 giờ

### Tính Năng Bot Telegram

- Báo Cáo Định Kỳ
- Thông Báo Đăng Nhập
- Thông Báo Ngưỡng CPU
- Ngưỡng cho Thời Gian Hết Hạn và Lưu Lượng để báo cáo trước
- Hỗ trợ menu báo cáo của khách hàng nếu tên người dùng telegram của khách hàng được thêm vào cấu hình người dùng
- Hỗ trợ báo cáo lưu lượng telegram được tìm kiếm bằng UUID (VMESS/VLESS) hoặc Mật khẩu (TROJAN) - ẩn danh
- Bot dựa trên menu
- Tìm kiếm khách hàng bằng email ( chỉ admin )
- Kiểm tra tất cả các inbounds
- Kiểm tra trạng thái máy chủ
- Kiểm tra khách hàng cạn kiệt
- Nhận bản sao lưu theo yêu cầu và trong các

 thông báo sao lưu hàng ngày
- Sao lưu ngay lập tức và nhận liên kết để tải về
- Sao lưu dựa trên menu


</details>


## Làm Thế Nào Để Sử Dụng 3X-UI?

Khi bạn đã cài đặt 3X-UI trên máy chủ của mình, bạn có thể truy cập vào giao diện quản lý thông qua trình duyệt web. Thông thường, bạn có thể truy cập vào địa chỉ IP của máy chủ của mình trên cổng mặc định 2053 (hoặc địa chỉ miền của bạn nếu bạn đã cấu hình DNS đúng cách).

Nếu bạn cần hỗ trợ hoặc muốn tham gia vào cộng đồng phát triển của 3X-UI, hãy ghé thăm [Github Repository](https://github.com/MHSanaei/3x-ui) của dự án. Bạn có thể đóng góp ý kiến, báo cáo lỗi hoặc đề xuất các tính năng mới.

Chú ý: Đây là một dự án mã nguồn mở được phát triển bởi cộng đồng, hãy tuân thủ các quy tắc và điều khoản của dự án khi sử dụng hoặc đóng góp vào nó.
