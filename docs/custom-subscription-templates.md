# 3x-ui Custom Subscription Templates

3x-ui can render your users' subscription pages from your own custom HTML templates.

## How to use a Custom Template

1. Go to the 3x-ui panel settings.
2. Under **Settings → Subscription → Information**, locate the **Sub Theme Directory** field.
3. Provide the absolute path to the folder containing your template (e.g. `/etc/3x-ui/sub_templates/my-theme/`).
4. Save the settings.

> **Note:** 3x-ui does not ship any templates by default. Create your own template folder anywhere
> on the server, put an `index.html` (or `sub.html`) inside it, and point **Sub Theme Directory** at
> that absolute path. Leave the field empty to use the default built-in page.

## Creating a Template

A custom template must be an HTML file named `index.html` or `sub.html` located within the directory you specified in the settings.
The panel uses standard Go `html/template` to render the subscription page.

### Available Variables

When rendering the template, the following variables are injected into the template context (`{{ .variable }}`):

- `{{ .sId }}`: Subscription ID (UUID).
- `{{ .enabled }}`: Whether the subscription/client is enabled (boolean).
- `{{ .isOnline }}`: Whether the subscription's client has a live connection right now (boolean). Computed from the panel's online-client tracking (local Xray plus any remote nodes) at render time.
- `{{ .download }}`: Formatted download traffic (e.g. "2.5 GB").
- `{{ .upload }}`: Formatted upload traffic.
- `{{ .total }}`: Formatted total traffic limit.
- `{{ .used }}`: Formatted used traffic (download + upload).
- `{{ .remained }}`: Formatted remaining traffic.
- `{{ .expire }}`: Expiration time as an int64 Unix timestamp in **seconds** (`0` means never). Multiply by 1000 for a JavaScript `Date`.
- `{{ .lastOnline }}`: Last online time as an int64 Unix timestamp in **milliseconds** (`0` means never seen).
- `{{ .depletedAt }}`: Time the current depletion period began, as an int64 Unix timestamp in **milliseconds**. `0` means no current event or an unknown historical time.
- `{{ .depletedReason }}`: Current blocking conditions: `"traffic"`, `"expiry"`, `"both"`, or `""` when neither limit blocks the client. Computed on each read, even when the historical timestamp is unknown.
- `{{ .downloadByte }}`: Download traffic in exact bytes (int64).
- `{{ .uploadByte }}`: Upload traffic in exact bytes (int64).
- `{{ .totalByte }}`: Total traffic limit in exact bytes (int64).
- `{{ .subUrl }}`: The URL of the subscription page.
- `{{ .subJsonUrl }}`: The URL for the JSON configuration of the subscription.
- `{{ .subClashUrl }}`: The URL for the Clash/Mihomo configuration.
- `{{ .subTitle }}`: The subscription title configured in the panel (Subscription → Information). Useful for page branding/headings. May be empty.
- `{{ .subSupportUrl }}`: The support URL configured in the panel. Useful for a "Contact support" link. May be empty.
- `{{ .links }}`: A list (slice) of string configurations (VMess, VLESS, etc. URLs). You can loop through them using `{{ range .links }} ... {{ end }}`.
- `{{ .emails }}`: A list (slice) of client emails, parallel to `links` — the email at index _i_ owns the link at index _i_. May contain duplicates when one client has several links.
- `{{ .announce }}`: The announcement text configured in the panel (Settings → Subscription → Announce). May be empty.
- `{{ .datepicker }}`: Current calendar format used by the panel (e.g. "gregorian" or "jalali").

### Depletion semantics

Traffic exhaustion is recorded when the traffic job detects it, so its precision is the job interval
(normally five seconds), rather than the instant the final byte was consumed. Expiry records the exact
configured expiry boundary when expiry establishes the first known event. When both are first detected
together, the earlier time wins; expiry wins a tie.
Cross-panel quota exhaustion is a traffic event. A master preserves the accepted timestamp from a node.

The first event remains throughout a continuous depletion period. A manual disable does not create an
event, and a manual enable does not replace it. The timestamp clears only when quota and expiry allow
the client again. Resetting traffic while the client remains expired therefore preserves its timestamp
and changes the current reason to `"expiry"`. Expiry after traffic exhaustion changes the reason to
`"both"` on the next read without a database write.

Recovery describes quota and expiry eligibility; clearing an event does not itself enable a client.
Cross-panel totals stop affecting eligibility after their existing freshness window expires. An already
disabled client that later receives exhausted totals has no new enabled-to-disabled transition, so its
event can remain unknown.

Quota eligibility uses the existing enforcement accounting, including the shared accumulator of node
deltas and each fresh cross-panel aggregate. Display-only per-direction maxima from different masters are never added
together to determine a reason. Existing renewal eligibility, including unattached-client behavior,
is unchanged.

A legacy adoption limitation remains: when a known node inbound has no master traffic row, upstream
starts the master accumulator at zero and treats the node's current counters as its delta baseline.
The master can therefore undercount usage from before adoption. Its depletion fields reflect its own
authoritative accumulator and fresh cross-panel totals; it does not infer a current quota period from
node baselines, which carry no period identifier. A node's own API still reports its local state.
This feature does not reset other nodes on renewal or change that existing accounting behavior.

For a shared subscription ID, every distinct member must currently be depleted, including members
without emitted links. Otherwise the subscription returns `0` and `""`. For an entirely depleted
subscription, the reason is the union of member causes: mixed traffic-only and expiry-only members
produce `"both"`; it does not imply every member has both causes. The timestamp is the latest member
event only when every event is known; otherwise it is `0` while the current reason remains available.

These read-only fields are also returned beside `usedTraffic` in existing client-detail API responses
and inside existing traffic objects in client lists, traffic lookups, and inbound responses. No request
body or route changes are required. Historical events from before the upgrade remain unknown. Database
backups preserve the persisted timestamp; portable client export/import retains its existing format and does not
transfer depletion history.

Only `depleted_at` is stored (`BIGINT NOT NULL DEFAULT 0`). No reason column is added; a legacy
`depleted_reason` column from an earlier build is retained but ignored by API reads and writes.
Ordinary traffic polling performs no full-table metadata cleanup. Relevant mutations clear recovered
timestamps transactionally in bounded batches; read-only responses hide obsolete timestamps.

Older binaries do not maintain these fields. If clients recover and deplete again during a downgrade,
the retained event can refer to an earlier period after re-upgrading. History continuity across such a
downgrade is unavailable; database backups retain the values that existed when the backup was taken.

## Live Status JSON (`?format=info`)

Every subscription URL also answers `GET <sub URL>?format=info` with the same view-model as JSON —
minus `links`, and with `emails` deduplicated — so a template can poll it and update usage or
online status live without reloading the page:

```json
{
  "sId": "…",
  "enabled": true,
  "isOnline": true,
  "used": "1.2 GB",
  "remained": "8.8 GB",
  "expire": 0,
  "lastOnline": 1735680000000,
  "depletedAt": 0,
  "depletedReason": "",
  "…": "…"
}
```

Example polling snippet for a template:

```html
<span id="status"></span>
<script>
  async function refreshStatus() {
    const res = await fetch(window.location.pathname + '?format=info');
    if (!res.ok) return;
    const info = await res.json();
    document.getElementById('status').textContent = info.isOnline ? 'Online' : 'Offline';
  }
  refreshStatus();
  setInterval(refreshStatus, 10000);
</script>
```
