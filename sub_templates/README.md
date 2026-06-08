# 3x-ui Custom Subscription Templates

This directory allows you to use custom HTML templates for your users' subscription pages.

## How to use a Custom Template

1. Go to the 3x-ui panel settings.
2. Under "Subscription" -> "Panel Settings", locate the **Sub Theme Directory** field.
3. Provide the absolute path to the folder containing your template (e.g. `/etc/3x-ui/sub_templates/tx-ui/`).
4. Save the settings and restart the panel if needed.

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
* `{{ .expire }}`: Expiration time as an int64 timestamp (in milliseconds).
* `{{ .lastOnline }}`: Last online time as an int64 timestamp.
* `{{ .downloadByte }}`: Download traffic in exact bytes (int64).
* `{{ .uploadByte }}`: Upload traffic in exact bytes (int64).
* `{{ .totalByte }}`: Total traffic limit in exact bytes (int64).
* `{{ .subUrl }}`: The URL of the subscription page.
* `{{ .subJsonUrl }}`: The URL for the JSON configuration of the subscription.
* `{{ .subClashUrl }}`: The URL for the Clash/Mihomo configuration.
* `{{ .links }}`: A list (slice) of string configurations (VMess, VLESS, etc. URLs). You can loop through them using `{{ range .links }} ... {{ end }}`.
* `{{ .emails }}`: A list (slice) of emails related to the subscription.
* `{{ .datepicker }}`: Current calendar format used by the panel (e.g. "gregorian" or "jalali").
* `{{ .result }}`: Alias for `.links`, added for tx-ui compatibility.
* `{{ .jsonUrl }}`: Alias for `.subJsonUrl`, added for tx-ui compatibility.

### tx-ui Compatibility

You can import subscription templates from [tx-ui (AghayeCoder/tx-ui)](https://github.com/AghayeCoder/tx-ui) directly. The `tx-ui` template is already included in this repository under the `tx-ui/` folder!

To use it, set your **Sub Theme Directory** to the absolute path of the `tx-ui` template folder (e.g., `/opt/3x-ui/sub_templates/tx-ui/`).

*Credit: The tx-ui template is created by [AghayeCoder](https://github.com/AghayeCoder/tx-ui).*
