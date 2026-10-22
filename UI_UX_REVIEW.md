# Đánh giá UI UX và hướng thiết kế 568Portal

Ngày 22 tháng 10 năm 2026. Khảo sát bằng Orca CLI trên bản chạy tại k7:8065, desktop 1440×900 và mobile 390×844. Phạm vi gồm đăng nhập, bản nháp, Town Square, tìm kiếm và chủ đề hội thoại.

Hướng đề xuất: giao diện sáng, hiện đại, thanh lịch, đơn giản. Ưu tiên đọc và trả lời tin nhắn; giảm thanh công cụ, đường viền và thông tin phụ. Bản ghi này xác định hướng thiết kế và backlog. Trạng thái triển khai được cập nhật ở cuối.

## Nhận xét về layout hiện tại

Giao diện mang cấu trúc quen thuộc của Slack và Mattermost: thanh trên cùng, sidebar kênh, tiêu đề kênh, dòng tin nhắn, composer và thanh tiện ích bên phải. Cấu trúc này vẫn dễ học; vấn đề chính là quá nhiều lớp điều khiển cùng xuất hiện và phân cấp thị giác yếu.

| Quan sát | Ảnh hưởng | Chỉnh sửa đề xuất |
| --- | --- | --- |
| Thanh ứng dụng, tiêu đề kênh, sidebar và rail tiện ích cùng hiện diện | Nội dung bị bao quanh bởi nhiều lớp giao diện | Giữ sidebar và một tiêu đề hội thoại; gộp chức năng toàn cục vào sidebar và tìm kiếm |
| Tìm kiếm toàn cục và tìm kênh nằm gần nhau nhưng khác phạm vi | Người mới phải đoán nên dùng ô nào | Một điểm tìm chính có bộ lọc Tin nhắn, Người, Kênh, Tệp; giữ phím tắt chuyển kênh cho người dùng quen |
| Thanh định dạng luôn mở với nhiều biểu tượng | Composer giống trình soạn tài liệu, chiếm chú ý dù chưa nhập | Mặc định chỉ hiện nhập tin, đính kèm, emoji, định dạng và gửi; mở định dạng khi bấm Aa |
| Phản hồi nằm trong dòng chat, có dòng tham chiếu và thêm dấu trích dẫn | Hội thoại nhiều tầng, khó theo dõi mạch chính | Với chủ đề, hiển thị một tin gốc và hàng “N trả lời · người trả lời · cập nhật”; mở nội dung trong panel khi chọn |
| Dấu trả lời và số lượng ở sát mép phải, xa nội dung | Mắt phải quét qua khoảng trống để nối hành động với tin | Đặt “N trả lời” ngay dưới tin gốc |
| Nội dung tin chạy ngang gần hết vùng chính | Tin dài khó đọc trên desktop rộng | Giới hạn cột nội dung khoảng 800–880px, căn trái trong vùng chat; bảng và mã có thể mở rộng |
| Nhiều nền xám gần nhau, chữ phụ nhỏ và nhạt | Tổng thể thiếu điểm nhấn, hơi mờ | Dùng nền trắng ấm, sidebar phân biệt nhẹ; tăng độ rõ của chữ và trạng thái được chọn |
| Rail bên phải luôn chiếm một dải nhưng ít chức năng | Thêm nhiễu và mất diện tích | Chuyển tiện ích vào menu tiêu đề; chỉ mở panel khi cần |

Không chuyển sang bong bóng cho mọi tin nhắn trong kênh. Giữ dòng tin gọn cho hội thoại nhóm, gom các tin liên tiếp cùng người và tách nhóm bằng khoảng cách. Không lặp avatar, tên, giờ cho từng dòng ngắn.

## Layout đề xuất

Desktop dùng hai vùng chính: sidebar rộng 232–248px và hội thoại chiếm phần còn lại. Sidebar chứa workspace, tìm kiếm, Chưa đọc, Bản nháp, kênh và tin nhắn trực tiếp. Tài khoản và cài đặt nằm cuối sidebar.

Tiêu đề hội thoại cao khoảng 56px, gồm tên, mô tả ngắn nếu có và ba điểm truy cập: tìm trong hội thoại, thành viên, menu thêm. Ghim và tệp nằm trong thông tin kênh. Tìm toàn cục vẫn truy cập được từ sidebar và phím tắt, kể cả khi đang mở chủ đề.

Dòng chat có nhịp khoảng cách ổn định: avatar 32px, tên rõ, giờ là thông tin phụ. Thanh hành động xuất hiện khi hover hoặc focus; điện thoại dùng menu dễ chạm. Trạng thái chưa đọc phải có cả chữ hoặc dấu chỉ báo, không chỉ dựa vào màu.

Panel chủ đề chỉ mở theo yêu cầu, rộng khoảng 360–420px khi đủ chỗ. Nếu vùng chat bị hẹp, dùng màn chủ đề riêng với nút quay lại có tên kênh. Không cố nhồi ba cột trên màn nhỏ.

Composer nằm trong luồng bố cục phía dưới danh sách tin, không đè lên tin cuối. Chiều cao ban đầu khoảng 56–64px, tự giãn theo nội dung đến giới hạn rồi cuộn bên trong. Chỉ có một viền nhẹ, tăng độ rõ khi focus. Hướng dẫn Enter và Shift+Enter xuất hiện khi cần, không tạo thêm một thanh trợ giúp thường trực.

Mobile dùng một cột; sidebar thành drawer. Vùng chạm mục tiêu ít nhất 44×44px cho thao tác chính. Composer phải thích ứng bàn phím ảo và safe area; cần kiểm tra thực tế trước khi nghiệm thu.

## Theme đề xuất

Chọn nền trắng ấm và điểm nhấn xanh teal. Màu tươi nằm ở trạng thái tương tác, không phủ lên toàn sidebar. Dùng bề mặt phẳng và khoảng trắng để phân vùng; hạn chế bóng đổ và khung bo tròn lồng nhau.

| Vai trò | Giá trị khởi điểm |
| --- | --- |
| Nền ứng dụng | `#F6F8FA` |
| Nền hội thoại và composer | `#FFFFFF` |
| Nền sidebar | `#F1F5F5` |
| Chữ chính | `#18232B` |
| Chữ phụ | `#586772` |
| Viền nhẹ | `#DFE6EA` |
| Màu hành động và liên kết | `#0F766E` |
| Hover hành động | `#115E59` |
| Nền mục được chọn | `#E0F2EE` |
| Chữ mục được chọn | `#115E59` |
| Nền đề cập | `#FFF4D6` |
| Lỗi | `#B42318` |

Các màu là đề xuất ban đầu, cần đo tương phản trong từng cặp nền và chữ khi triển khai. Mục được chọn dùng nền nhẹ, chữ đậm vừa và chỉ báo rõ; không chỉ đổi màu chữ. Chế độ tương phản cao và theme người dùng cần tiếp tục hoạt động.

Typography: ưu tiên font hệ thống hỗ trợ tiếng Việt, nội dung chat 15px với line-height khoảng 1.5, metadata 12–13px, tiêu đề 16–18px. Tránh chữ siêu nhạt, quá nhiều chữ in hoa và quá nhiều cấp kích thước. Dùng nét icon nhất quán. Bo góc khoảng 8px cho điều khiển, 10–12px cho composer và panel nổi; không bo mọi vùng thành thẻ.

Khi triển khai, ánh xạ palette vào semantic theme variables đang có và dùng foundation tokens của Compass cho khoảng cách, radius, typography. Không hardcode màu rải rác hoặc thay màu nội dung do người dùng tạo.

## Các vấn đề chức năng và nội dung cần giữ trong backlog

| Ưu tiên | Phát hiện | Hướng xử lý |
| --- | --- | --- |
| P1 | Thông báo có tên hỗ trợ truy cập “joined the team” nhưng phần hiển thị ghi “rời nhóm” | Đối chiếu sự kiện gốc, sửa bản dịch và kiểm tra cả tham gia lẫn rời nhóm |
| P1 | AI lộ tên công cụ, ID kênh, Accept/Reject và Used 95 tools | Diễn đạt quyền bằng ngôn ngữ người dùng, ví dụ “Cho phép đọc 20 tin nhắn trong Town Square?”; đưa thông số vào Chi tiết và vẫn giữ quyết định cấp quyền rõ ràng |
| P1 | Nút mobile có vùng hiển thị 28–32px | Tăng vùng chạm, kiểm tra khoảng cách giữa các nút |
| P1 | Có nút không tên hoặc tên chỉ là ký tự icon trong accessibility tree | Thêm accessible name mô tả đúng hành động, tooltip và focus nhìn thấy được |
| P2 | DRAFT, bộ lọc tìm kiếm và nhiều phần AI còn tiếng Anh | Thống nhất tiếng Việt; bộ lọc hiển thị tên dễ hiểu nhưng giữ cú pháp truy vấn tương thích |
| P2 | Ô đăng nhập dùng placeholder, label liên kết rỗng | Thêm label cố định, autocomplete phù hợp và kiểm tra trình đọc màn hình |
| P2 | Bản nháp thiếu hành động tiếp tục nổi bật | Hiện người nhận, preview, thời điểm lưu và hành động “Tiếp tục soạn”; dùng “Tin nhắn” thay cho “thư” |
| P3 | Bước chọn ứng dụng desktop hoặc browser trước đăng nhập | Với đường dẫn web, ưu tiên vào web; giới thiệu desktop bằng lời mời nhỏ, nhớ lựa chọn |

## Thứ tự thực hiện

1. Sửa nội dung sai nghĩa, tên nút và vùng chạm trước.
2. Thu gọn composer, đưa hành động trả lời về gần tin và gom tin cùng người.
3. Áp dụng theme sáng cùng typography và trạng thái chọn mới.
4. Gộp navigation, thu rail tiện ích và cải tiến tìm kiếm.
5. Điều chỉnh cách hiển thị chủ đề. Đây là thay đổi hành vi: cần giữ khả năng hiện phản hồi quan trọng trong kênh và kiểm tra cùng người dùng trước khi đổi mặc định.

Tiêu chí đánh giá: người mới tìm đúng kênh và gửi tin không cần hướng dẫn; nhận biết ngay đang trả lời kênh hay chủ đề; thao tác chính dùng được bằng bàn phím; không có nội dung bị composer che; không tràn ngang ở 390px; đọc được nội dung dài trên desktop; mọi trạng thái đều đủ tương phản. Đo thời gian tìm tin và số lần bấm nhầm trước và sau thay đổi thay vì chỉ đánh giá bằng cảm giác đẹp hơn.

## Đã triển khai đợt đầu

- Cập nhật Quartz sang bảng màu sáng teal, chữ chính đậm và nền sidebar nhẹ. Các theme khác và màu tùy chỉnh tiếp tục giữ lựa chọn riêng.
- Mặc định thu gọn thanh định dạng khi chưa có preference; tôn trọng lựa chọn đã lưu. Giữ AI trong hàng thao tác khi thu gọn.
- Composer thu gọn trên desktop có chiều cao tối thiểu 86px sau điều chỉnh căn giữa; viền mảnh và focus teal theo theme.
- Nút định dạng có aria-expanded, icon trang trí không bị đọc lặp.
- Nút thao tác composer trên mobile có chiều cao 44px; các nút icon chính rộng 44px. Khi mở định dạng, toolbar và hành động gửi nằm ở hai hàng riêng để tránh chồng lấn.
- Giới hạn các đoạn văn, danh sách và trích dẫn trực tiếp trong nội dung tin ở 80ch; bảng và mã không bị giới hạn bởi quy tắc này.

Kiểm tra: 51 test đạt giữa bộ advanced text editor và theme utilities; ESLint không có lỗi (component editor có 3 cảnh báo sẵn có); Stylelint và git diff --check đạt. Orca xác nhận bản đang chạy dùng màu teal, composer desktop tối thiểu 86px, mobile 390×844 không tràn ngang và toolbar mở không chồng nút hành động. Chưa kiểm tra bàn phím ảo trên thiết bị thật.

Các thay đổi navigation, rail tiện ích, bố cục chủ đề, nhãn tìm kiếm, bản dịch sai nghĩa và trình bày quyền AI vẫn nằm trong backlog; đợt đầu chưa xử lý các phần này.

## Điều chỉnh căn giữa composer

Desktop dùng bố cục flex cho vùng nhập và hàng nút thay vì đặt nút nổi bằng tọa độ. Chiều cao tối thiểu 86px; một dòng và hàng nút cùng căn giữa. Hai dòng nằm giữa vùng nhập, vì vậy dòng đầu tự dịch lên khoảng 10px; nội dung dài làm khung giãn thêm. Mobile đặt hàng nút dưới vùng nhập để chữ giữ đủ chiều rộng. Các trạng thái có định dạng, tệp đính kèm, nhãn ưu tiên hoặc xem trước tiếp tục dùng bố cục nhiều hàng.

## WYSIWYG mặc định

Bật mặc định cờ WysiwygEditor phía server và lựa chọn rich text phía webapp. Với người chưa lưu lựa chọn editor, Markdown được hiển thị định dạng ngay trong ô nhập; không hiện nút preview. Lựa chọn Markdown thuần đã lưu và cờ tắt phía server vẫn được tôn trọng. Nhãn cài đặt Markdown tiếng Anh và tiếng Việt không còn ghi là mặc định.

Bản chạy đã được khởi động lại với mặc định mới. Orca xác nhận editor dùng contenteditable và chuyển Markdown dán vào thành chữ đậm, chữ nghiêng; nội dung kiểm tra đã xóa và không gửi tin nhắn.

## Nút gạch chân

Thêm nút U cạnh B và I trong toolbar WYSIWYG, có nhãn tiếng Việt và trạng thái aria-pressed. Trên màn hẹp, định dạng này chuyển vào menu mở rộng cùng các nút còn lại. Gạch chân được lưu bằng thẻ u không có thuộc tính; phần hiển thị chỉ cho phép đúng thẻ mở và đóng này, mọi HTML khác vẫn được escape. Test kiểm tra lưu, mở lại, tắt gạch chân, hiển thị tin, bỏ định dạng trong plain text và không cho phép thuộc tính HTML. Orca xác nhận nút tạo chữ gạch chân và báo trạng thái đang bật; nội dung kiểm tra đã được xóa, không gửi tin.
