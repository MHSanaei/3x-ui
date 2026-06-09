# 3x-ui Custom Subscription Templates

This directory allows you to use custom HTML templates for your users' subscription pages.

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
* `{{ .emails }}`: A list (slice) of emails related to the subscription.
* `{{ .datepicker }}`: Current calendar format used by the panel (e.g. "gregorian" or "jalali").
