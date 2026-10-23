# Mattermost production với PostgreSQL và S3 bên ngoài

Compose này chỉ chạy một container Mattermost. PostgreSQL, S3 và SMTP dùng dịch vụ đã có. HTTPS được xử lý tại reverse proxy bên ngoài.

## Cấu hình và chạy

```sh
cd deploy/production
cp .env.example .env
chmod 600 .env
openssl rand -hex 32
```

Điền `.env`, gồm image, domain, PostgreSQL, S3, SMTP và đưa chuỗi ngẫu nhiên vừa tạo vào `MM_FILESETTINGS_PUBLICLINKSALT`. Giữ nguyên salt khi nâng cấp. Các biến bắt buộc chưa điền sẽ khiến Compose từ chối chạy.

`MATTERMOST_IMAGE` phải là image runtime đã build từ phiên bản repo muốn triển khai, có server, webapp và mmctl tại `/mattermost`. Dùng tag cố định hoặc digest. Image `mattermost-build-server` trong workflow hiện tại là môi trường build, không phải image runtime. Image Mattermost upstream sẽ không có những thay đổi riêng của fork này.

```sh
docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100 -f mattermost
```

Không chạy Compose này trên cùng database với một server Mattermost khác đang hoạt động. Server tự chạy migration khi khởi động. Với database đã dùng migration 236 cho OIDC, tính năng posting/membership dùng migration 237.

## Dịch vụ bên ngoài

- PostgreSQL: database và user phải tồn tại. User cần quyền tạo/sửa bảng và index để chạy migration. Mặc định URL dùng `sslmode=verify-full`; với CA riêng, bật mount PEM trong Compose và thêm `sslrootcert` vào URL. Host trong URL phải truy cập được từ container, không dùng `localhost` để trỏ về host Docker.
- S3: bucket phải tồn tại. Tài khoản cần list bucket và đọc/ghi/xóa object, cùng quyền multipart upload. Endpoint chỉ chứa hostname và port, không có scheme hoặc tên bucket. Với AWS S3, để endpoint trống và đặt đúng region. Nếu dùng KMS hoặc bucket policy riêng, cấp thêm quyền tương ứng.
- SMTP: điền server, tài khoản, mật khẩu và địa chỉ gửi hợp lệ. Mặc định bật xác minh email nên SMTP phải hoạt động để hoàn tất luồng đăng ký.

## Reverse proxy

Mặc định chỉ publish `127.0.0.1:8065`. Reverse proxy trên host chuyển tiếp tới `http://127.0.0.1:8065`, giữ `Host`, đặt `X-Forwarded-Proto: https`, `X-Forwarded-For`, hỗ trợ WebSocket Upgrade và timeout dài cho `/api/v4/websocket`. Giới hạn upload cần khớp `MM_FILESETTINGS_MAXFILESIZE`.

Nếu reverse proxy nằm trên máy khác, đổi `MATTERMOST_BIND_IP` thành IP private của host và cho phép proxy truy cập cổng đó qua firewall. Nếu proxy chạy trong container khác, `127.0.0.1` bên trong proxy không trỏ về host; nối hai service vào cùng Docker network và dùng `mattermost:8065`, hoặc cấu hình đường tới host phù hợp.

## Tài khoản quản trị đầu tiên

Tạo admin qua mmctl. Trong Bash, đọc mật khẩu vào biến rồi truyền cho lệnh:

```bash
read -r -s -p 'Admin password: ' admin_password
printf '\n'
docker compose exec mattermost /mattermost/bin/mmctl --local user create \
  --email admin@example.com --username admin --password "$admin_password" \
  --system-admin --email-verified
unset admin_password
```

Healthcheck dùng mmctl qua Unix socket nội bộ. Local mode được bật cho CLI; socket không được mount hoặc publish ra ngoài.

## OIDC và plugin

Các biến `MM_OPENIDSETTINGS_*` dành cho OIDC native, mặc định tắt. Chỉ bật khi image có provider Enterprise và license phù hợp. Plugin `mattermost-oidc` có backend và cấu hình riêng; cài plugin trong image hoặc tạm bật `MM_PLUGINSETTINGS_ENABLEUPLOADS`, cài qua System Console rồi tắt lại. Callback URL phải lấy theo provider/plugin đang dùng.

## Dữ liệu và nâng cấp

File đính kèm lưu ở S3. Docker volumes giữ config, plugin, dữ liệu tạm của import/export và index. Log console được Docker xoay vòng. Backup database, bucket, `.env` và các volumes cần giữ trước khi đổi image. Không dùng `docker compose down -v` khi muốn giữ dữ liệu.

Các giá trị được đặt qua env có ưu tiên cao hơn cấu hình lưu trong System Console. Sau khi sửa `.env`, áp dụng bằng `docker compose up -d`.
