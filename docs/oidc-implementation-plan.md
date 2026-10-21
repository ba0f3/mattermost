# Kế hoạch triển khai OIDC cho Mattermost OSS

Ngày 21/10/2026. Phạm vi: package OIDC và luồng đăng nhập web trong fork r2d-ai/mattermost. Bản gốc để đối chiếu: commit 29845a7a189b82f4f510eb2e18fdb1cc598bcdf0 trên master. Đây là đặc tả triển khai; chưa phải mã đã implement hoặc kết quả chạy test.

Đường dẫn đề xuất khi đưa tài liệu vào repository: docs/oidc-implementation-plan.md.

## 1. Quyết định và điều kiện hoàn thành

Bản đầu cung cấp OIDC Authorization Code với PKCE cho Keycloak, chạy hoàn toàn từ source công khai, tạo user và session Mattermost chuẩn. Web là phạm vi bắt buộc. Mobile và cơ chế chuyển phiên sang ứng dụng desktop là phần mở rộng, không chặn bản đầu.

Các quyết định:

- Đặt protocol package tại server/platform/services/oidc. Package không import channels/app, channels/web hoặc einterfaces.
- Dùng adapter tại channels/app và HTTP handlers tại channels/web. Không mở rộng interface OAuthProvider cũ chỉ để nhét thêm transaction OIDC.
- Giữ tên auth service openid và các URL web quen thuộc. Không yêu cầu RegisterOAuthProvider cho luồng mới.
- Giữ section cấu hình OpenIdSettings; tạo kiểu Go chuyên biệt, phẳng, tương thích các tên field/env hiện tại.
- Dùng Tokens hiện có cho transaction tạm, có TTL 5 phút và ConsumeOnce. Tạo một bảng OIDCIdentities riêng để lưu issuer/sub phục vụ liên kết danh tính nội bộ, bao gồm Plane sau này.
- Keycloak token chỉ dùng trong callback. Browser và các API Mattermost sử dụng session Mattermost.
- Không tự động liên kết tài khoản chỉ vì trùng email. Có command chuyển tài khoản local sang OIDC bằng user ID và subject rõ ràng.
- Build/CI không có repository enterprise, không tag enterprise, không cần license.

### 1.1 Phạm vi

| Bắt buộc trong v1 | Để sau v1 |
| --- | --- |
| Login và JIT trên trình duyệt | Mobile SSO và native desktop handoff |
| Discovery, JWKS, code exchange, PKCE, state, nonce | Nhiều issuer/provider đồng thời |
| Email đã được IdP xác minh, identity ổn định | Email auto-link, group/team/role synchronization |
| Cập nhật profile có giới hạn | RP-initiated logout và backchannel logout |
| Chuyển tài khoản local có sẵn qua CLI quản trị | SCIM, Keycloak admin event synchronization |
| Config JSON/env, nút login, thông báo lỗi | UI cấu hình System Console đầy đủ |
| Session/logout Mattermost, audit tối thiểu | Lấy token và gọi API Plane |
| Test package, store, HTTP, E2E Keycloak, build OSS | Bảo đảm đăng nhập bằng ứng dụng mobile |

V1 hỗ trợ một Keycloak issuer và một confidential client. Với Keycloak được kiểm soát bởi công ty, yêu cầu ID token chứa đủ email/profile và email_verified=true. Không thêm đường nhận email chưa xác minh trong v1.

## 2. Căn cứ từ source và tác động đến thiết kế

| Source đã đối chiếu | Phát hiện | Tác động |
| --- | --- | --- |
| server/einterfaces/oauthproviders.go | Hook GetUserFromIdToken không có transaction/expected nonce/verifier rõ ràng | Dùng adapter mới, không sửa toàn bộ flow OAuth cũ |
| server/channels/app/app.go | App tồn tại theo từng request | Runtime cache discovery/JWKS đặt trên Server |
| server/channels/store/sqlstore/tokens_store.go | ConsumeOnce dùng DELETE RETURNING trên primary | Tái sử dụng, bảo đảm callback chỉ được xử lý một lần |
| server/public/model/token.go | Token key đúng 64 ký tự, expiry mặc định 48 giờ | State hash 64 hex và TTL OIDC riêng 5 phút |
| 000028_create_tokens.up.sql | Extra giới hạn 2048 | Không lưu token sets, full config hoặc context không giới hạn |
| 000046_create_users.up.sql | AuthData VARCHAR(128) UNIQUE | Dùng identity key 72 ký tự |
| server/channels/store/sqlstore/user_store.go | GetByAuth đọc replica | Thêm primary-read cho resolve và xử lý race |
| server/channels/app/user.go | UpdateOAuthUserAttrs có đoạn đặt DeleteAt về 0 | Tuyệt đối không dùng đường này để sync user OIDC |
| server/channels/app/users/users.go | User đầu tiên được tự cấp system_admin | JIT phải chặn first-user bootstrap hoặc chủ động loại hành vi này |
| server/channels/app/login.go | DoLogin tạo session, không tự kiểm tra toàn bộ trạng thái user | Adapter kiểm tra user trước khi gọi |
| server/config/client.go | Flag OpenID cho client nằm trong nhánh license | Công bố capability dựa vào implementation OSS và config |
| .cursor/environment.json | Có repositoryDependencies tới enterprise | Sửa môi trường agent và CI |

Các source được liên kết trong mục 16. Tên file mới, API và command trong các mục sau là thiết kế đề xuất.

## 3. Kiến trúc và ranh giới package

### 3.1 Protocol package

Thư mục server/platform/services/oidc gồm:

| File mới | Trách nhiệm |
| --- | --- |
| config.go | Config nội bộ đã được normalize về kiểu dữ liệu, validate issuer/redirect/scopes, fingerprint |
| client.go | Client bất biến cho một config snapshot, xây authorization URL và đổi code |
| discovery.go | Discovery, kiểm tra metadata endpoint, cache/singleflight |
| verify.go | ID-token verification, nonce, audience/azp, subject và profile validation |
| identity.go | VerifiedIdentity và hàm identity key thuần |
| errors.go | Mã lỗi ổn định, dữ liệu an toàn cho logging/UI |
| client_test.go, verify_test.go, identity_test.go | Test bằng OIDC server giả lập với signing keys thật |

Package không biết Mattermost user, channel, role, cookie HTTP hoặc SQL. Không để các DTO của package chứa token raw khi trả về tầng app.

API minh họa:

~~~go
type VerifiedIdentity struct {
    Issuer            string
    Subject           string
    Email             string
    EmailVerified     bool
    PreferredUsername string
    GivenName         string
    FamilyName        string
}

type AuthorizationInput struct {
    State        string
    Nonce        string
    CodeVerifier string
}

type CallbackInput struct {
    Code          string
    ExpectedNonce string
    CodeVerifier  string
}

func NewClient(ctx context.Context, cfg Config, httpClient *http.Client) (*Client, error)
func (c *Client) AuthorizationURL(in AuthorizationInput) (string, error)
func (c *Client) ExchangeAndVerify(ctx context.Context, in CallbackInput) (VerifiedIdentity, error)
func IdentityKey(issuer, subject string) string
~~~

Đây là hợp đồng để triển khai, không phải đoạn code có thể compile ngay.

### 3.2 Adapter Mattermost

Các file mới:

- channels/app/oidc.go: điều phối BeginOIDCLogin và CompleteOIDCLogin.
- channels/app/oidc_runtime.go: manager trên Server, config listener, lifetime của client/cache.
- channels/app/oidc_transactions.go: adapter Tokens, state, TTL, serialized transaction.
- channels/app/oidc_identity.go: resolve/JIT/profile sync, kiểm tra trạng thái user.
- channels/app/oidc_link.go: chuyển tài khoản local sang OIDC và invalidation/revoke.
- channels/web/oidc.go: đọc request/cookie, redirect, error response và session cookies.

API app dự kiến:

~~~go
BeginOIDCLogin(rctx request.CTX, in OIDCLoginInput) (OIDCLoginStart, *model.AppError)
CompleteOIDCLogin(rctx request.CTX, in OIDCCallbackInput) (OIDCLoginResult, *model.AppError)
ResolveOIDCUser(rctx request.CTX, identity oidc.VerifiedIdentity) (*model.User, *model.AppError)
LinkUserToOIDC(rctx request.CTX, userID string, in OIDCLinkInput) (OIDCLinkResult, *model.AppError)
~~~

Web handler gọi DoLogin với IsOAuthUser=true và AttachSessionCookies sau khi app đã resolve user. Không tự tạo session/token/cookie có cấu trúc mới.

Store API mới sử dụng request.CTX theo server/AGENTS.md. context.Context chỉ dùng ở protocol library và phần thực hiện HTTP.

### 3.3 Runtime

Server giữ OIDCManager. App chỉ sử dụng manager đó.

- Khởi tạo sau config, store và HTTP service.
- Startup chỉ validate cấu trúc config. Keycloak đang down không làm cả Mattermost không khởi động.
- Discovery chạy lazy với timeout; cache theo immutable config fingerprint, singleflight để chống hàng loạt request cùng discover.
- Không tạo goroutine vô hạn hoặc retry loop nền. Discovery lỗi được trả thành lỗi đăng nhập có thể thử lại; cache lỗi ngắn, ví dụ 5 giây, để giảm tải.
- Config listener tạo generation/fingerprint mới và bỏ runtime không còn hợp lệ. Fingerprint có các giá trị ảnh hưởng xác thực, gồm issuer, client ID/secret, redirect URI và policy; không log nội dung/secret.
- Callback của generation cũ bị từ chối với config_changed, user bắt đầu login lại.
- Kiểm tra generation/Enable lại sau HTTP exchange, trước mutation user và trước DoLogin. Trước khi gửi credential/session cookie cho browser, dùng một gate ngắn đồng bộ với config listener để xác nhận generation vẫn hoạt động. Không giữ gate trong HTTP call hoặc plugin hooks. Nếu DoLogin vừa tạo session nhưng generation đã đổi, revoke session mới, xóa HeaderToken và các cookie chuẩn bị gửi, trả config_changed. Generation switch và việc công bố session có thứ tự rõ ràng; session đã công bố trước switch vẫn theo policy session hiện có.
- Tắt Enable chặn start/callback mới. Session Mattermost đã tồn tại không tự bị revoke bởi việc tắt config.
- Shutdown tháo listener, hủy tác vụ của manager và đóng connection do manager sở hữu; không đóng shared HTTP client của service khác.

## 4. Cấu hình

### 4.1 Một section duy nhất

Giữ OpenIdSettings và đổi kiểu Go từ SSOSettings thành OpenIDSettings chuyên biệt, các field vẫn phẳng. Giữ nguyên các tên cũ gồm Enable, Id, Secret, Scope, DiscoveryEndpoint, AuthEndpoint, TokenEndpoint, UserAPIEndpoint, ButtonText, ButtonColor, UsePreferredUsername.

Bổ sung tối thiểu:

| Field mới | Ý nghĩa | Mặc định |
| --- | --- | --- |
| Issuer | Issuer chính xác của realm | Rỗng; bắt buộc khi Enable=true |
| EnableJIT | Cho phép tạo user từ identity mới | false; ví dụ triển khai nội bộ chủ động bật |
| DevAllowHTTPLoopback | Cho phép fixture HTTP khi cả issuer và SiteURL là địa chỉ loopback literal | false; env MM_OPENIDSETTINGS_DEVALLOWHTTPLOOPBACK |

Các giá trị protocol v1 được cố định trong implementation: PKCE S256 bật, transaction TTL 5 phút, yêu cầu email_verified=true, không email auto-link, scopes openid profile email, signing algorithm RS256. Đây là lựa chọn của dự án để giới hạn phạm vi Keycloak; không tuyên bố mọi OIDC provider đều dùng cùng cấu hình.

Ví dụ cấu hình đề xuất:

~~~json
{
  "ServiceSettings": {
    "SiteURL": "https://chat.example.internal"
  },
  "OpenIdSettings": {
    "Enable": true,
    "Issuer": "https://login.example.internal/realms/company",
    "Id": "mattermost-web",
    "Scope": "openid profile email",
    "EnableJIT": true,
    "UsePreferredUsername": true,
    "ButtonText": "Đăng nhập công ty"
  }
}
~~~

Secret được cung cấp bằng env MM_OPENIDSETTINGS_SECRET hoặc cơ chế quản lý cấu hình hiện có; không commit giá trị thật.

### 4.2 Compatibility và validation

- Giữ tên JSON/env MM_OPENIDSETTINGS_*; thêm test đọc config file và env.
- Dùng OpenIDSettings.SSOSettings() làm compatibility projection cho GetSSOService và code hỗ trợ cần kiểu cũ. Không mở rộng shared SSOSettings bằng mọi policy OIDC.
- Issuer là giá trị chuẩn được cấu hình. Không lấy issuer từ JWT chưa verify.
- Với v1, DiscoveryEndpoint được derive từ Issuer. Nếu field cũ được đặt, nó phải khớp discovery URL đã derive; cấu hình khác bị từ chối kèm hướng dẫn sửa.
- Endpoint metadata được lưu trong runtime snapshot, không ghi ngược vào config trong request login. Các endpoint thủ công cũ nếu có phải được kiểm tra tương thích; không âm thầm trộn issuer/endpoints.
- Require SiteURL tuyệt đối, canonical; redirect URI = SiteURL đã xử lý subpath + /signup/openid/complete. Không lấy Host hay forwarded host tùy ý để xây redirect.
- DevAllowHTTPLoopback=true chỉ hợp lệ nếu cả issuer và SiteURL dùng http với hostname literal 127.0.0.1 hoặc ::1; metadata endpoints cũng phải ở expected loopback origin. Không chấp nhận hostname tùy ý, IP private hay production HTTPS trộn HTTP. Chế độ này dùng cookie HttpOnly/SameSite=Lax/Path=/ nhưng Secure=false và tên không có __Host-. Mọi cấu hình khác bắt buộc HTTPS và cookie Secure.
- Issuer không có query, fragment hoặc userinfo; prod yêu cầu HTTPS. So sánh exact issuer, không lower-case, xóa slash hoặc đổi realm path.
- Thêm validation cho độ dài issuer/subject/return path và tất cả trường transaction.
- Secret masking, placeholder restoration, support packet và telemetry phải giữ secret kín khi đổi type Go.
- Frontend type OpenIDSettings và default config phải đồng bộ với backend.

System Console đầy đủ là v1.1. Nếu giữ màn cũ trước khi bổ sung UI, cần làm rõ các giá trị mới được quản lý qua config; không để thao tác Save màn cũ ghi mất Issuer/EnableJIT.

## 5. Luồng đăng nhập web

### 5.1 Routes

Đăng ký route chính xác trước route OAuth tổng quát:

| Route | Xử lý v1 |
| --- | --- |
| GET /oauth/openid/login | Bắt đầu đăng nhập |
| GET /oauth/openid/signup | Cùng flow, JIT bị policy kiểm soát |
| GET /signup/openid/complete | Callback duy nhất đăng ký trong Keycloak |
| GET /oauth/openid/complete | Chỉ giữ alias nếu có nhu cầu tương thích đã xác minh; không tạo redirect URI thứ hai mặc định |
| /oauth/openid/mobile_login | Trả lỗi không hỗ trợ có hướng dẫn dùng web |

Thêm guard trong handler OAuth cũ: service=openid không được rơi xuống provider Enterprise khi một URL chưa được handler mới xử lý. Không sửa hành vi GitLab/Google/Office365 ngoài những thay đổi type compatibility cần thiết.

Không xử lý invite-based enrollment, desktop_token hoặc mobile handoff trong v1. Return path tới channel có thể giữ; authorization vào channel vẫn do Mattermost kiểm tra.

### 5.2 Bắt đầu

1. Kiểm tra feature được bật, config hợp lệ, không nhận return_to bên ngoài site.
2. Lấy client runtime theo config hiện tại.
3. Sinh state và nonce độc lập bằng crypto/rand, ít nhất 32 byte entropy; verifier dùng oauth2.GenerateVerifier.
4. Sinh secret gắn với browser, độc lập với state.
5. Lưu transaction Tokens: key là SHA256(state) ở dạng 64 hex, type oidc_login.
6. Set cookie gắn với transaction. Production dùng prefix __Host-, suffix chứa instance/site identifier và digest state; value là browser secret, HttpOnly, Secure, SameSite=Lax, Path=/, không Domain; TTL 5 phút. Path=/ bao gồm cả start và callback dù Mattermost chạy dưới subpath. Fixture HTTP development dùng tên riêng không có __Host-; production không hạ yêu cầu này.
7. Redirect tới authorization endpoint với response_type=code, state, nonce, PKCE S256, client_id, scopes và redirect_uri cố định.

Cookie mỗi transaction cho phép hai tab đăng nhập đồng thời. Giới hạn 5 cookie transaction đang chờ trên một browser/instance; start handler nhìn được các cookie nhờ Path=/. Nếu đã đủ 5, từ chối tạo thêm và hướng dẫn hoàn tất một tab đang mở hoặc chờ transaction hết hạn. Callback luôn xóa cookie tương ứng khi transaction được xử lý. Áp dụng rate limit cho start/callback theo cơ chế của server.

V1 dùng query response mode với top-level GET callback. form_post không nằm trong phạm vi vì cookie SameSite cần xử lý khác.

### 5.3 Callback

1. Đọc tham số với giới hạn kích thước; từ chối tham số nhạy cảm trùng nhiều giá trị. Callback thành công cần state+code và không có error; callback từ chối cần state+error và không có code. Không yêu cầu code cho trường hợp từ chối, không chấp nhận đồng thời code và error.
2. Lấy đúng cookie theo state. Không có cookie thì fail, không fallback.
3. Đọc transaction bằng GetByTokenFromMaster mới; kiểm tra type, JSON, TTL, browser-binding hash và config fingerprint trước khi consume. Sai binding không được xóa transaction của browser khác.
4. ConsumeOnce(type=oidc_login, key=SHA256(state)) trên primary. Chỉ một callback thắng. Kiểm tra lại record trả về và config hiện tại trước khi tiếp tục. Callback fail sau bước consume phải bắt đầu transaction mới; không đưa transaction về trạng thái dùng lại.
5. Với error từ Keycloak, xử lý sau state/binding/consume rồi trả lỗi đã sanitize, không đổi code.
6. Đổi code với verifier trên server bằng HTTP client đã kiểm soát.
7. Verify ID token và các expected claims. Kiểm tra lại generation/Enable sau khi network call hoàn tất.
8. Resolve/sync/JIT user và private identity mapping sau khi xác nhận generation vẫn hiện hành.
9. Kiểm tra lại trạng thái user từ nguồn có thẩm quyền trước khi cấp session; không nhận user disabled, bot hoặc remote.
10. Kiểm tra generation trước DoLogin với IsOAuthUser=true; sau khi có session dùng publication gate ở mục 3.3, cập nhật request session context và AttachSessionCookies. Generation hết hiệu lực thì revoke session mới và không gửi credential.
11. Xóa cookie transaction, trả redirect về local return path đã lưu hoặc trang mặc định.
12. Callback/error response có Cache-Control: no-store và Referrer-Policy: no-referrer. Không đưa code/token/error_description raw vào URL lỗi hoặc UI.

Không retry token exchange một cách mù quáng khi timeout: authorization code có thể đã bị provider dùng. Trả thông báo bắt đầu login mới.

### 5.4 Return path

Chỉ chấp nhận relative path trong SiteURL/subpath. Từ chối absolute URL, scheme-relative //, backslash, control characters, encoded bypass và đường dẫn thoát subpath. Không làm URL-decode nhiều lần không nhất quán giữa validation và redirect. Giá trị được validate một lần lúc bắt đầu và lưu server-side; callback không thay thế từ query.

## 6. Xác minh token và HTTP

### 6.1 Protocol checks

Dùng coreos/go-oidc cho discovery/JWKS, chữ ký, issuer, audience và expiry. Dùng golang.org/x/oauth2 cho code exchange/PKCE. Pin version tương thích với Go/module của fork.

Application bổ sung các kiểm tra mà thư viện không tự hoàn tất:

- Nonce phải tồn tại và bằng expected nonce của transaction.
- Subject không rỗng, ASCII, tối đa 255 ký tự, không normalize/case-fold.
- Issuer phải bằng exact configured issuer.
- V1 chỉ nhận aud là client ID được cấu hình, dạng string hoặc array chỉ có một phần tử đó. Audience bổ sung chưa được tin cậy bị từ chối ngay cả khi azp đúng. Nếu azp có mặt, nó phải bằng client ID.
- ID token thiếu, sai type claim hoặc token không có cấu trúc hợp lệ đều fail.
- email phải hợp lệ và email_verified phải là boolean true trước mọi create/update/login.
- Không dùng access token hoặc JWT chưa verify như bằng chứng danh tính.
- Không bật SkipIssuerCheck, SkipClientIDCheck, SkipExpiryCheck hoặc bỏ signature validation.

V1 yêu cầu các profile claim cần thiết xuất hiện trong ID token và không gọi UserInfo. Điều này làm luồng Keycloak ngắn và nhất quán. UserInfo có thể bổ sung ở v1.1; nếu bổ sung phải kiểm tra subject bằng đúng subject ID token trước khi dùng dữ liệu.

Tài liệu go-oidc xác định nonce là trách nhiệm của caller. Quy tắc issuer/sub và UserInfo theo OIDC Core; PKCE cho confidential client được RFC 9700 khuyến nghị. [S11][S12][S13]

### 6.2 HTTP client

Dùng cùng một controlled HTTP client cho discovery, JWKS và code exchange. Không để thư viện dùng http.DefaultClient.

- Giữ DNS/dial filtering và allowlist mạng nội bộ của Mattermost HTTPService.
- Cho phép Keycloak nội bộ bằng hostname trần hoặc CIDR hẹp trong AllowedUntrustedInternalConnections. HTTPService không dùng cú pháp host:port cho allowlist này; scheme/host/port được kiểm soát riêng bằng origin validation của OIDC. Không cấm toàn bộ địa chỉ private rồi buộc vận hành tắt bảo vệ.
- TLS certificate verification luôn bật cho OIDC production, kể cả global EnableInsecureOutgoingConnections được bật cho dịch vụ khác.
- Nếu cần, bổ sung option vào HTTP service để tạo client với TLS bắt buộc; không mutate shared Transport của request khác.
- CA nội bộ được thêm vào trust store của runtime/container. Không dùng InsecureSkipVerify.
- Đề xuất timeout 10 giây cho từng HTTP call và deadline tổng cho callback; giới hạn response, ví dụ 1 MiB metadata/JWKS/token response.
- Với Keycloak v1, metadata endpoints phải thuộc issuer origin được cấu hình. Provider dùng endpoint khác origin nằm ngoài phạm vi.
- Không theo redirect sang host/origin khác; token POST không theo redirect ngoài expected endpoint.
- Kiểm tra cả discovery metadata và request cuối cùng để tránh bypass qua URL/redirect.
- Development HTTP chỉ theo contract DevAllowHTTPLoopback ở mục 4; flag này không thể áp dụng cho host production hoặc toàn bộ mạng private.

## 7. Dữ liệu và transaction

### 7.1 Login transaction trên Tokens

Record mới:

~~~text
Token = hex(SHA256(state))
Type = oidc_login
CreateAt = thời điểm bắt đầu
Extra = JSON {
  version,
  browser_binding_hash,
  nonce,
  code_verifier,
  config_fingerprint,
  return_to
}
~~~

TTL cố định 5 phút dựa trên CreateAt; thêm TokenTypeOIDCLogin và expiry tương ứng hoặc enforce trong adapter với test chống default 48 giờ. Độ dài JSON UTF-8 tối đa 2048 byte, return_to giới hạn 512 byte; record quá dài bị từ chối trước insert.

Không lưu issuer dài/secret/full config/JWT vào Extra; fingerprint tham chiếu snapshot hiện tại. Cleanup chỉ xóa oidc_login đã hết hạn, không đổi global cleanup gây xóa token khác sớm. ConsumeOnce đã xóa record trong callback thành công hoặc thất bại sau consume. Verifier là dữ liệu nhạy cảm tạm thời trong database; không ghi log, giữ TTL ngắn và cleanup.

### 7.2 Identity key trong Users

~~~text
identity_key = "oidc:v1:" + hex(SHA256(JSON([exact_issuer, exact_subject])))
Users.AuthService = "openid"
Users.AuthData = identity_key
~~~

JSON array được serialize theo một hàm duy nhất và khóa encoding bằng test vector. Identity key dài 72 ký tự, phù hợp AuthData 128. Không nối bằng ký tự phân cách mà có thể nhập nhằng.

### 7.3 Private identity mapping

Bảng đề xuất:

~~~sql
CREATE TABLE OIDCIdentities (
    IdentityKey VARCHAR(72) PRIMARY KEY,
    UserId VARCHAR(26) NOT NULL UNIQUE,
    Issuer VARCHAR(1024) NOT NULL,
    Subject VARCHAR(255) NOT NULL,
    CreatedAt BIGINT NOT NULL,
    UpdatedAt BIGINT NOT NULL
);
~~~

DDL trên thể hiện schema logic. Tên/case và số migration phải theo conventions và migration mới nhất của branch khi implement; không sửa migration lịch sử.

- IdentityKey là primary key, UserId unique: v1 một identity OIDC cho mỗi Mattermost user.
- Verify key được tính từ exact tuple trước ghi và khi đọc. Nếu key trùng nhưng tuple khác, fail cứng.
- Mapping và Users.AuthData phải trỏ đúng cùng user. Không tự sửa bằng email nếu không khớp.
- API Users/Props, client config và plugin public profile không chứa bảng này.
- Giữ mapping khi soft disable. Permanent user deletion phải xóa mapping qua existing deletion flow hoặc FK cascade đã kiểm thử.
- Tạo store OIDCIdentityStore riêng với Ensure/Find/GetForUser và luồng bind atomic quản trị.
- Cung cấp internal accessor theo user ID cho tích hợp sau; việc mở API cho Plane cần quyền riêng. Lưu identity không đồng nghĩa có token hoặc quyền truy cập Plane. Khi liên kết hai client cần kiểm tra cấu hình subject của Keycloak: pairwise subject có thể khác nhau giữa Mattermost và Plane; nếu khác phải dùng mapping hoặc một common-ID claim được kiểm soát.

Bảng này có chủ đích giữ raw issuer/sub sau login. Chỉ lưu hash trong Users.AuthData sẽ không lấy lại được subject để link tài khoản Plane sau này.

## 8. Resolve user, JIT và profile

### 8.1 Resolve

1. Chỉ nhận VerifiedIdentity từ protocol client.
2. Tính identity key, lookup mapping và native user trên primary.
3. Mapping và user khớp thì dùng user đó.
4. Có user với đúng openid/key nhưng thiếu mapping: Ensure mapping sau khi đã verify identity; đây là recovery sau crash.
5. Mapping tồn tại nhưng native user/authservice/key khác: báo identity_conflict, không auto-rebind.
6. Không có mapping/user và JIT tắt: báo account_not_provisioned.
7. Nếu JIT bật, chạy policy và tạo user qua native CreateUser.

### 8.2 Tạo user

- Require email_verified=true và email hợp lệ.
- Tuân thủ EnableUserCreation, domain restriction và user limit hiện có. Realm/domain được tin cậy không bỏ qua chính sách Mattermost.
- Không nhận quyền admin/guest/team từ token trong v1.
- Thêm guard để JIT không trở thành first-user bootstrap. Nếu instance chưa có bootstrap account, trả lỗi rõ ràng. Có thể triển khai guard ngay trong nhánh native first-user promotion cho nguồn OIDC để tránh race/check bên ngoài đơn thuần.
- Username lần đầu ưu tiên preferred_username nếu UsePreferredUsername=true, normalize theo quy tắc Mattermost. Nếu trùng, thêm suffix ngắn từ identity key và thử có giới hạn.
- Email đã tồn tại nhưng identity chưa link: đọc lại exact openid/key trên primary trước để nhận ra user vừa được một callback đồng thời tạo. Nếu vẫn không có cùng identity thì trả linking_required, không tạo user phụ hay auto-merge.
- Native CreateUser tạo user; EnsureIdentity hoàn tất mapping trước khi tạo session.
- Nếu crash giữa hai bước, lần callback mới resolve theo exact AuthData/service và hoàn tất mapping.
- Với bất kỳ lỗi unique AuthData, email hoặc username, đọc lại exact openid/key trên primary trước; các unique constraint có thể báo lỗi theo thứ tự khác nhau. Nếu đã có cùng identity thì hội tụ về user thắng race và Ensure mapping. Chỉ khi vẫn không có identity đó mới xử lý username collision hoặc email linking_required.
- Thêm typed conflict cho AuthData trong SqlUserStore.Save; không chỉ bắt lỗi username/email.

### 8.3 Cập nhật profile

Dùng hàm cập nhật chuyên biệt, chỉ viết email, EmailVerified, FirstName, LastName và UpdateAt. Username giữ nguyên sau lần tạo đầu để không làm thay đổi mentions; không sync roles, DeleteAt, auth binding, memberships hoặc local MFA.

- Đổi email vẫn cùng identity và cùng user ID.
- Email mới conflict tài khoản khác: giữ dữ liệu cũ và từ chối login với lỗi xử lý được; không tự chuyển mapping.
- Không dùng UpdateOAuthUserAttrs vì nó có hành vi reactivate.
- Không dùng generic trusted User.Update với snapshot có Roles/DeleteAt cũ, tránh ghi đè thay đổi đồng thời của admin.
- Invalidate user cache và phát event profile theo cơ chế core khi có thay đổi.
- Preflight cuối cùng kiểm tra disabled/bot/remote trước DoLogin. Test riêng việc admin disable trong khi profile đang được cập nhật; profile sync không được xóa trạng thái disable.
- MFA cho phiên SSO được áp dụng ở Keycloak. Không tự tắt MFA hoặc sửa auth policy khác của user.

## 9. Chuyển tài khoản hiện có

Command đề xuất, chưa tồn tại:

~~~bash
mmctl user link-oidc USER_ID \
  --issuer https://login.example.internal/realms/company \
  --subject KEYCLOAK_SUB \
  --dry-run
~~~

Bỏ --dry-run để thực hiện. Không thêm yêu cầu ticket/reason/approval flow riêng. Command hiện có migrate-auth chỉ phục vụ LDAP/SAML nên không dùng thay cho command này.

Server endpoint quản trị đề xuất: POST /api/v4/users/{user_id}/oidc/link. Bắt buộc quyền system admin và audit theo API hiện có; local mmctl dùng authorization cơ chế sẵn có.

### 9.1 Preconditions

- User ID tồn tại, không bot/remote, không bị xóa.
- Issuer bằng issuer đang cấu hình; subject đúng định dạng.
- Chỉ chuyển user local hoặc lặp lại đúng mapping đã áp dụng. Không thay identity khác hoặc SSO provider khác bằng command v1.
- Không tìm user qua email. Không tạo user khi ID sai. Không thay roles, profile, membership hoặc activation.
- Dry-run chỉ kiểm tra và mô tả thay đổi; thao tác thật kiểm tra lại trong transaction.

### 9.2 Ghi dữ liệu và retry

Dùng một transaction store hẹp để lock user, kiểm tra auth source bằng compare-and-set, cập nhật Users.AuthService/AuthData, xóa password như flow UpdateAuthData và insert mapping. Giữ MFA; reset failed attempts theo semantics core.

Sau commit: invalidate cache và revoke toàn bộ session cũ. Dùng lại hành vi của UpdateUserAuth, không gọi thêm lần ghi auth vô điều kiện.

Nếu revoke thất bại sau commit, báo rõ auth binding đã đổi nhưng revoke chưa hoàn tất. Retry cùng command phải chạy revoke lại ngay cả khi mapping đã tồn tại. Không báo thành công toàn bộ và không rollback về password đã bị xóa.

Đây là chuyển phương thức đăng nhập, không thêm phương thức thứ hai. User ID và dữ liệu chat giữ nguyên.

Các API/CLI auth-edit tổng quát phải phối hợp cập nhật mapping hoặc từ chối sửa trực tiếp user có binding oidc:v1. Bao gồm đường SwitchOAuthToEmail → UpdatePassword vốn có thể xóa AuthData/AuthService ngoài UpdateUserAuth. V1 chặn chuyển ngược bằng những đường này và thông báo cần luồng quản trị được hỗ trợ. Không để một command edit authdata hoặc thao tác đổi sang password tạo mapping mồ côi.

## 10. UI, lỗi và audit

### 10.1 UI web

- Nút công ty trên login và signup, dùng OpenIdButtonText/ButtonColor.
- EnableSignUpWithOpenId dựa trên config hợp lệ và implementation đã có trong OSS. Không sửa global IsLicensed hoặc unlock capability không liên quan.
- Rà signup vì có gate isLicensed riêng.
- Ẩn/disable Account Settings → Security → tự chuyển phương thức đăng nhập sang OpenID trong v1: UI này dùng legacy SwitchEmailToOAuth/getSSOProvider. Hướng dẫn dùng luồng liên kết quản trị đã định nghĩa; không để bật capability login kéo theo một màn chuyển auth bị hỏng.
- Tắt JIT vẫn cho existing linked user đăng nhập; UI phải phân biệt login và signup khi cần.
- V1 không tự redirect ngay khi vừa logout; user bấm nút SSO để bắt đầu lần mới.
- Route mobile trả lỗi hỗ trợ rõ ràng, không rơi xuống nhánh Enterprise hoặc trả session token qua URL.

### 10.2 Mã lỗi

| Mã đề xuất | Hiển thị/hành vi |
| --- | --- |
| oidc_disabled | Phương thức đăng nhập đang tắt |
| oidc_config_invalid | Admin kiểm tra cấu hình |
| oidc_provider_unavailable | Không kết nối được; thử lại sau |
| oidc_transaction_invalid | Yêu cầu không hợp lệ; bắt đầu lại |
| oidc_transaction_expired | Phiên đăng nhập đã hết hạn |
| oidc_config_changed | Cấu hình thay đổi; bắt đầu lại |
| oidc_authentication_failed | Không xác thực được tài khoản |
| oidc_email_unverified | Cần email đã xác minh trên hệ thống đăng nhập |
| oidc_linking_required | Tài khoản cần được liên kết bởi quản trị viên |
| oidc_user_disabled | Tài khoản đã vô hiệu hóa |
| oidc_identity_conflict | Binding không nhất quán; admin xử lý |
| oidc_client_unsupported | Dùng trình duyệt cho bản hiện tại |

UI có thể gộp lỗi nhạy cảm để tránh lộ tài khoản; log giữ mã phân loại và request ID. Không log secret, authorization code, state raw, nonce, verifier, JWT hoặc Keycloak response body.

Audit tối thiểu: login_success, login_failure, user_provisioned, user_linked, identity_conflict, config_changed. Ghi actor/request ID, Mattermost user ID nếu có, result code; không cần dump claims.

## 11. Logout và vòng đời phiên

V1 dùng session/logout của Mattermost.

- Sau OIDC callback, session duration áp dụng SessionLengthSSOInHours qua IsOAuthUser=true.
- Token Keycloak không được giữ để refresh phiên Mattermost.
- Logout Mattermost revoke session và clear cookie theo core; session Keycloak có thể còn.
- Lần bấm SSO tiếp theo có thể vào ngay nhờ session Keycloak; đây là hành vi local logout đã định nghĩa.
- Disable user trên Keycloak chặn xác thực mới, không được quảng cáo là thu hồi tức thì mọi session Mattermost.
- Offboarding cần thao tác deactivate/revoke phía Mattermost hiện có.
- Backchannel logout/RP-initiated logout là task riêng sau v1, không kéo vào điều kiện ship package.

## 12. File cần sửa

| Khu vực | Thay đổi |
| --- | --- |
| .cursor/environment.json, install/start scripts, README | Bỏ private repo requirement, build OSS rõ ràng |
| server/go.mod, go.sum | Pin go-oidc/oauth2 phù hợp, dùng make modules-tidy |
| server/platform/services/oidc/* | Protocol implementation và package tests |
| server/channels/app/server.go | Runtime manager/listener lifecycle |
| server/channels/app/oidc*.go | Điều phối transaction, identity, profile, linking |
| server/channels/web/oidc.go | Web routes và callback |
| server/channels/web/oauth.go | Route precedence, guard không fallback openid legacy |
| server/public/model/config.go, oidc_settings.go | Specialized flat config, default/validate/projection |
| server/public/model/token.go | oidc_login type và TTL |
| server/public/model/oidc_identity.go | Server model cho mapping |
| server/channels/store/store.go | Identity store và primary-read/profile-update contracts |
| server/channels/store/sqlstore/store.go | Field, constructor và accessor cho OIDCIdentityStore |
| server/channels/store/sqlstore/oidc_identity_store.go | Mapping, bind atomic, cleanup |
| server/channels/store/sqlstore/user_store.go | Primary auth lookup, unique-conflict recovery, profile-only update |
| server/channels/store/sqlstore/tokens_store.go | GetByTokenFromMaster trước consume; giữ ConsumeOnce atomic |
| server/channels/app/users/users.go | Guard OIDC không auto-bootstrap system_admin |
| server/channels/db/migrations/postgres/* | Migration mới cho OIDCIdentities, up/down phù hợp |
| server/config/client.go | Capability OpenID độc lập license cho implementation mới |
| webapp login/signup và config types | Nút, gate và error rendering |
| server/channels/api4/oidc.go, route init | Admin link API |
| server/cmd/mmctl/commands/* | link-oidc và dry-run |
| support packet/config sanitization callers | Type compatibility và không lộ Secret |
| e2e-tests/playwright/* | E2E web trên OSS không license |
| docs/oidc-implementation-plan.md, docs/oidc-keycloak.md | Spec và runbook |
| Docker fixture/test data mới | Keycloak test realm và hướng dẫn chạy |

Không sửa toàn bộ luồng OAuth legacy hoặc mở các feature thương mại không liên quan.

## 13. Test và tiêu chí nghiệm thu

### 13.1 Package tests

Mock OIDC server chạy HTTP test fixture riêng, phát JWT ký thật bằng key test.

- Discovery issuer sai, metadata endpoint ngoài allowlist.
- PKCE S256 được gửi đúng; token exchange nhận đúng verifier.
- Nonce sai/mất; chữ ký sai; issuer/audience/azp sai; audience bổ sung lạ dù azp đúng; token hết hạn; sub rỗng/quá dài/sai type.
- email thiếu, invalid hoặc chưa verified.
- JWKS rotation; unknown key; timeout; response quá lớn.
- IdentityKey ổn định, khác issuer hoặc sub tạo key khác.
- Không dùng config insecure hoặc http.DefaultClient ngoài policy.

### 13.2 Store/app tests

- State TTL 5 phút; state thiếu/sai; browser-binding sai; config đổi.
- Hai callback đồng thời cùng state: chỉ một lần xử lý thành công.
- Hai transaction khác nhau cùng identity: một user và một mapping, xử lý conflict bằng primary read; cover trường hợp lỗi được báo là email unique trước AuthData.
- Crash sau native user insert: callback mới phục hồi mapping đúng.
- Mapping/native auth mâu thuẫn: fail, không email recovery.
- Existing user đổi email giữ user ID; email conflict có lỗi rõ.
- User disabled không được bật lại.
- Profile update không ghi đè roles/DeleteAt đang được admin thay đổi.
- JIT không cấp admin trên instance mới.
- JIT tắt/EnableUserCreation tắt/domain restriction/user limit có hiệu lực.
- Linking dry-run, linking thật, repeat idempotent, identity đã thuộc user khác, source auth bị đổi đồng thời.
- Auth đã commit nhưng revoke lỗi: retry hoàn tất revoke.
- Permanent delete xử lý mapping; soft disable giữ mapping.
- Token cleanup không ảnh hưởng loại token khác.

### 13.3 HTTP và E2E

| Ca | Kết quả bắt buộc |
| --- | --- |
| Login user mới | Keycloak → callback → user thường → /users/me thành công |
| Login user cũ | Cùng user ID, không tạo tài khoản trùng |
| Session hoạt động | API có auth và WebSocket hoạt động |
| Local account được migrate | Giữ lịch sử/membership, session cũ bị revoke |
| Logout | Session Mattermost cũ không còn dùng được |
| Hai tab | Không vô tình ghi đè transaction của nhau |
| Callback browser khác | Không cấp session |
| Return path ác ý | Không redirect ra ngoài site/subpath |
| Keycloak down/wrong secret/bad CA | Lỗi rõ ràng, Mattermost vẫn phục vụ session đang có |
| Enable=false/config đổi giữa flow | Callback bị chặn |
| Config đổi lúc exchange/DoLogin | Check generation sau network và publication gate ngăn phát session cũ; session vừa tạo bị revoke nếu cần |
| Hủy đăng nhập trên Keycloak | state+error được xử lý sạch, không đòi code, transaction được kết thúc |
| Không có enterprise/license | Vẫn pass flow web |
| Mobile route | Thông báo không hỗ trợ, không chặn release |

E2E chạy một worker mỗi Mattermost server theo e2e-tests/playwright/AGENTS.md. Có thể song song bằng nhiều server độc lập. Test OIDC OSS không được skip vì thiếu license.

### 13.4 Keycloak fixture

Dùng image Keycloak được pin version/digest tại thời điểm implement, không dùng latest. Realm test được import lúc khởi động bằng start-dev --import-realm. Reset bằng môi trường/volume dùng một lần để không nhầm dữ liệu cũ với fixture mới. [S14]

Client test:

- Client type OIDC, Client authentication bật.
- Standard flow bật; implicit, direct access grants, service accounts tắt.
- PKCE S256 bắt buộc.
- Redirect URI chính xác tới callback của Mattermost fixture.
- ID token có email, email_verified, preferred_username, given_name, family_name.
- Signing algorithm RS256.
- Secret và user/password chỉ dành cho fixture, không dùng credential thật.

Users: verified, unverified, disabled, email trùng tài khoản Mattermost local, username có dấu/ký tự đặc biệt, hai user preferred_username giống nhau.

PoC đơn giản chạy Mattermost và browser trên host, Keycloak Docker bind 127.0.0.1:18080. Issuer chung là http://127.0.0.1:18080/realms/mattermost-dev, SiteURL là http://127.0.0.1:8065 và MM_OPENIDSETTINGS_DEVALLOWHTTPLOOPBACK=true. Browser test phải kiểm tra đúng cookie development được định nghĩa ở mục 4. Nếu chạy toàn Docker, phải bảo đảm browser và backend nhìn thấy cùng canonical issuer; fixture không thỏa literal loopback phải dùng HTTPS với CA test đã được cả browser và backend tin cậy, không tắt issuer check.

## 14. Chia PR và thứ tự triển khai

| PR | Nội dung | Phụ thuộc | Điều kiện merge |
| --- | --- | --- | --- |
| 1 | OSS build setup, specialized config, package skeleton/dependencies | Không | Clean build; config JSON/env round-trip; feature mặc định tắt |
| 2 | Discovery, PKCE, exchange, verification, HTTP policy, runtime cache | PR1 | Package tests và config reload tests |
| 3 | Tokens adapter, TTL/browser binding, identity schema/store, concurrency primitives | PR1 | Store tests và migration test |
| 4 | Dedicated web flow, user resolve/JIT/profile, native session, UI capability | PR2+PR3 | Browser flow với mock IdP và policy tests |
| 5 | Admin link API/mmctl, mapping protection, invalidation/revocation | PR3+PR4 | Existing-account migration và partial-failure tests |
| 6 | Keycloak E2E, CI không license/enterprise, runbook và rollout | PR4+PR5 | Tất cả acceptance v1 pass |

PR2 và PR3 có thể làm song song sau khi thống nhất DTO/config/identity key. PR4 phải đi sau để không có nút login trỏ vào backend chưa an toàn. Không thêm menu người dùng cuối khi backend còn stub.

Ước lượng kế hoạch: khoảng 7–10 ngày công cho một developer Go quen codebase, gồm test/review và CLI migration; đây là dự trù, không phải thời gian đã đo. Các phần protocol và store có thể chia người/agent để giảm thời gian chờ, nhưng auth integration và review cuối vẫn cần làm tuần tự. UI System Console/mobile không nằm trong dự trù này.

## 15. Rollout và rollback

1. Build binary/image OSS sạch; record commit/image digest.
2. Apply migration OIDCIdentities; để Enable=false.
3. Cấu hình Keycloak client, issuer, secret và verified test user trên staging.
4. Bật OIDC ở staging. Test JIT, existing account link, disabled user, logout và lỗi provider.
5. Pilot một nhóm nhỏ; không batch migrate toàn bộ người dùng trước khi verify.
6. Bật cho nội bộ sau acceptance. Việc tắt local sign-in là quyết định rollout riêng, chỉ thực hiện khi mọi tài khoản cần dùng đã có đường đăng nhập.
7. Có runbook quản trị khi issuer/secret sai và cách revoke session bằng cơ chế Mattermost hiện có.

Rollback ứng dụng: tắt OpenIdSettings.Enable, dừng bắt đầu/callback mới; giữ bảng identity và auth mapping. Không drop bảng hoặc đổi issuer để chữa lỗi đăng nhập. User đã chuyển sang OIDC không tự có lại password local vì password đã bị xóa khi chuyển. Khôi phục tài khoản phải dùng quy trình quản trị rõ ràng, giữ user ID.

Nếu cần rollback database, chỉ làm sau khi xử lý toàn bộ Users có auth binding custom; migration down xóa bảng không phải là kế hoạch khôi phục account.

## 16. Nguồn đối chiếu

[S1]: https://github.com/r2d-ai/mattermost/commit/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0
[S2]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/einterfaces/oauthproviders.go
[S3]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/app/oauth.go
[S4]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/store/sqlstore/tokens_store.go
[S5]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/public/model/token.go
[S6]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/db/migrations/postgres/000046_create_users.up.sql
[S7]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/app/user.go
[S8]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/app/users/users.go
[S9]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/app/login.go
[S10]: https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/public/model/config.go
[S11]: https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc
[S12]: https://openid.net/specs/openid-connect-core-1_0.html
[S13]: https://datatracker.ietf.org/doc/html/rfc9700
[S14]: https://www.keycloak.org/server/importExport

- [Fork baseline][S1]; [OAuthProvider interface][S2]; [OAuth flow][S3].
- [Atomic token store][S4]; [Token type và TTL][S5].
- [Users schema][S6]; [User operations][S7]; [First-user bootstrap][S8]; [Session creation][S9].
- [Config models][S10]; [go-oidc][S11]; [OIDC Core][S12]; [OAuth Security BCP][S13]; [Keycloak realm import][S14].
- [Tokens schema](https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/db/migrations/postgres/000028_create_tokens.up.sql).
- [User store](https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/channels/store/sqlstore/user_store.go).
- [Server instructions](https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/server/AGENTS.md).
- [Playwright instructions](https://github.com/r2d-ai/mattermost/blob/29845a7a189b82f4f510eb2e18fdb1cc598bcdf0/e2e-tests/playwright/AGENTS.md).

