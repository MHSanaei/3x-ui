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

* `{{ .sId }}`: Subscription ID (UUID).
* `{{ .enabled }}`: Whether the subscription/client is enabled (boolean).
* `{{ .isOnline }}`: Whether the subscription's client has a live connection right now (boolean). Computed from the panel's online-client tracking (local Xray plus any remote nodes) at render time.
* `{{ .download }}`: Formatted download traffic (e.g. "2.5 GB").
* `{{ .upload }}`: Formatted upload traffic.
* `{{ .total }}`: Formatted total traffic limit.
* `{{ .used }}`: Formatted used traffic (download + upload).
* `{{ .remained }}`: Formatted remaining traffic.
* `{{ .expire }}`: Expiration time as an int64 Unix timestamp in **seconds** (`0` means never). Multiply by 1000 for a JavaScript `Date`.
* `{{ .lastOnline }}`: Last online time as an int64 Unix timestamp in **milliseconds** (`0` means never seen).
* `{{ .downloadByte }}`: Download traffic in exact bytes (int64).
* `{{ .uploadByte }}`: Upload traffic in exact bytes (int64).
* `{{ .totalByte }}`: Total traffic limit in exact bytes (int64).
* `{{ .subUrl }}`: The URL of the subscription page.
* `{{ .subJsonUrl }}`: The URL for the JSON configuration of the subscription.
* `{{ .subClashUrl }}`: The URL for the Clash/Mihomo configuration.
* `{{ .subTitle }}`: The subscription title configured in the panel (Subscription → Information). Useful for page branding/headings. May be empty.
* `{{ .subSupportUrl }}`: The support URL configured in the panel. Useful for a "Contact support" link. May be empty.
* `{{ .links }}`: A list (slice) of string configurations (VMess, VLESS, etc. URLs). You can loop through them using `{{ range .links }} ... {{ end }}`.
* `{{ .emails }}`: A list (slice) of client emails, parallel to `links` — the email at index *i* owns the link at index *i*. May contain duplicates when one client has several links.
* `{{ .announce }}`: The announcement text configured in the panel (Settings → Subscription → Announce). May be empty.
* `{{ .datepicker }}`: Current calendar format used by the panel (e.g. "gregorian" or "jalali").

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
