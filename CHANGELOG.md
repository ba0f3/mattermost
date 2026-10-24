# Changelog

Các thay đổi đáng chú ý của fork này. Ngày theo múi giờ Việt Nam (UTC+7).
Upstream release notes: <https://docs.mattermost.com/administration/changelog.html>.

## 2026-10-25 — Fork 12.1.0-dev

### Thêm mới

- **Read receipts (Seen by)**: API server, UI hiển thị trạng thái đã gửi/đã xem, modal danh sách người đã xem kèm reactions. Mặc định bật, không có tuỳ chọn tắt; dấu check trên post có thể bấm để mở modal trực tiếp.
- **Đếm tin nhắn chưa đọc**: hiển thị số bên cạnh tên channel ở sidebar, badge trên favicon và icon app đã cài; có setting tắt đếm trên sidebar.
- **Markdown file preview**: xem file `.md` ngay trong modal preview với toggle Preview/Source; render Mermaid trong sandboxed worker; bổ sung chuỗi tiếng Việt.
- **Posting policies & mandatory membership**: chính sách đăng bài theo channel và yêu cầu membership bắt buộc.
- **Production deploy**: Compose cho Postgres/S3 bên ngoài, host bind mounts và domain routing; tài liệu login chỉ bằng OIDC plugin.
- **Dev workflow**: root `Makefile` để start/stop server + web, load `.env`, tạo admin, cài plugin Agents, build/package và tạo image runtime `linux/amd64`; publish image lên GHCR.

### Thay đổi

- Nâng giới hạn user của bản không license lên mức thực tế không giới hạn (`limitless`).
- Chat composer: bố cục mới, bật rich text editing mặc định.
- Product menu/branding: dùng logo custom từ System Console; ẩn badge edition, mục edition và các CTA upgrade/trial.
- Acknowledgements: cho phép dùng không cần license; yêu cầu user thuộc channel và có yêu cầu ack trước khi acknowledge.
- Tiếng Việt: `Forward` dịch thành **"Chuyển tiếp"** (thay vì "Phía trước"/"Tiếp theo").
- Read receipts: giữ trạng thái đã xem sau khi Mark as Unread bằng con trỏ đơn điệu; loại bot/integration post khỏi thống kê; dấu check vẫn hiển thị khi hover.

### Sửa lỗi

- Trang login hiển thị và tự chuyển sang OIDC plugin khi email/username login bị tắt; desktop mở popup, có `?oidc_manual=1` để hiện nút thủ công.
- Production webapp compile shared components từ source, tránh lẫn JSX development từ Parcel watch vào bundle production.
- Chỉ gọi API trial license trên bản Enterprise-ready, tránh request thừa ở Team Edition.
- CI fork: chuyển buildenv/server image sang GHCR, bỏ phụ thuộc Docker Hub secrets; skip migration review khi thiếu `ANTHROPIC_API_KEY`; gate FIPS jobs cho upstream.

### CI / Build / Deploy

- `make build` compile server + production web client và tạo package.
- `make package` đóng gói lại từ binary/web client có sẵn.
- `make image` tạo image từ package có sẵn (tự build nếu chưa có); mặc định `linux/amd64`, hỗ trợ `IMAGE_REBUILD=true`.
- `make image-push` push lên GHCR.
- Build server amd64 trên máy ARM cần binfmt/QEMU (Docker Desktop có sẵn; Linux dùng `tonistiigi/binfmt`).
- Cloud Agent: sửa nested Docker startup/socket access; mặc định cài Team Edition khi không có Enterprise.

### Tài liệu

- README mới: yêu cầu toolchain, quick start, cấu hình `.env`, start/stop dev, build/package/image.
- `deploy/production/README.md`: hướng dẫn OIDC plugin-only, biến môi trường và lưu ý vận hành.

### Commit tham khảo

| Khu vực | Commit |
|---|---|
| Dev workflow / Makefile | `0e32ffd0e9`, `d1cbf54417`, `64041164d0`, `f46613fdf6` |
| Seat limits | `bce8cfa44d` |
| Ẩn upgrade/trial, branding | `9a549530f1`, `857c755c07`, `30f27ce6f5`, `bd977ec845` |
| Composer / i18n | `b7b1588915`, `b9dbf4d29c`, `e899312f78` |
| Read receipts | `3b4d6f0b66`, `5e7f2be4fd`, `4884ba636c`, `8d0cbab2fa`, `e164580b5b`, `537962b812` |
| Unread counts | `760b252f84`, `e75a200d96`, `001903b6fb` |
| Markdown preview | `31b276037b`, `253e09591d`, `e76af1a99b` |
| Posting policies / ack | `dd3fa3d800`, `5189e7082d`, `e72e126475` |
| Production deploy / OIDC | `bf298fe64e`, `ce232c8b09`, `c6388d7b23`, `1a3d04b0be` |
| CI GHCR | `9fe94cf97f`, `0a1f7a1af9` |
| Production shared build | `e622ea53b0`, `9de04a60b6` |
| Docs | `8a93911263`, `1a3d04b0be` |
