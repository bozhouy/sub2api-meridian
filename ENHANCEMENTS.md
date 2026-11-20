# Sub2API 0.2.23 + local request enhancements

Base: Wei-Shaw/sub2api release v0.2.23, commit `5af9afb588c4b014b3df9db7d4073af1f8ae01b6`.
Meridian release: `0.2.23-meridian.1`.

Administrator navigation follows Sub4API's original account workflow:
- **账号管理 → 添加账号 / 编辑账号 → 请求时区** uses its searchable timezone field.
- **账号管理 → 更多 → 降智检测** opens the original account-scoped model picker,
  sequential progress and results dialog, adapted to the authenticated standalone diagnostic API.
- Existing Accept-Language normalization runs automatically in the gateway.
The temporary separate enhancement sidebar was removed; its old URL redirects to account management.

- Per-account fixed IANA timezone, follow account exit IP, or preserve client timezone.
  Uses the existing JSON `accounts.extra` fields; no custom SQL migration.
  Only standalone environment_context user blocks and existing search timezone fields are rewritten.
  Historical dates and ordinary conversation content are preserved, matching the source implementation
  in Sub4API v1.1.11. Its README's statement about replacing current_date is broader than its actual code.
- Existing Accept-Language headers on OpenAI HTTP and WebSocket requests are normalized to
  en-US,en;q=0.9. Missing headers stay absent. Per-account opt-out is available.
- Manual ModelTrace diagnostics use an administrator-owned gateway API key and pin a selected
  eligible account through the normal scheduler. Normal authentication, group selection, quotas,
  concurrency and billing remain in effect. No ticket harvesting/injection is included.
  At most six models, one concurrent diagnostic per administrator, two MiB response capture
  and a two-minute deadline per model. Failed/incomplete requests are not called degradation.
  Fingerprint mismatch is only a heuristic; the displayed score is not identity certainty.
  The bank is fixed at the version included with Sub4API v1.1.11, not silently updated online.

Source attribution: locale/timezone processing, exit detection and ModelTrace classifier/challenge
were adapted from MACOS-DO/sub4api v1.1.11, under LGPL-3.0-or-later (see LICENSE).
ModelTrace assets and challenge include xqy2006's MIT license in
backend/internal/service/modeltrace_LICENSE. Standalone diagnostic handler and API adapter are local; timezone controls and the diagnostic dialog/picker are adapted from the same Sub4API release.

Build with pnpm 9.15.9 and the frozen frontend lockfile. Use Go 1.27.4 with
CGO_ENABLED=0, GOOS=linux, GOARCH=amd64 and the embed tag for the VPS binary.
Do not use the website's upstream updater to replace this custom build: it would remove these features.
Future updates should rebase the local commits, rebuild, test a database copy, and back up before cutover.
